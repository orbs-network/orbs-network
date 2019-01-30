package transactionpool

import (
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/instrumentation/log"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/synchronization"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/orbs-spec/types/go/services/gossiptopics"
	"github.com/orbs-network/orbs-spec/types/go/services/handlers"
	"sync"
	"time"
)

var LogTag = log.Service("transaction-pool")

type BlockHeightReporter interface {
	IncrementTo(height primitives.BlockHeight)
}

type service struct {
	gossip                     gossiptopics.TransactionRelay
	virtualMachine             services.VirtualMachine
	blockHeightReporter        BlockHeightReporter // used to allow test to wait for a block height to reach the transaction pool
	transactionResultsHandlers []handlers.TransactionResultsHandler
	logger                     log.BasicLogger
	config                     config.TransactionPoolConfig

	lastCommitted struct {
		sync.RWMutex
		blockHeight primitives.BlockHeight
		timestamp   primitives.TimestampNano
	}

	pendingPool          *pendingTxPool
	committedPool        *committedTxPool
	blockTracker         *synchronization.BlockTracker
	transactionForwarder *transactionForwarder
	transactionWaiter    *transactionWaiter

	metrics struct {
		blockHeight *metric.Gauge
	}

	addCommitLock sync.RWMutex
}

func (s *service) lastCommittedBlockHeightAndTime() (primitives.BlockHeight, primitives.TimestampNano) {
	s.lastCommitted.RLock()
	defer s.lastCommitted.RUnlock()
	return s.lastCommitted.blockHeight, s.lastCommitted.timestamp
}

func (s *service) createValidationContext() *validationContext {
	s.lastCommitted.RLock()
	defer s.lastCommitted.RUnlock()
	return &validationContext{
		nodeTime:                    time.Now(),
		lastCommittedBlockTimestamp: s.lastCommitted.timestamp,
		expiryWindow:                s.config.TransactionExpirationWindow(),
		nodeSyncRejectInterval:      s.config.TransactionPoolNodeSyncRejectTime(),
		futureTimestampGrace:        s.config.TransactionPoolFutureTimestampGraceTimeout(),
		virtualChainId:              s.config.VirtualChainId(),
	}
}
