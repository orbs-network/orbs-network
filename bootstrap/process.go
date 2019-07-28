// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package bootstrap

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/orbs-network/govnr"
	"github.com/orbs-network/orbs-network-go/bootstrap/httpserver"
	"github.com/orbs-network/scribe/log"
)

type OrbsProcess struct {
	CancelFunc   context.CancelFunc
	Logger       log.Logger
	HttpServer   httpserver.HttpServer
	shutdownCond *sync.Cond
}

func NewOrbsProcess(logger log.Logger, cancelFunc context.CancelFunc, httpserver httpserver.HttpServer) OrbsProcess {
	return OrbsProcess{
		shutdownCond: sync.NewCond(&sync.Mutex{}),
		Logger:       logger,
		CancelFunc:   cancelFunc,
		HttpServer:   httpserver,
	}
}

func (n *OrbsProcess) GracefulShutdown(timeout time.Duration) {
	n.CancelFunc()
	n.HttpServer.GracefulShutdown(timeout)
	n.shutdownCond.Broadcast()
}

func (n *OrbsProcess) WaitUntilShutdown() {
	// if waiting for shutdown, listen for sigint and sigterm
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	govnr.GoOnce(n.Logger, func() {
		<-signalChan
		n.Logger.Info("terminating node gracefully due to os signal received")
		n.GracefulShutdown(0)
	})

	n.shutdownCond.L.Lock()
	n.shutdownCond.Wait()
	n.shutdownCond.L.Unlock()
}
