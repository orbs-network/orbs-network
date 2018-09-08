package blockstorage

import (
	"context"
	"fmt"
	"github.com/orbs-network/orbs-network-go/crypto/bloom"
	"github.com/orbs-network/orbs-network-go/instrumentation/log"
	"github.com/orbs-network/orbs-network-go/services/blockstorage/adapter"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/orbs-spec/types/go/services/gossiptopics"
	"github.com/orbs-network/orbs-spec/types/go/services/handlers"
	"github.com/pkg/errors"
	"sync"
	"time"
)

type Config interface {
	NodePublicKey() primitives.Ed25519PublicKey
	BlockSyncBatchSize() uint32
	BlockSyncInterval() time.Duration
	BlockSyncCollectResponseTimeout() time.Duration
	BlockSyncCollectChunksTimeout() time.Duration
	BlockTransactionReceiptQueryGraceStart() time.Duration
	BlockTransactionReceiptQueryGraceEnd() time.Duration
	BlockTransactionReceiptQueryExpirationWindow() time.Duration
}

const (
	// TODO extract it to the spec
	ProtocolVersion = 1
)

type service struct {
	persistence  adapter.BlockPersistence
	stateStorage services.StateStorage
	gossip       gossiptopics.BlockSync
	txPool       services.TransactionPool

	config Config

	reporting               log.BasicLogger
	consensusBlocksHandlers []handlers.ConsensusBlocksHandler

	lastCommittedBlock *protocol.BlockPairContainer
	lastBlockLock      *sync.RWMutex

	blockSync *BlockSync
}

func NewBlockStorage(ctx context.Context, config Config, persistence adapter.BlockPersistence, stateStorage services.StateStorage, gossip gossiptopics.BlockSync,
	txPool services.TransactionPool, reporting log.BasicLogger) services.BlockStorage {
	logger := reporting.For(log.Service("block-storage"))

	storage := &service{
		persistence:   persistence,
		stateStorage:  stateStorage,
		gossip:        gossip,
		txPool:        txPool,
		reporting:     logger,
		config:        config,
		lastBlockLock: &sync.RWMutex{},
	}

	lastBlock, err := persistence.GetLastBlock()

	if err != nil {
		logger.Error("could not update last block from persistence", log.Error(err))
	}

	if lastBlock != nil {
		storage.updateLastCommittedBlock(lastBlock)
	}

	gossip.RegisterBlockSyncHandler(storage)
	storage.blockSync = NewBlockSync(ctx, config, storage, gossip, reporting)

	return storage
}

func (s *service) CommitBlock(input *services.CommitBlockInput) (*services.CommitBlockOutput, error) {
	txBlockHeader := input.BlockPair.TransactionsBlock.Header
	s.reporting.Info("Trying to commit a block", log.BlockHeight(txBlockHeader.BlockHeight()))

	if err := s.validateProtocolVersion(input.BlockPair); err != nil {
		return nil, err
	}

	// TODO there might be a non-idiomatic pattern here, but the commit block output is an empty struct, if that changes this should be refactored
	if ok, err := s.validateBlockDoesNotExist(txBlockHeader); err != nil || !ok {
		return nil, err
	}

	if err := s.validateBlockHeight(input.BlockPair); err != nil {
		return nil, err
	}

	if err := s.persistence.WriteBlock(input.BlockPair); err != nil {
		return nil, err
	}

	s.updateLastCommittedBlock(input.BlockPair)

	s.reporting.Info("Committed a block", log.BlockHeight(txBlockHeader.BlockHeight()))

	if err := s.syncBlockToStateStorage(input.BlockPair); err != nil {
		// TODO: since the intra-node sync flow is self healing, we should not fail the entire commit if state storage is slow to sync
		s.reporting.Error("intra-node sync to state storage failed", log.Error(err))
	}

	if err := s.syncBlockToTxPool(input.BlockPair); err != nil {
		// TODO: since the intra-node sync flow is self healing, should we fail if pool fails ?
		s.reporting.Error("intra-node sync to tx pool failed", log.Error(err))
	}

	return nil, nil
}

func (s *service) updateLastCommittedBlock(block *protocol.BlockPairContainer) {
	s.lastBlockLock.Lock()
	defer s.lastBlockLock.Unlock()

	s.lastCommittedBlock = block
}

