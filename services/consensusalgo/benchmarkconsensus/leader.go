// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package benchmarkconsensus

import (
	"context"
	"fmt"
	"github.com/orbs-network/crypto-lib-go/crypto/digest"
	ethereumDigest "github.com/orbs-network/crypto-lib-go/crypto/ethereum/digest"
	"github.com/orbs-network/orbs-network-go/instrumentation/logfields"
	"github.com/orbs-network/orbs-network-go/instrumentation/trace"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/protocol/consensus"
	"github.com/orbs-network/orbs-spec/types/go/protocol/gossipmessages"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/orbs-spec/types/go/services/gossiptopics"
	"github.com/orbs-network/scribe/log"
	"github.com/pkg/errors"
	"time"
)

func (s *Service) leaderConsensusRoundRunLoop(parent context.Context) {
	if _, block := s.getLastCommittedBlock(); block == nil {
		s.mutex.Lock()
		s.lastCommittedBlockUnderMutex = s.leaderGenerateGenesisBlock(parent)
		s.mutex.Unlock()
	}
	for {
		start := time.Now()
		ctx := trace.NewContext(parent, "BenchmarkConsensus.Tick")
		logger := s.logger.WithTags(trace.LogFieldFrom(ctx))

		err := s.leaderConsensusRoundTick(ctx)
		if err != nil {
			logger.Info("consensus round tick failed", log.Error(err))
			s.metrics.failedConsensusTicksRate.Measure(1)
		}
		select {
		case <-ctx.Done():
			logger.Info("consensus round run loop terminating with context")
			return
		case s.lastSuccessfullyVotedBlock = <-s.successfullyVotedBlocks:
			logger.Info("consensus round waking up after successfully voted block", logfields.BlockHeight(s.lastSuccessfullyVotedBlock))
			s.metrics.consensusRoundTickTime.RecordSince(start)
			continue
		case <-time.After(s.config.BenchmarkConsensusRetryInterval()):
			logger.Info("consensus round waking up after retry timeout")
			s.metrics.timedOutConsensusTicksRate.Measure(1)
			continue
		}
	}
}

func (s *Service) leaderConsensusRoundTick(ctx context.Context) error {
	lastCommittedBlockHeight, lastCommittedBlock := s.getLastCommittedBlock()
	logger := s.logger.WithTags(trace.LogFieldFrom(ctx))

	// check if we need to move to next block
	if s.lastSuccessfullyVotedBlock == lastCommittedBlockHeight {
		proposedBlock, err := s.leaderGenerateNewProposedBlock(ctx, lastCommittedBlockHeight, lastCommittedBlock)
		if err != nil {
			return err
		}

		err = s.saveToBlockStorage(ctx, proposedBlock)
		if err != nil {
			logger.Error("leader failed to save block to storage", log.Error(err))
			return err
		}
		s.metrics.lastCommittedTime.Update(time.Now().UnixNano())
		updated := s.setLastCommittedBlockIfPreviousBlockMatches(proposedBlock, lastCommittedBlock)
		if !updated {
			return nil // Updated concurrently
		}
		// don't forget to update internal vars too since they may be used later on in the function
		lastCommittedBlock = proposedBlock
		lastCommittedBlockHeight = lastCommittedBlock.TransactionsBlock.Header.BlockHeight()
	}

	// broadcast the commit via gossip for last committed block
	err := s.leaderBroadcastCommittedBlock(ctx, lastCommittedBlock)
	if err != nil {
		return err
	}

	if len(s.network) == 1 {
		s.successfullyVotedBlocks <- lastCommittedBlockHeight
	}

	return nil
}

// used for the first commit a leader does which is nop (genesis block) just to see where everybody's at
func (s *Service) leaderGenerateGenesisBlock(ctx context.Context) *protocol.BlockPairContainer {
	transactionsBlock := &protocol.TransactionsBlockContainer{
		Header: (&protocol.TransactionsBlockHeaderBuilder{
			BlockHeight:          0,
			BlockProposerAddress: s.config.BenchmarkConsensusConstantLeader(),
		}).Build(),
		Metadata:           (&protocol.TransactionsBlockMetadataBuilder{}).Build(),
		SignedTransactions: []*protocol.SignedTransaction{},
		BlockProof:         nil, // will be generated in a minute when signed
	}
	resultsBlock := &protocol.ResultsBlockContainer{
		Header: (&protocol.ResultsBlockHeaderBuilder{
			BlockHeight:          0,
			BlockProposerAddress: s.config.BenchmarkConsensusConstantLeader(),
		}).Build(),
		TransactionReceipts: []*protocol.TransactionReceipt{},
		ContractStateDiffs:  []*protocol.ContractStateDiff{},
		BlockProof:          nil, // will be generated in a minute when signed
	}
	blockPair, err := s.leaderSignBlockProposal(ctx, transactionsBlock, resultsBlock)
	if err != nil {
		s.logger.Error("leader failed to sign genesis block", log.Error(err))
		panic(fmt.Sprintf("leader failed to sign genesis block, abort, err=%s", err.Error()))
		return nil
	}
	return blockPair
}

