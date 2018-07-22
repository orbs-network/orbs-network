package blockstorage

import (
	"encoding/binary"
	"fmt"
	"github.com/orbs-network/orbs-network-go/services/blockstorage/adapter"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/orbs-spec/types/go/services/gossiptopics"
	"github.com/orbs-network/orbs-spec/types/go/services/handlers"
)

const (
	// TODO extract it to the spec
	ProtocolVersion = 1
)

type service struct {
	persistence  adapter.BlockPersistence
	stateStorage services.StateStorage

	lastCommittedBlockHeight    primitives.BlockHeight
	lastCommittedBlockTimestamp primitives.TimestampNano
}

func NewBlockStorage(persistence adapter.BlockPersistence, stateStorage services.StateStorage) services.BlockStorage {
	return &service{
		persistence:  persistence,
		stateStorage: stateStorage,
	}
}

func (s *service) CommitBlock(input *services.CommitBlockInput) (*services.CommitBlockOutput, error) {
	txBlockHeader := input.BlockPair.TransactionsBlock.Header

	if err := s.validateProtocolVersion(txBlockHeader); err != nil {
		return nil, err
	}

	if ok := s.validateBlockDoesNotExist(txBlockHeader); !ok {
		return nil, nil
	}

	s.validateMonotonicIncreasingBlockHeight(txBlockHeader)

	s.persistence.WriteBlock(input.BlockPair)

	s.lastCommittedBlockHeight = txBlockHeader.BlockHeight()
	s.lastCommittedBlockTimestamp = txBlockHeader.Timestamp()

	// TODO: why are we updating the state? nothing about this in the spec
	s.updateStateStorage(input.BlockPair.TransactionsBlock)

	return nil, nil
}

func (s *service) GetTransactionsBlockHeader(input *services.GetTransactionsBlockHeaderInput) (*services.GetTransactionsBlockHeaderOutput, error) {
	panic("Not implemented")
}

func (s *service) GetResultsBlockHeader(input *services.GetResultsBlockHeaderInput) (*services.GetResultsBlockHeaderOutput, error) {
	panic("Not implemented")
}

func (s *service) GetTransactionReceipt(input *services.GetTransactionReceiptInput) (*services.GetTransactionReceiptOutput, error) {
	panic("Not implemented")
}

func (s *service) GetLastCommittedBlockHeight(input *services.GetLastCommittedBlockHeightInput) (*services.GetLastCommittedBlockHeightOutput, error) {
	return &services.GetLastCommittedBlockHeightOutput{
		LastCommittedBlockHeight:    s.lastCommittedBlockHeight,
		LastCommittedBlockTimestamp: s.lastCommittedBlockTimestamp,
	}, nil
}

func (s *service) ValidateBlockForCommit(input *services.ValidateBlockForCommitInput) (*services.ValidateBlockForCommitOutput, error) {
	panic("Not implemented")
}

func (s *service) RegisterConsensusBlocksHandler(handler handlers.ConsensusBlocksHandler) {
	panic("Not implemented")
}

func (s *service) HandleBlockAvailabilityRequest(input *gossiptopics.BlockAvailabilityRequestInput) (*gossiptopics.EmptyOutput, error) {
	panic("Not implemented")
}

func (s *service) HandleBlockAvailabilityResponse(input *gossiptopics.BlockAvailabilityResponseInput) (*gossiptopics.EmptyOutput, error) {
	panic("Not implemented")
}
func (s *service) HandleBlockSyncRequest(input *gossiptopics.BlockSyncRequestInput) (*gossiptopics.EmptyOutput, error) {
	panic("Not implemented")
}
func (s *service) HandleBlockSyncResponse(input *gossiptopics.BlockSyncResponseInput) (*gossiptopics.EmptyOutput, error) {
	panic("Not implemented")
}

//TODO how do we check if block with same height is the same block? do we compare the block bit-by-bit? https://github.com/orbs-network/orbs-spec/issues/50
func (s *service) validateBlockDoesNotExist(txBlockHeader *protocol.TransactionsBlockHeader) bool {
	if txBlockHeader.BlockHeight() <= s.lastCommittedBlockHeight {
		if txBlockHeader.BlockHeight() == s.lastCommittedBlockHeight && txBlockHeader.Timestamp() != s.lastCommittedBlockTimestamp {
			// TODO should this really panic
			message := fmt.Sprintf("block with height %d already in storage, timestamp mismatch", s.lastCommittedBlockHeight)
			println(message)
			panic(message)
		}
		return false
	}

	return true
}

func (s *service) validateProtocolVersion(txBlockHeader *protocol.TransactionsBlockHeader) error {
	if txBlockHeader.ProtocolVersion() != ProtocolVersion {
		return fmt.Errorf("protocol version mismatch: expected 1 got %d", txBlockHeader.ProtocolVersion())
	}

	return nil
}

func (s *service) validateMonotonicIncreasingBlockHeight(txBlockHeader *protocol.TransactionsBlockHeader) {
	expectedNextBlockHeight := s.lastCommittedBlockHeight + 1
	if txBlockHeader.BlockHeight() != expectedNextBlockHeight {
		// TODO should this really panic
		message := fmt.Sprintf("expected block of height %d but got %d", expectedNextBlockHeight, txBlockHeader.BlockHeight())
		println(message)
		panic(message)
	}
}

func (s *service) updateStateStorage(txBlock *protocol.TransactionsBlockContainer) {
	var state []*protocol.StateRecordBuilder
	for _, i := range txBlock.SignedTransactions {
		byteArray := make([]byte, 8)
		binary.LittleEndian.PutUint64(byteArray, uint64(i.Transaction().InputArgumentsIterator().NextInputArguments().Uint64Value()))
		transactionStateDiff := &protocol.StateRecordBuilder{
			Key: primitives.Ripmd160Sha256("balance"),
			Value: byteArray,
		}
		state = append(state, transactionStateDiff)
	}
	csdi := []*protocol.ContractStateDiff{(&protocol.ContractStateDiffBuilder{StateDiffs: state, ContractName: "BenchmarkToken"}).Build()}
	s.stateStorage.CommitStateDiff(&services.CommitStateDiffInput{ContractStateDiffs: csdi})
}
