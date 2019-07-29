// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package tcp

import (
	"context"
	"fmt"
	"github.com/orbs-network/membuffers/go"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/instrumentation/trace"
	"github.com/orbs-network/orbs-network-go/services/gossip/adapter"
	"github.com/orbs-network/orbs-network-go/synchronization/supervised"
	"github.com/orbs-network/scribe/log"
	"github.com/pkg/errors"
	"net"
	"time"
)

type clientConnectionConfig interface {
	GossipNetworkTimeout() time.Duration
	GossipReconnectInterval() time.Duration
	GossipConnectionKeepAliveInterval() time.Duration
}

type clientConnection struct {
	logger          log.Logger
	config          clientConnectionConfig
	sharedMetrics   *metrics // TODO this is smelly, see how we can restructure metrics so that a client connection doesn't have to share the Transport's metrics
	queue           *transportQueue
	peerNodeAddress string
	disconnect      context.CancelFunc

	sendErrors      *metric.Gauge
	sendQueueErrors *metric.Gauge
}

func newClientConnection(peerHexAddress string, peer config.GossipPeer, parentLogger log.Logger, metricFactory metric.Registry, sharedMetrics *metrics, transportConfig config.GossipTransportConfig) *clientConnection {
	peerAddress := fmt.Sprintf("%s:%d", peer.GossipEndpoint(), peer.GossipPort())
	queue := NewTransportQueue(SEND_QUEUE_MAX_BYTES, SEND_QUEUE_MAX_MESSAGES, metricFactory, peerHexAddress)
	queue.networkAddress = peerAddress
	queue.Disable() // until connection is established

	client := &clientConnection{
		logger:          parentLogger.WithTags(log.String("peer-node-address", peerHexAddress), log.String("peer-network-address", peerAddress)),
		sharedMetrics:   sharedMetrics,
		config:          transportConfig,
		queue:           queue,
		sendErrors:      metricFactory.NewGauge(fmt.Sprintf("Gossip.OutgoingConnection.SendError.%s.Count", peerHexAddress)),
		sendQueueErrors: metricFactory.NewGauge(fmt.Sprintf("Gossip.OutgoingConnection.EnqueueErrors.%s.Count", peerHexAddress)),
	}

	return client
}

func (c *clientConnection) connect(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	c.disconnect = cancel

	supervised.GoForever(ctx, c.logger, func() {
		c.clientMainLoop(ctx)
	})
}

func (c *clientConnection) clientMainLoop(parentCtx context.Context) {
	for {
		ctx := trace.NewContext(parentCtx, fmt.Sprintf("Gossip.Transport.TCP.Client.%s", c.peerNodeAddress))
		logger := c.logger.WithTags(trace.LogFieldFrom(ctx))

		logger.Info("attempting outgoing transport connection")
		conn, err := net.DialTimeout("tcp", c.queue.networkAddress, c.config.GossipNetworkTimeout())

		if err != nil {
			logger.Info("cannot connect to gossip peer endpoint")
			time.Sleep(c.config.GossipReconnectInterval())
			continue
		}

		if !c.clientHandleOutgoingConnection(ctx, conn) {
			return
		}
	}
}

// returns true if should attempt reconnect on error
func (c *clientConnection) clientHandleOutgoingConnection(ctx context.Context, conn net.Conn) bool {
	logger := c.logger.WithTags(trace.LogFieldFrom(ctx))
	logger.Info("successful outgoing gossip transport connection")

	c.sharedMetrics.activeOutgoingConnections.Inc()
	defer c.sharedMetrics.activeOutgoingConnections.Dec()

	c.queue.OnNewConnection(ctx)
	defer c.queue.Disable()

	defer conn.Close() // we only exit this function when this connection has errored, or if we're disconnecting, so we can safely defer close()

	for {
		if data := c.popMessageFromQueue(ctx); data != nil {
			// got data from queue
			err := c.sendToSocket(ctx, conn, data)
			if err != nil {
				return c.reconnectAfterSocketError(logger, err)
			} // else - continue looping

		} else if shouldKeepAlive(ctx) {
			err := c.sendKeepAlive(ctx, conn)
			if err != nil {
				return c.reconnectAfterKeepAliveFailure(logger, err)
			}

		} else {
			// parent ctx is closed, we're disconnecting
			return c.onDisconnect(logger)
		}
	}
}