func (s *Service) leaderGenerateNewProposedBlock(ctx context.Context, lastCommittedBlockHeight primitives.BlockHeight, lastCommittedBlock *protocol.BlockPairContainer) (*protocol.BlockPairContainer, error) {
	logger := s.logger.WithTags(trace.LogFieldFrom(ctx))

	logger.Info("generating new proposed block", logfields.BlockHeight(lastCommittedBlockHeight+1))

	// get tx
	txOutput, err := s.consensusContext.RequestNewTransactionsBlock(ctx, &services.RequestNewTransactionsBlockInput{
		CurrentBlockHeight:      lastCommittedBlockHeight + 1,
		MaxBlockSizeKb:          0, // TODO(v1): fill in or remove from spec
		MaxNumberOfTransactions: 0,
		PrevBlockHash:           digest.CalcTransactionsBlockHash(lastCommittedBlock.TransactionsBlock),
		PrevBlockTimestamp:      lastCommittedBlock.TransactionsBlock.Header.Timestamp(),
		PrevBlockReferenceTime:  lastCommittedBlock.TransactionsBlock.Header.ReferenceTime(),
		BlockProposerAddress:    s.config.BenchmarkConsensusConstantLeader(),
	})
	if err != nil {
		return nil, err
	}

	// get rx
	rxOutput, err := s.consensusContext.RequestNewResultsBlock(ctx, &services.RequestNewResultsBlockInput{
		CurrentBlockHeight:     lastCommittedBlockHeight + 1,
		PrevBlockHash:          digest.CalcResultsBlockHash(lastCommittedBlock.ResultsBlock),
		TransactionsBlock:      txOutput.TransactionsBlock,
		PrevBlockTimestamp:     lastCommittedBlock.ResultsBlock.Header.Timestamp(),
		PrevBlockReferenceTime: lastCommittedBlock.ResultsBlock.Header.ReferenceTime(),
		BlockProposerAddress:   s.config.BenchmarkConsensusConstantLeader(),
	})
	if err != nil {
		return nil, err
	}

	// generate signed block
	return s.leaderSignBlockProposal(ctx, txOutput.TransactionsBlock, rxOutput.ResultsBlock)
}

func (s *Service) leaderSignBlockProposal(ctx context.Context, transactionsBlock *protocol.TransactionsBlockContainer, resultsBlock *protocol.ResultsBlockContainer) (*protocol.BlockPairContainer, error) {
	blockPair := &protocol.BlockPairContainer{
		TransactionsBlock: transactionsBlock,
		ResultsBlock:      resultsBlock,
	}

	// prepare signature over the block headers
	signedData := s.signedDataForBlockProof(blockPair)
	sig, err := s.signer.Sign(ctx, signedData)
	if err != nil {
		return nil, err
	}

	// generate tx block proof
	blockPair.TransactionsBlock.BlockProof = (&protocol.TransactionsBlockProofBuilder{
		Type:               protocol.TRANSACTIONS_BLOCK_PROOF_TYPE_BENCHMARK_CONSENSUS,
		BenchmarkConsensus: &consensus.BenchmarkConsensusBlockProofBuilder{},
	}).Build()

	// generate rx block proof
	blockPair.ResultsBlock.BlockProof = (&protocol.ResultsBlockProofBuilder{
		TransactionsBlockHash: digest.CalcTransactionsBlockHash(transactionsBlock),
		Type:                  protocol.RESULTS_BLOCK_PROOF_TYPE_BENCHMARK_CONSENSUS,
		BenchmarkConsensus: &consensus.BenchmarkConsensusBlockProofBuilder{
			BlockRef: consensus.BenchmarkConsensusBlockRefBuilderFromRaw(signedData),
			Nodes: []*consensus.BenchmarkConsensusSenderSignatureBuilder{{
				SenderNodeAddress: s.config.NodeAddress(),
				Signature:         sig,
			}},
			Placeholder: []byte{0x01, 0x02},
		},
	}).Build()

	return blockPair, nil
}

