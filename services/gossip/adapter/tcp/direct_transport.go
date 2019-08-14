// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package tcp

import (
	"context"
	"github.com/orbs-network/govnr"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/instrumentation/logfields"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/services/gossip/adapter"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol/gossipmessages"
	"github.com/orbs-network/scribe/log"
	"github.com/pkg/errors"
	"sync"
	"sync/atomic"
)

const MAX_PAYLOADS_IN_MESSAGE = 100000
const MAX_PAYLOAD_SIZE_BYTES = 20 * 1024 * 1024
const SEND_QUEUE_MAX_MESSAGES = 1000
const SEND_QUEUE_MAX_BYTES = 20 * 1024 * 1024

var LogTag = log.String("adapter", "gossip")

type metrics struct {
	incomingConnectionAcceptSuccesses *metric.Gauge
	incomingConnectionAcceptErrors    *metric.Gauge
	incomingConnectionTransportErrors *metric.Gauge
	outgoingConnectionSendErrors      *metric.Gauge
	outgoingConnectionKeepaliveErrors *metric.Gauge
	outgoingConnectionSendQueueErrors *metric.Gauge

	activeIncomingConnections *metric.Gauge
	activeOutgoingConnections *metric.Gauge

	outgoingMessageSize *metric.Histogram
}

type lockableClientConnections struct {
	sync.RWMutex
	peers map[string]*clientConnection
}

type lockableTransportServer struct {
	sync.RWMutex
	listener  adapter.TransportListener
	listening bool
	port      int
}

type DirectTransport struct {
	atomicConfig atomic.Value
	logger       log.Logger

	clientConnections *lockableClientConnections

	server *lockableTransportServer

	metrics        *metrics
	metricRegistry metric.Registry
	serverClosed   chan struct{}
	cancelServer   context.CancelFunc
}

func getMetrics(registry metric.Registry) *metrics {
	return &metrics{
		incomingConnectionAcceptSuccesses: registry.NewGauge("Gossip.IncomingConnection.ListeningOnTCPPortSuccess.Count"),
		incomingConnectionAcceptErrors:    registry.NewGauge("Gossip.IncomingConnection.ListeningOnTCPPortErrors.Count"),
		incomingConnectionTransportErrors: registry.NewGauge("Gossip.IncomingConnection.TransportErrors.Count"),
		outgoingConnectionSendErrors:      registry.NewGauge("Gossip.OutgoingConnection.SendErrors.Count"),
		outgoingConnectionKeepaliveErrors: registry.NewGauge("Gossip.OutgoingConnection.KeepaliveErrors.Count"),
		outgoingConnectionSendQueueErrors: registry.NewGauge("Gossip.OutgoingConnection.SendQueueErrors.Count"),
		activeIncomingConnections:         registry.NewGauge("Gossip.IncomingConnection.Active.Count"),
		activeOutgoingConnections:         registry.NewGauge("Gossip.OutgoingConnection.Active.Count"),
		outgoingMessageSize:               registry.NewHistogram("Gossip.OutgoingConnection.MessageSize.Bytes", MAX_PAYLOAD_SIZE_BYTES),
	}
}

func NewDirectTransport(parent context.Context, config config.GossipTransportConfig, logger log.Logger, registry metric.Registry) *DirectTransport {
	serverCtx, cancelServer := context.WithCancel(parent)

	t := &DirectTransport{
		logger:         logger.WithTags(LogTag),
		metricRegistry: registry,

		clientConnections: newLockableClientConnections(),
		server:            newDirectTransportServer(),

		metrics: getMetrics(registry),

		cancelServer: cancelServer,
	}

	t.atomicConfig.Store(config)

	// server goroutine
	handle := govnr.Forever(serverCtx, "TCP server", logfields.GovnrErrorer(t.logger), func() {
		t.serverMainLoop(serverCtx, config.GossipListenPort())
	})
	t.serverClosed = handle.Done()
	handle.MarkSupervised() // TODO use real supervision

	// client goroutines
	for peerNodeAddress, peer := range config.GossipPeers() {
		t.connectForever(parent, peerNodeAddress, peer)
	}

	return t
}

func newDirectTransportServer() *lockableTransportServer {
	return &lockableTransportServer{
		listener:  nil,
		listening: false,
		port:      0,
	}
}

func newLockableClientConnections() *lockableClientConnections {
	return &lockableClientConnections{
		peers: make(map[string]*clientConnection),
	}
}

func (t *DirectTransport) config() config.GossipTransportConfig {
	if c, ok := t.atomicConfig.Load().(config.GossipTransportConfig); ok {
		return c
	}

	return nil
}

// note that bgCtx MUST be a long-running background context - if it's a short lived context, the new connection will die as soon as
// the context is done
func (t *DirectTransport) connectForever(bgCtx context.Context, peerNodeAddress string, peer config.GossipPeer) {
	t.clientConnections.Lock()
	defer t.clientConnections.Unlock()

	config := t.config()
	if peerNodeAddress != config.NodeAddress().KeyForMap() {
		client := newClientConnection(peer, t.logger, t.metricRegistry, t.metrics, config)

		t.clientConnections.peers[peerNodeAddress] = client

		client.connect(bgCtx)

	}
}