func (s *service) LastCommittedBlockHeight() primitives.BlockHeight {
	s.lastBlockLock.RLock()
	defer s.lastBlockLock.RUnlock()

	if s.lastCommittedBlock == nil {
		return 0
	}
	return s.lastCommittedBlock.TransactionsBlock.Header.BlockHeight()
}

func (s *service) lastCommittedBlockTimestamp() primitives.TimestampNano {
	s.lastBlockLock.RLock()
	defer s.lastBlockLock.RUnlock()

	if s.lastCommittedBlock == nil {
		return 0
	}
	return s.lastCommittedBlock.TransactionsBlock.Header.Timestamp()
}

func (s *service) getLastCommittedBlock() *protocol.BlockPairContainer {
	s.lastBlockLock.RLock()
	defer s.lastBlockLock.RUnlock()

	return s.lastCommittedBlock
}

func (s *service) loadTransactionsBlockHeader(height primitives.BlockHeight) (*services.GetTransactionsBlockHeaderOutput, error) {
	txBlock, err := s.persistence.GetTransactionsBlock(height)

	if err != nil {
		return nil, err
	}

	return &services.GetTransactionsBlockHeaderOutput{
		TransactionsBlockProof:    txBlock.BlockProof,
		TransactionsBlockHeader:   txBlock.Header,
		TransactionsBlockMetadata: txBlock.Metadata,
	}, nil
}

func (s *service) GetTransactionsBlockHeader(input *services.GetTransactionsBlockHeaderInput) (result *services.GetTransactionsBlockHeaderOutput, err error) {
	err = s.persistence.GetBlockTracker().WaitForBlock(input.BlockHeight)

	if err == nil {
		return s.loadTransactionsBlockHeader(input.BlockHeight)
	}

	return nil, err
}

func (s *service) loadResultsBlockHeader(height primitives.BlockHeight) (*services.GetResultsBlockHeaderOutput, error) {
	txBlock, err := s.persistence.GetResultsBlock(height)

	if err != nil {
		return nil, err
	}

	return &services.GetResultsBlockHeaderOutput{
		ResultsBlockProof:  txBlock.BlockProof,
		ResultsBlockHeader: txBlock.Header,
	}, nil
}

func (s *service) GetResultsBlockHeader(input *services.GetResultsBlockHeaderInput) (result *services.GetResultsBlockHeaderOutput, err error) {
	err = s.persistence.GetBlockTracker().WaitForBlock(input.BlockHeight)

	if err == nil {
		return s.loadResultsBlockHeader(input.BlockHeight)
	}

	return nil, err
}

func (s *service) createEmptyTransactionReceiptResult() *services.GetTransactionReceiptOutput {
	return &services.GetTransactionReceiptOutput{
		TransactionReceipt: nil,
		BlockHeight:        s.LastCommittedBlockHeight(),
		BlockTimestamp:     s.lastCommittedBlockTimestamp(),
	}
}

func (s *service) GetTransactionReceipt(input *services.GetTransactionReceiptInput) (*services.GetTransactionReceiptOutput, error) {
	searchRules := adapter.BlockSearchRules{
		EndGraceNano:          s.config.BlockTransactionReceiptQueryGraceEnd().Nanoseconds(),
		StartGraceNano:        s.config.BlockTransactionReceiptQueryGraceStart().Nanoseconds(),
		TransactionExpireNano: s.config.BlockTransactionReceiptQueryExpirationWindow().Nanoseconds(),
	}
	blocksToSearch := s.persistence.GetReceiptRelevantBlocks(input.TransactionTimestamp, searchRules)
	if blocksToSearch == nil {
		return nil, errors.Errorf("failed to search for blocks on tx timestamp of %d, hash %s", input.TransactionTimestamp, input.Txhash)
	}

	if len(blocksToSearch) == 0 {
		return s.createEmptyTransactionReceiptResult(), nil
	}

	for _, b := range blocksToSearch {
		tbf := bloom.NewFromRaw(b.ResultsBlock.Header.TimestampBloomFilter())
		if tbf.Test(input.TransactionTimestamp) {
			for _, txr := range b.ResultsBlock.TransactionReceipts {
				if txr.Txhash().Equal(input.Txhash) {
					return &services.GetTransactionReceiptOutput{
						TransactionReceipt: txr,
						BlockHeight:        b.ResultsBlock.Header.BlockHeight(),
						BlockTimestamp:     b.ResultsBlock.Header.Timestamp(),
					}, nil
				}
			}
		}
	}

	return s.createEmptyTransactionReceiptResult(), nil
}

