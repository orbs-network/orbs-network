package serializer

import (
	"context"
	"fmt"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/services/blockstorage/adapter/filesystem"
	"github.com/orbs-network/orbs-network-go/services/blockstorage/servicesync"
	"github.com/orbs-network/orbs-network-go/services/statestorage"
	"github.com/orbs-network/orbs-network-go/services/statestorage/adapter/memory"
	"github.com/orbs-network/orbs-network-go/synchronization"
	"github.com/orbs-network/orbs-network-go/test"
	"github.com/orbs-network/orbs-network-go/test/with"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/scribe/log"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"testing"
	"time"
)

type localConfig struct {
	dir         string
	chainId     primitives.VirtualChainId
	networkType protocol.SignerNetworkType
}

func (l *localConfig) BlockStorageFileSystemDataDir() string {
	return l.dir
}

func (l *localConfig) BlockStorageFileSystemMaxBlockSizeInBytes() uint32 {
	return 64 * 1024 * 1024
}

func (l *localConfig) VirtualChainId() primitives.VirtualChainId {
	return l.chainId
}

func (l *localConfig) NetworkType() protocol.SignerNetworkType {
	return l.networkType
}

func TestServiceSyncManually(t *testing.T) {
	//t.Skip("manual test")

	with.Context(func(ctx context.Context) {
		logger := log.GetLogger().WithFilters(log.DiscardAll())
		metricFactory := metric.NewRegistry()

		persistence, blockHeight := getBlockPersistence(t, logger, metricFactory)

		blockTracker := synchronization.NewBlockTracker(logger, 0, 10)
		inmemory := memory.NewStatePersistence(metricFactory)
		stateStorageConfig := config.ForStateStorageTest(5, 5, 100)
		stateStorage := statestorage.NewStateStorage(stateStorageConfig, inmemory, blockTracker, logger, metricFactory)
		committer := servicesync.NewStateStorageCommitter(stateStorage)

		servicesync.NewServiceBlockSync(ctx, logger, persistence, committer)

		syncStart := time.Now()
		require.Eventually(t, func() bool {
			inmemoryBlockHeight, _, _, _, _, _, err := inmemory.ReadMetadata()
			fmt.Println(fmt.Sprintf("stateBH/persistentBH %d/%d", inmemoryBlockHeight, blockHeight))
			fmt.Println("time elapsed", time.Since(syncStart).String())
			return err == nil && inmemoryBlockHeight >= blockHeight-100 // gets stuck on last 5 blocks for whatever reason
		}, 10*time.Minute, 1*time.Second)
		fmt.Println("sync time", time.Since(syncStart).String())

		dumpStart := time.Now()
		dump, err := NewStatePersistenceSerializer(inmemory).Dump()
		fmt.Println("dump time", time.Since(dumpStart).String())
		require.NoError(t, err)

		dumpWriteTime := time.Now()
		err = ioutil.WriteFile("./dump.bin", dump, 0644)
		fmt.Println("dump writing time", time.Since(dumpWriteTime).String())
		require.NoError(t, err)
	})
}

func TestServiceSyncStartupManually(t *testing.T) {
	t.Skip("manual test")

	with.Context(func(ctx context.Context) {
		dump, err := ioutil.ReadFile("./dump.bin")
		require.NoError(t, err)

		metricFactory := metric.NewRegistry()
		inmemory := memory.NewStatePersistence(metricFactory)
		err = NewStatePersistenceDeserializer(inmemory).Deserialize(dump)
		require.NoError(t, err)

		fmt.Println(inmemory.Dump())

		logger := log.GetLogger().WithFilters(log.DiscardAll())

		persistence, blockHeight := getBlockPersistence(t, logger, metricFactory)

		blockTracker := synchronization.NewBlockTracker(logger, 0, 10)
		stateStorageConfig := config.ForStateStorageTest(5, 5, 100)
		stateStorage := statestorage.NewStateStorage(stateStorageConfig, inmemory, blockTracker, logger, metricFactory)
		committer := servicesync.NewStateStorageCommitter(stateStorage)

		servicesync.NewServiceBlockSync(ctx, logger, persistence, committer)

		syncStart := time.Now()
		test.Eventually(10*time.Second, func() bool {
			inmemoryBlockHeight, _, _, _, _, _, err := inmemory.ReadMetadata()
			fmt.Println(fmt.Sprintf("stateBH/persistentBH %d/%d", inmemoryBlockHeight, blockHeight))
			fmt.Println("time elapsed", time.Since(syncStart).String())
			return err == nil && inmemoryBlockHeight >= blockHeight-100 // gets stuck on last 5 blocks for whatever reason
		})
		fmt.Println("sync time", time.Since(syncStart).String())
	})

}

func getBlockPersistence(t *testing.T, logger log.Logger, metricFactory metric.Factory) (*filesystem.BlockPersistence, primitives.BlockHeight) {
	start := time.Now()

	//vcid, _ := strconv.ParseInt(os.Getenv("VCHAIN"), 10, 32)
	//blocksPath := os.Getenv("BLOCKS_DIR")
	//
	//persistence, err := filesystem.NewBlockPersistence(&localConfig{
	//	chainId:     primitives.VirtualChainId(vcid),
	//	networkType: protocol.NETWORK_TYPE_RESERVED,
	//	dir:         blocksPath,
	//}, logger, metricFactory)

	//persistence, err := filesystem.NewBlockPersistence(&localConfig{
	//	chainId:     1960000,
	//	networkType: protocol.NETWORK_TYPE_RESERVED,
	//	dir:         "/Users/kirill/Downloads",
	//}, logger, metricFactory)

	persistence, err := filesystem.NewBlockPersistence(&localConfig{
		chainId:     1100000,
		networkType: protocol.NETWORK_TYPE_RESERVED,
		dir:         "/usr/local/var/orbs",
	}, logger, metricFactory)

	fmt.Println("startup time", time.Since(start).String())

	if err != nil {
		t.Fatal("failed to start processing", err)
	}

	blockHeight, err := persistence.GetLastBlockHeight()
	if err != nil {
		t.Fatal("failed to get block height", err)
	}

	return persistence, blockHeight
}