func (t *DirectTransport) AddPeer(bgCtx context.Context, address primitives.NodeAddress, peer config.GossipPeer) {
	t.config().GossipPeers()[address.KeyForMap()] = peer
	t.connectForever(bgCtx, address.KeyForMap(), peer)
}

func (t *DirectTransport) UpdateTopology(bgCtx context.Context, newConfig config.GossipTransportConfig) {

	oldConfig := t.config()

	peersToRemove, peersToAdd := peerDiff(oldConfig.GossipPeers(), newConfig.GossipPeers())

	t.disconnectAllClients(bgCtx, peersToRemove)

	t.atomicConfig.Store(newConfig)
	for peerNodeAddress, peer := range peersToAdd {
		t.connectForever(bgCtx, peerNodeAddress, peer)
	}
}

func (t *DirectTransport) disconnectAllClients(ctx context.Context, peersToDisconnect gossipPeers) {
	t.clientConnections.Lock()
	defer t.clientConnections.Unlock()
	for key, peer := range peersToDisconnect {
		if client, found := t.clientConnections.peers[key]; found {
			select {
			case <-client.disconnect():
				delete(t.clientConnections.peers, key)
			case <-ctx.Done():
				t.logger.Info("system shutdown while waiting for clients to disconnect")
			}
		} else {
			t.logger.Error("attempted to disconnect a client that was not connected", log.String("missing-peer", peer.HexOrbsAddress()))
		}

	}
}

func (t *DirectTransport) RegisterListener(listener adapter.TransportListener, listenerNodeAddress primitives.NodeAddress) {
	t.server.Lock()
	defer t.server.Unlock()

	t.server.listener = listener
}

// TODO(https://github.com/orbs-network/orbs-network-go/issues/182): we are not currently respecting any intents given in ctx (added in context refactor)
func (t *DirectTransport) Send(ctx context.Context, data *adapter.TransportData) error {
	t.clientConnections.RLock()
	defer t.clientConnections.RUnlock()

	switch data.RecipientMode {
	case gossipmessages.RECIPIENT_LIST_MODE_BROADCAST:
		for _, client := range t.clientConnections.peers {
			client.addDataToOutgoingPeerQueue(ctx, data)
			t.metrics.outgoingMessageSize.Record(int64(data.TotalSize()))
		}
		return nil
	case gossipmessages.RECIPIENT_LIST_MODE_LIST:
		for _, recipientPublicKey := range data.RecipientNodeAddresses {
			if client, found := t.clientConnections.peers[recipientPublicKey.KeyForMap()]; found {
				client.addDataToOutgoingPeerQueue(ctx, data)
				t.metrics.outgoingMessageSize.Record(int64(data.TotalSize()))
			} else {
				return errors.Errorf("unknown recipient public key: %s", recipientPublicKey.String())
			}
		}
		return nil
	case gossipmessages.RECIPIENT_LIST_MODE_ALL_BUT_LIST:
		panic("Not implemented")
	}
	return errors.Errorf("unknown recipient mode: %s", data.RecipientMode.String())
}

func (t *DirectTransport) GetServerPort() int {
	t.server.Lock()
	defer t.server.Unlock()

	return t.server.port
}

func (t *DirectTransport) setServerPort(v int) {
	t.server.Lock()
	defer t.server.Unlock()

	t.server.port = v
}

func (t *DirectTransport) allOutgoingQueuesEnabled() bool {
	t.clientConnections.RLock()
	defer t.clientConnections.RUnlock()

	for _, client := range t.clientConnections.peers {
		if client.queue.disabled() {
			return false
		}
	}
	return true
}

func (t *DirectTransport) GracefulShutdown(shutdownContext context.Context) {
	t.logger.Info("Shutting down")
	t.clientConnections.Lock()
	defer t.clientConnections.Unlock()
	for _, client := range t.clientConnections.peers {
		client.disconnect()
	}
	t.cancelServer()
}

func (t *DirectTransport) WaitUntilShutdown(shutdownContext context.Context) {
	t.clientConnections.Lock()
	defer t.clientConnections.Unlock()
	for _, client := range t.clientConnections.peers {
		t.waitFor(shutdownContext, client.closed)
	}
	t.waitFor(shutdownContext, t.serverClosed)
}

func (t *DirectTransport) waitFor(shutdownContext context.Context, closed chan struct{}) {
	select {
	case <-closed:
	case <-shutdownContext.Done():
		t.logger.Error("failed shutting down within shutdown context")
	}
}

func calcPaddingSize(size uint32) uint32 {
	const contentAlignment = 4
	alignedSize := (size + contentAlignment - 1) / contentAlignment * contentAlignment
	return alignedSize - size
}