func (s *service) GetLastCommittedBlockHeight(input *services.GetLastCommittedBlockHeightInput) (*services.GetLastCommittedBlockHeightOutput, error) {
	return &services.GetLastCommittedBlockHeightOutput{
		LastCommittedBlockHeight:    s.LastCommittedBlockHeight(),
		LastCommittedBlockTimestamp: s.lastCommittedBlockTimestamp(),
	}, nil
}

// FIXME implement all block checks
func (s *service) ValidateBlockForCommit(input *services.ValidateBlockForCommitInput) (*services.ValidateBlockForCommitOutput, error) {
	if protocolVersionError := s.validateProtocolVersion(input.BlockPair); protocolVersionError != nil {
		return nil, protocolVersionError
	}

	if blockHeightError := s.validateBlockHeight(input.BlockPair); blockHeightError != nil {
		return nil, blockHeightError
	}

	if err := s.validateWithConsensusAlgos(s.lastCommittedBlock, input.BlockPair); err != nil {
		s.reporting.Error("intra-node sync to consensus algo failed", log.Error(err))
	}

	return &services.ValidateBlockForCommitOutput{}, nil
}

func (s *service) RegisterConsensusBlocksHandler(handler handlers.ConsensusBlocksHandler) {
	s.consensusBlocksHandlers = append(s.consensusBlocksHandlers, handler)

	// update the consensus algo about the latest block we have (for its initialization)
	// TODO: should this be under mutex since it reads s.lastCommittedBlock
	s.UpdateConsensusAlgosAboutLatestCommittedBlock()
}

func (s *service) UpdateConsensusAlgosAboutLatestCommittedBlock() {
	lastCommitted := s.getLastCommittedBlock()

	if lastCommitted != nil {
		// passing nil on purpose, see spec
		err := s.validateWithConsensusAlgos(nil, lastCommitted)
		if err != nil {
			s.reporting.Error(err.Error())
		}
	}
}

func (s *service) HandleBlockAvailabilityRequest(input *gossiptopics.BlockAvailabilityRequestInput) (*gossiptopics.EmptyOutput, error) {
	if s.blockSync != nil {
		s.blockSync.events <- input.Message
	}
	return nil, nil
}

func (s *service) HandleBlockAvailabilityResponse(input *gossiptopics.BlockAvailabilityResponseInput) (*gossiptopics.EmptyOutput, error) {
	if s.blockSync != nil {
		s.blockSync.events <- input.Message
	}
	return nil, nil
}

func (s *service) HandleBlockSyncRequest(input *gossiptopics.BlockSyncRequestInput) (*gossiptopics.EmptyOutput, error) {
	if s.blockSync != nil {
		s.blockSync.events <- input.Message
	}
	return nil, nil
}

func (s *service) HandleBlockSyncResponse(input *gossiptopics.BlockSyncResponseInput) (*gossiptopics.EmptyOutput, error) {
	if s.blockSync != nil {
		s.blockSync.events <- input.Message
	}
	return nil, nil
}

//TODO how do we check if block with same height is the same block? do we compare the block bit-by-bit? https://github.com/orbs-network/orbs-spec/issues/50
func (s *service) validateBlockDoesNotExist(txBlockHeader *protocol.TransactionsBlockHeader) (bool, error) {
	currentBlockHeight := s.LastCommittedBlockHeight()
	if txBlockHeader.BlockHeight() <= currentBlockHeight {
		if txBlockHeader.BlockHeight() == currentBlockHeight && txBlockHeader.Timestamp() != s.lastCommittedBlockTimestamp() {
			errorMessage := "block already in storage, timestamp mismatch"
			s.reporting.Error(errorMessage, log.BlockHeight(currentBlockHeight))
			return false, errors.New(errorMessage)
		}

		s.reporting.Info("block already in storage, skipping", log.BlockHeight(currentBlockHeight))
		return false, nil
	}

	return true, nil
}

func (s *service) validateBlockHeight(blockPair *protocol.BlockPairContainer) error {
	expectedBlockHeight := s.LastCommittedBlockHeight() + 1

	txBlockHeader := blockPair.TransactionsBlock.Header
	rsBlockHeader := blockPair.ResultsBlock.Header

	if txBlockHeader.BlockHeight() != expectedBlockHeight {
		return fmt.Errorf("block height is %d, expected %d", txBlockHeader.BlockHeight(), expectedBlockHeight)
	}

	if rsBlockHeader.BlockHeight() != expectedBlockHeight {
		return fmt.Errorf("block height is %d, expected %d", rsBlockHeader.BlockHeight(), expectedBlockHeight)
	}

	return nil
}

