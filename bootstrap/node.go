// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package bootstrap

import (
	"context"
	"fmt"
	"github.com/orbs-network/govnr"
	"github.com/orbs-network/orbs-network-go/bootstrap/httpserver"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/instrumentation/logfields"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/services/blockstorage/adapter/filesystem"
	ethereumAdapter "github.com/orbs-network/orbs-network-go/services/crosschainconnector/ethereum/adapter"
	"github.com/orbs-network/orbs-network-go/services/gossip/adapter/tcp"
	"github.com/orbs-network/orbs-network-go/services/management"
	managementAdapter "github.com/orbs-network/orbs-network-go/services/management/adapter"
	nativeProcessorAdapter "github.com/orbs-network/orbs-network-go/services/processor/native/adapter"
	"github.com/orbs-network/orbs-network-go/services/statestorage/adapter/serializer"
	txPoolAdapter "github.com/orbs-network/orbs-network-go/services/transactionpool/adapter"
	"github.com/orbs-network/orbs-network-go/synchronization/supervised"
	"github.com/orbs-network/scribe/log"
	"github.com/pkg/errors"
)

type Node struct {
	govnr.TreeSupervisor
	logic            NodeLogic
	cancelFunc       context.CancelFunc
	httpServer       *httpserver.HttpServer
	transport        *tcp.DirectTransport
	logger           log.Logger
	blockPersistence *filesystem.BlockPersistence
	statePersistence serializer.MemoryPersistenceWrapper
}

func GetMetricRegistry(nodeConfig config.NodeConfig) metric.Registry {
	metricRegistry := metric.NewRegistry().WithVirtualChainId(nodeConfig.VirtualChainId()).WithNodeAddress(nodeConfig.NodeAddress())

	version := config.GetVersion()

	metricRegistry.NewText("Node.Address", nodeConfig.NodeAddress().String())
	metricRegistry.NewText("Version.Semantic", version.Semantic)
	metricRegistry.NewText("Version.Commit", version.Commit)

	return metricRegistry
}

func NewNode(nodeConfig config.NodeConfig, logger log.Logger) *Node {
	ctx, ctxCancel := context.WithCancel(context.Background())

	nodeLogger := logger.WithTags(
		log.Node(nodeConfig.NodeAddress().String()),
		logfields.VirtualChainId(nodeConfig.VirtualChainId()),
	)
	metricRegistry := GetMetricRegistry(nodeConfig)

	httpServer := httpserver.NewHttpServer(nodeConfig, nodeLogger, metricRegistry)

	transport := tcp.NewDirectTransport(ctx, nodeConfig, nodeLogger, metricRegistry)

	var managementProvider management.Provider
	if nodeConfig.ManagementFilePath() == "" {
		err := errors.New("ManagementFilePath is empty")
		nodeLogger.Error("Cannot start node without a ManagementFilePath", log.Error(err))
		panic(err)
	} else {
		managementProvider = managementAdapter.NewFileProvider(nodeConfig)
	}

	blockPersistence, err := filesystem.NewBlockPersistence(nodeConfig, nodeLogger, metricRegistry)
	if err != nil {
		panic(fmt.Sprintf("failed initializing blocks database, err=%s", err.Error()))
	}

	statePersistence := serializer.NewInMemoryPersistenceWrapper(nodeConfig, logger, metricRegistry)
	ethereumConnection := ethereumAdapter.NewEthereumRpcConnection(nodeConfig, logger, metricRegistry)
	nativeCompiler := nativeProcessorAdapter.NewNativeCompiler(nodeConfig, nodeLogger, metricRegistry)
	nodeLogic := NewNodeLogic(ctx,
		transport, blockPersistence, statePersistence, nil, nil, txPoolAdapter.NewSystemClock(), nativeCompiler, managementProvider,
		nodeLogger, metricRegistry, nodeConfig, ethereumConnection)

	httpServer.RegisterPublicApi(nodeLogic.PublicApi())

	n := &Node{
		logger:           nodeLogger,
		cancelFunc:       ctxCancel,
		logic:            nodeLogic,
		transport:        transport,
		httpServer:       httpServer,
		blockPersistence: blockPersistence,
		statePersistence: statePersistence,
	}

	// TODO re-enable Ethereum access (with refTime based finality)
	// ethereumConnection.ReportConnectionStatus(ctx)

	n.Supervise(statePersistence)
	n.Supervise(ethereumConnection)
	n.Supervise(nodeLogic)
	n.Supervise(transport)
	n.Supervise(httpServer)
	return n
}

func (n *Node) GracefulShutdown(shutdownContext context.Context) {
	n.logger.Info("Shutting down")
	n.cancelFunc()
	supervised.ShutdownAllGracefully(shutdownContext, n.httpServer, n.transport, n.blockPersistence, n.statePersistence)
}
