package serializer

import (
	"context"
	"fmt"
	"github.com/orbs-network/govnr"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/services/statestorage/adapter"
	"github.com/orbs-network/orbs-network-go/services/statestorage/adapter/memory"
	"github.com/orbs-network/orbs-network-go/synchronization/supervised"
	"github.com/orbs-network/scribe/log"
	"io/ioutil"
	"path/filepath"
	"time"
)

type MemoryPersistenceWrapper interface {
	govnr.ShutdownWaiter
	supervised.GracefulShutdowner
	adapter.StatePersistence
}

type memoryPersistenceWrapper struct {
	config config.NodeConfig
	logger log.Logger

	*memory.InMemoryStatePersistence
}

func NewInMemoryPersistenceWrapper(config config.NodeConfig, logger log.Logger, registry metric.Registry) MemoryPersistenceWrapper {
	wrapper := &memoryPersistenceWrapper{
		config:                   config,
		logger:                   logger,
		InMemoryStatePersistence: memory.NewStatePersistence(registry),
	}
	wrapper.loadStatePersistence()

	return wrapper
}

func (w *memoryPersistenceWrapper) loadStatePersistence() {
	start := time.Now()

	dump, err := ioutil.ReadFile(w.getStateFilePath())
	if err != nil {
		w.logger.Error("failed to read state file", log.Error(err))
		return
	}

	if err := NewStatePersistenceDeserializer(w.InMemoryStatePersistence).Deserialize(dump); err != nil {
		w.logger.Error("failed to deserialize state file", log.Error(err))
		return
	} else {
		w.logger.Info("finished loading the state", log.String("duration", time.Since(start).String()))
	}

	fmt.Println("loading the state ")

	return
}

func (w *memoryPersistenceWrapper) getStateFilePath() string {
	return filepath.Join(w.config.BlockStorageFileSystemDataDir(), "state")
}

func (w *memoryPersistenceWrapper) WaitUntilShutdown(shutdownContext context.Context) {

}

func (w *memoryPersistenceWrapper) GracefulShutdown(shutdownContext context.Context) {
	start := time.Now()
	dump, err := NewStatePersistenceSerializer(w.InMemoryStatePersistence).Dump()
	if err != nil {
		w.logger.Error("failed to serialize state file", log.Error(err))
		return
	}

	if err := ioutil.WriteFile(w.getStateFilePath(), dump, 0644); err != nil {
		w.logger.Error("failed not write state file", log.Error(err))
		return
	}

	fmt.Println("serialized the state")

	w.logger.Info("successfully serialized the state", log.String("duration", time.Since(start).String()))
}
