// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package gossip

import (
	"context"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/instrumentation/trace"
	"github.com/orbs-network/orbs-network-go/services/gossip/adapter"
	"github.com/orbs-network/orbs-network-go/synchronization/supervised"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol/gossipmessages"
	"github.com/orbs-network/orbs-spec/types/go/services/gossiptopics"
	"github.com/orbs-network/scribe/log"
	"sync"
)

var LogTag = log.Service("gossip")

type Config interface {
	NodeAddress() primitives.NodeAddress
	VirtualChainId() primitives.VirtualChainId
}

type gossipListeners struct {
	sync.RWMutex
	transactionHandlers        []gossiptopics.TransactionRelayHandler
	leanHelixHandlers          []gossiptopics.LeanHelixHandler
	benchmarkConsensusHandlers []gossiptopics.BenchmarkConsensusHandler
	blockSyncHandlers          []gossiptopics.BlockSyncHandler
}

type Service struct {
	supervised.TreeSupervisor

	config          Config
	logger          log.Logger
	transport       adapter.Transport
	handlers        gossipListeners
	headerValidator *headerValidator

	messageDispatcher *gossipMessageDispatcher
}

func NewGossip(ctx context.Context, transport adapter.Transport, config Config, parent log.Logger, metricRegistry metric.Registry) *Service {
	logger := parent.WithTags(LogTag)
	dispatcher := newMessageDispatcher(metricRegistry)
	s := &Service{
		transport:       transport,
		config:          config,
		logger:          logger,
		handlers:        gossipListeners{},
		headerValidator: newHeaderValidator(config, parent),

		messageDispatcher: dispatcher,
	}
	transport.RegisterListener(s, s.config.NodeAddress())
	s.SuperviseChan("Transaction Relay gossip topic", dispatcher.runHandler(ctx, logger, gossipmessages.HEADER_TOPIC_TRANSACTION_RELAY, s.receivedTransactionRelayMessage))
	s.SuperviseChan("Block Sync gossip topic", dispatcher.runHandler(ctx, logger, gossipmessages.HEADER_TOPIC_BLOCK_SYNC, s.receivedBlockSyncMessage))
	s.SuperviseChan("Lean Helix gossip topic", dispatcher.runHandler(ctx, logger, gossipmessages.HEADER_TOPIC_LEAN_HELIX, s.receivedLeanHelixMessage))
	s.SuperviseChan("Benchmark Consensus gossip topic", dispatcher.runHandler(ctx, logger, gossipmessages.HEADER_TOPIC_BENCHMARK_CONSENSUS, s.receivedBenchmarkConsensusMessage))

	return s
}

func (s *Service) OnTransportMessageReceived(ctx context.Context, payloads [][]byte) {
	select {
	case <-ctx.Done():
		return
	default:
		logger := s.logger.WithTags(trace.LogFieldFrom(ctx))
		if len(payloads) == 0 {
			logger.Error("transport did not receive any payloads, header missing")
			return
		}
		header := gossipmessages.HeaderReader(payloads[0])
		if !header.IsValid() {
			logger.Error("transport header is corrupt", log.Bytes("header", payloads[0]))
			return
		}

		if err := s.headerValidator.validateMessageHeader(header); err != nil {
			logger.Error("dropping a received message that isn't valid", log.Error(err), log.Stringable("message-header", header))
			return
		}

		logger.Info("transport message received", log.Stringable("header", header), log.String("gossip-topic", header.StringTopic()))
		s.messageDispatcher.dispatch(logger, header, payloads[1:])
	}
}