func (s *service) validateProtocolVersion(blockPair *protocol.BlockPairContainer) error {
	txBlockHeader := blockPair.TransactionsBlock.Header
	rsBlockHeader := blockPair.ResultsBlock.Header

	// FIXME we may be logging twice, this should be fixed when handling the logging structured errors in logger issue
	if txBlockHeader.ProtocolVersion() != ProtocolVersion {
		errorMessage := "protocol version mismatch"
		s.reporting.Error(errorMessage, log.String("expected", "1"), log.Stringable("received", txBlockHeader.ProtocolVersion()))
		return fmt.Errorf(errorMessage)
	}

	if rsBlockHeader.ProtocolVersion() != ProtocolVersion {
		errorMessage := "protocol version mismatch"
		s.reporting.Error(errorMessage, log.String("expected", "1"), log.Stringable("received", txBlockHeader.ProtocolVersion()))
		return fmt.Errorf(errorMessage)
	}

	return nil
}

// TODO: this should not be called directly from CommitBlock, it should be called from a long living goroutine that continuously syncs the state storage
func (s *service) syncBlockToStateStorage(committedBlockPair *protocol.BlockPairContainer) error {
	_, err := s.stateStorage.CommitStateDiff(&services.CommitStateDiffInput{
		ResultsBlockHeader: committedBlockPair.ResultsBlock.Header,
		ContractStateDiffs: committedBlockPair.ResultsBlock.ContractStateDiffs,
	})
	return err
}

// TODO: this should not be called directly from CommitBlock, it should be called from a long living goroutine that continuously syncs the state storage
func (s *service) syncBlockToTxPool(committedBlockPair *protocol.BlockPairContainer) error {
	_, err := s.txPool.CommitTransactionReceipts(&services.CommitTransactionReceiptsInput{
		ResultsBlockHeader:       committedBlockPair.ResultsBlock.Header,
		TransactionReceipts:      committedBlockPair.ResultsBlock.TransactionReceipts,
		LastCommittedBlockHeight: committedBlockPair.ResultsBlock.Header.BlockHeight(),
	})
	return err
}

func (s *service) validateWithConsensusAlgos(prevBlockPair *protocol.BlockPairContainer, lastCommittedBlockPair *protocol.BlockPairContainer) error {
	for _, handler := range s.consensusBlocksHandlers {
		_, err := handler.HandleBlockConsensus(&handlers.HandleBlockConsensusInput{
			Mode:                   handlers.HANDLE_BLOCK_CONSENSUS_MODE_UPDATE_ONLY,
			BlockType:              protocol.BLOCK_TYPE_BLOCK_PAIR,
			BlockPair:              lastCommittedBlockPair,
			PrevCommittedBlockPair: prevBlockPair,
		})

		// one of the consensus algos has validated the block, this means it's a valid block
		if err == nil {
			return nil
		}
	}

	return errors.Errorf("all consensus %d algos refused to validate the block", len(s.consensusBlocksHandlers))
}

// Returns a slice of blocks containing first and last
// TODO support chunking
func (s *service) GetBlocks(first primitives.BlockHeight, last primitives.BlockHeight) (blocks []*protocol.BlockPairContainer, firstAvailableBlockHeight primitives.BlockHeight, lastAvailableBlockHeight primitives.BlockHeight) {
	// FIXME use more efficient way to slice blocks

	allBlocks := s.persistence.ReadAllBlocks()
	allBlocksLength := primitives.BlockHeight(len(allBlocks))

	s.reporting.Info("Reading all blocks", log.Stringable("blocks-total", allBlocksLength))

	firstAvailableBlockHeight = first

	if firstAvailableBlockHeight > allBlocksLength {
		return blocks, firstAvailableBlockHeight, firstAvailableBlockHeight
	}

	lastAvailableBlockHeight = last
	if allBlocksLength < last {
		lastAvailableBlockHeight = allBlocksLength
	}

	for i := first - 1; i < lastAvailableBlockHeight; i++ {
		s.reporting.Info("Retrieving block", log.BlockHeight(i), log.Stringable("blocks-total", i))
		blocks = append(blocks, allBlocks[i])
	}

	return blocks, firstAvailableBlockHeight, lastAvailableBlockHeight
}