func (s *Service) leaderBroadcastCommittedBlock(ctx context.Context, blockPair *protocol.BlockPairContainer) error {
	logger := s.logger.WithTags(trace.LogFieldFrom(ctx))
	logger.Info("broadcasting commit block", logfields.BlockHeight(blockPair.TransactionsBlock.Header.BlockHeight()))

	// the block pair fields we have may be partial (for example due to being read from persistence storage on init) so don't broadcast it in this case
	if blockPair == nil || blockPair.TransactionsBlock.BlockProof == nil || blockPair.ResultsBlock.BlockProof == nil {
		err := errors.Errorf("attempting to broadcast commit of a partial block that is missing fields like block proofs: %v", blockPair.String())
		logger.Error("leader broadcast commit failed", log.Error(err))
		return err
	}

	_, err := s.gossip.BroadcastBenchmarkConsensusCommit(ctx, &gossiptopics.BenchmarkConsensusCommitInput{
		Message: &gossipmessages.BenchmarkConsensusCommitMessage{
			BlockPair: blockPair,
		},
	})

	return err
}

func (s *Service) leaderHandleCommittedVote(ctx context.Context, sender *gossipmessages.SenderSignature, status *gossipmessages.BenchmarkConsensusStatus) error {
	s.logger.Info("Got committed message", trace.LogFieldFrom(ctx))
	lastCommittedBlockHeight, lastCommittedBlock := s.getLastCommittedBlock()

	// validate the vote
	err := s.leaderValidateVote(sender, status, lastCommittedBlockHeight)
	if err != nil {
		return err
	}

	// add the vote
	enoughVotesReceived, err := s.leaderAddVote(ctx, sender, status, lastCommittedBlock)
	if err != nil {
		return err
	}

	// move the consensus forward
	if enoughVotesReceived {
		select {
		case s.successfullyVotedBlocks <- lastCommittedBlockHeight:
			s.logger.Info("Block has reached consensus", logfields.BlockHeight(lastCommittedBlockHeight), trace.LogFieldFrom(ctx))
		case <-ctx.Done():
			return ctx.Err()
		}

	}

	return nil
}

func (s *Service) leaderValidateVote(sender *gossipmessages.SenderSignature, status *gossipmessages.BenchmarkConsensusStatus, lastCommittedBlockHeight primitives.BlockHeight) error {
	// block height
	blockHeight := status.LastCommittedBlockHeight()
	if blockHeight != lastCommittedBlockHeight {
		return errors.Errorf("committed message with wrong block height %d, expecting %d", blockHeight, lastCommittedBlockHeight)
	}

	// approved signer TODO https://github.com/orbs-network/orbs-network-go/issues/1602 make aware of committes changes and better signature checking
	found := false
	for i := range s.network {
		if s.network[i].Equal(sender.SenderNodeAddress()) {
			found = true;
			break;
		}
	}
	if !found {
		return errors.Errorf("signer with public key %s is not a valid validator", sender.SenderNodeAddress())
	}

	// signature
	if err := ethereumDigest.VerifyNodeSignature(sender.SenderNodeAddress(), status.Raw(), sender.Signature()); err != nil {
		return errors.Wrapf(err, "sender signature is invalid: %s, signed data: %s", sender.Signature(), status.Raw())
	}

	return nil
}

func (s *Service) leaderAddVote(ctx context.Context, sender *gossipmessages.SenderSignature, status *gossipmessages.BenchmarkConsensusStatus, expectedLastCommittedBlockBefore *protocol.BlockPairContainer) (bool, error) {
	logger := s.logger.WithTags(trace.LogFieldFrom(ctx))

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.lastCommittedBlockUnderMutex != expectedLastCommittedBlockBefore {
		return false, errors.New("aborting shared state update due to inconsistency")
	}

	// add the vote to our shared state variable
	s.lastCommittedBlockVotersUnderMutex[sender.SenderNodeAddress().KeyForMap()] = true

	// count if we have enough votes to move forward
	existingVotes := len(s.lastCommittedBlockVotersUnderMutex) + 1
	logger.Info("valid vote arrived", logfields.BlockHeight(status.LastCommittedBlockHeight()), log.Int("existing-votes", existingVotes), log.Int("required-votes", s.requiredQuorumSize()))
	if existingVotes >= s.requiredQuorumSize() && !s.lastCommittedBlockVotersReachedQuorumUnderMutex {
		s.lastCommittedBlockVotersReachedQuorumUnderMutex = true
		return true, nil
	}
	return false, nil
}
