package adapter

import (
	"github.com/orbs-network/orbs-network-go/instrumentation"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
)

type BlockPersistence interface {
	WithLogger(reporting instrumentation.BasicLogger) BlockPersistence
	WriteBlock(blockPairs *protocol.BlockPairContainer) error
	ReadAllBlocks() []*protocol.BlockPairContainer
	GetLastBlockDetails() (primitives.BlockHeight, primitives.TimestampNano)
	GetTransactionsBlock(height primitives.BlockHeight) (*protocol.TransactionsBlockContainer, error)
	GetResultsBlock(height primitives.BlockHeight) (*protocol.ResultsBlockContainer, error)
}
