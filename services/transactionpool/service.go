// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package transactionpool

import (
	"github.com/orbs-network/govnr"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/services/transactionpool/adapter"
	"github.com/orbs-network/orbs-network-go/synchronization"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/orbs-spec/types/go/services/gossiptopics"
	"github.com/orbs-network/orbs-spec/types/go/services/handlers"
	"github.com/orbs-network/scribe/log"
	"sync"
)

var LogTag = log.Service("transaction-pool")

type BlockHeightReporter interface {
	IncrementTo(height primitives.BlockHeight)
}

type Service struct {
	govnr.TreeSupervisor

	clock               adapter.Clock
	gossip              gossiptopics.TransactionRelay
	virtualMachine      services.VirtualMachine
	blockHeightReporter BlockHeightReporter // used to allow test to wait for a block height to reach the transaction pool
	logger              log.Logger
	config              config.TransactionPoolConfig

	transactionResultsHandlers struct {
		sync.RWMutex
		handlers []handlers.TransactionResultsHandler
	}

	lastCommitted struct {
		sync.RWMutex
		blockHeight primitives.BlockHeight
		timestamp   primitives.TimestampNano
	}

	pendingPool                         *pendingTxPool
	committedPool                       *committedTxPool
	blockTracker                        *synchronization.BlockTracker
	transactionForwarder                *transactionForwarder
	transactionWaiter                   *transactionWaiter
	validationContext                   *validationContext
	addNewTransactionConcurrencyLimiter *requestConcurrencyLimiter

	metrics struct {
		blockHeight *metric.Gauge
		commitRate  *metric.Rate
		commitCount *metric.Gauge
	}

	addCommitLock sync.RWMutex
}

func (s *Service) lastCommittedBlockHeightAndTime() (primitives.BlockHeight, primitives.TimestampNano) {
	s.lastCommitted.RLock()
	defer s.lastCommitted.RUnlock()
	return s.lastCommitted.blockHeight, s.lastCommitted.timestamp
}

func (s *Service) createValidationContext() *validationContext {
	return &validationContext{
		expiryWindow:           s.config.TransactionExpirationWindow(),
		nodeSyncRejectInterval: s.config.TransactionPoolNodeSyncRejectTime(),
		futureTimestampGrace:   s.config.TransactionPoolFutureTimestampGraceTimeout(),
		virtualChainId:         s.config.VirtualChainId(),
	}
}