func (c *clientConnection) popMessageFromQueue(ctx context.Context) *adapter.TransportData {
	ctxWithKeepAliveTimeout, cancelCtxWithKeepAliveTimeout := context.WithTimeout(ctx, c.config.GossipConnectionKeepAliveInterval())
	defer cancelCtxWithKeepAliveTimeout()

	return c.queue.Pop(ctxWithKeepAliveTimeout)
}

// if this context is not closed, we send keep alive; otherwise - we're asked to disconnect
func shouldKeepAlive(ctx context.Context) bool {
	return ctx.Err() == nil
}

func (c *clientConnection) onDisconnect(logger log.Logger) bool {
	logger.Info("client loop stopped since a disconnect was requested (topology change or system shutdown)")
	return false
}

func (c *clientConnection) reconnectAfterKeepAliveFailure(logger log.Logger, err error) bool {
	c.sharedMetrics.outgoingConnectionKeepaliveErrors.Inc()
	logger.Info("failed sending keepalive, reconnecting", log.Error(err))
	return true
}

func (c *clientConnection) reconnectAfterSocketError(logger log.Logger, err error) bool {
	c.sharedMetrics.outgoingConnectionSendErrors.Inc() //TODO remove, replaced by following metric
	c.sendErrors.Inc()
	logger.Info("failed sending transport data, reconnecting", log.Error(err))
	return true
}

func (c *clientConnection) addDataToOutgoingPeerQueue(ctx context.Context, data *adapter.TransportData) {
	err := c.queue.Push(data)
	if err != nil {
		c.sharedMetrics.outgoingConnectionSendQueueErrors.Inc() //TODO remove, replaced by following metric
		c.sendQueueErrors.Inc()
		c.logger.Info("direct transport send queue error", log.Error(err), trace.LogFieldFrom(ctx))
	}
}

func (c *clientConnection) sendToSocket(ctx context.Context, conn net.Conn, data *adapter.TransportData) error {
	timeout := c.config.GossipNetworkTimeout()
	zeroBuffer := make([]byte, 4)
	sizeBuffer := make([]byte, 4)

	// send num payloads
	membuffers.WriteUint32(sizeBuffer, uint32(len(data.Payloads)))
	err := write(ctx, conn, sizeBuffer, timeout)
	if err != nil {
		return err
	}

	for _, payload := range data.Payloads {
		// send payload size
		membuffers.WriteUint32(sizeBuffer, uint32(len(payload)))
		err := write(ctx, conn, sizeBuffer, timeout)
		if err != nil {
			return err
		}

		// send payload data
		err = write(ctx, conn, payload, timeout)
		if err != nil {
			return err
		}

		// send padding
		paddingSize := calcPaddingSize(uint32(len(payload)))
		if paddingSize > 0 {
			err = write(ctx, conn, zeroBuffer[:paddingSize], timeout)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *clientConnection) sendKeepAlive(ctx context.Context, conn net.Conn) error {
	timeout := c.config.GossipNetworkTimeout()
	zeroBuffer := make([]byte, 4)

	// send zero num payloads
	err := write(ctx, conn, zeroBuffer, timeout)
	if err != nil {
		return err
	}

	return nil
}

func write(ctx context.Context, conn net.Conn, buffer []byte, timeout time.Duration) error {
	// TODO(https://github.com/orbs-network/orbs-network-go/issues/182): consider whether the right approach is to poll context this way or have a single watchdog goroutine that closes all active connections when context is cancelled
	// make sure context is still open
	err := ctx.Err()
	if err != nil {
		return err
	}

	err = conn.SetWriteDeadline(time.Now().Add(timeout))
	if err != nil {
		return err
	}
	written, err := conn.Write(buffer)
	if written != len(buffer) {
		if err == nil {
			return errors.Errorf("attempted to write %d bytes but only wrote %d", len(buffer), written)
		} else {
			return errors.Wrapf(err, "attempted to write %d bytes but only wrote %d", len(buffer), written)
		}
	}
	return nil
}
