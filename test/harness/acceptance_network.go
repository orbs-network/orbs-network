package harness

import (
	"context"
	"fmt"
	sdkContext "github.com/orbs-network/orbs-contract-sdk/go/context"
	"github.com/orbs-network/orbs-network-go/bootstrap/inmemory"
	"github.com/orbs-network/orbs-network-go/instrumentation/log"
	blockStorageAdapter "github.com/orbs-network/orbs-network-go/services/blockstorage/adapter"
	ethereumAdapter "github.com/orbs-network/orbs-network-go/services/crosschainconnector/ethereum/adapter"
	"github.com/orbs-network/orbs-network-go/synchronization"
	"github.com/orbs-network/orbs-network-go/test"
	"github.com/orbs-network/orbs-network-go/test/harness/callcontract"
	testGossipAdapter "github.com/orbs-network/orbs-network-go/test/harness/services/gossip/adapter"
	nativeProcessorAdapter "github.com/orbs-network/orbs-network-go/test/harness/services/processor/native/adapter"
	testStateStorageAdapter "github.com/orbs-network/orbs-network-go/test/harness/services/statestorage/adapter"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/services"
)

type TestNetworkDriver interface {
	callcontract.CallContractAPI
	PublicApi(nodeIndex int) services.PublicApi
	Size() int
	BenchmarkTokenContract() callcontract.BenchmarkTokenClient
	TransportTamperer() testGossipAdapter.Tamperer
	EthereumSimulator() *ethereumAdapter.EthereumSimulator
	BlockPersistence(nodeIndex int) blockStorageAdapter.TamperingInMemoryBlockPersistence
	DumpState()
	WaitForTransactionInNodeState(ctx context.Context, txHash primitives.Sha256, nodeIndex int)
	WaitForTransactionInState(ctx context.Context, txHash primitives.Sha256)
	GetTransactionPoolBlockHeightTracker(nodeIndex int) *synchronization.BlockTracker
	MockContract(fakeContractInfo *sdkContext.ContractInfo, code string)
}

type acceptanceNetworkHarness struct {
	inmemory.Network

	tamperingTransport         testGossipAdapter.Tamperer
	ethereumConnection         *ethereumAdapter.EthereumSimulator
	fakeCompiler               nativeProcessorAdapter.FakeCompiler
	tamperingBlockPersistences []blockStorageAdapter.TamperingInMemoryBlockPersistence
	dumpingStatePersistences   []testStateStorageAdapter.DumpingStatePersistence
}

func (n *acceptanceNetworkHarness) WaitForTransactionInNodeState(ctx context.Context, txHash primitives.Sha256, nodeIndex int) {
	blockHeight := n.tamperingBlockPersistences[nodeIndex].WaitForTransaction(ctx, txHash)
	err := n.Nodes[nodeIndex].StateBlockHeightTracker.WaitForBlock(ctx, blockHeight)
	if err != nil {
		test.DebugPrintGoroutineStacks() // since test timed out, help find deadlocked goroutines
		panic(fmt.Sprintf("statePersistence.WaitUntilCommittedBlockOfHeight failed: %s", err.Error()))
	}
}

func (n *acceptanceNetworkHarness) WaitForTransactionInState(ctx context.Context, txHash primitives.Sha256) {
	for i, node := range n.Nodes {
		if node.Started() {
			n.WaitForTransactionInNodeState(ctx, txHash, i)
		}
	}
}

func (n *acceptanceNetworkHarness) TransportTamperer() testGossipAdapter.Tamperer {
	return n.tamperingTransport
}

func (n *acceptanceNetworkHarness) EthereumSimulator() *ethereumAdapter.EthereumSimulator {
	return n.ethereumConnection
}

func (n *acceptanceNetworkHarness) BenchmarkTokenContract() callcontract.BenchmarkTokenClient {
	return callcontract.NewContractClient(n)
}

func (n *acceptanceNetworkHarness) MockContract(fakeContractInfo *sdkContext.ContractInfo, code string) {
	n.fakeCompiler.ProvideFakeContract(fakeContractInfo, code)
}

func (n *acceptanceNetworkHarness) GetTransactionPoolBlockHeightTracker(nodeIndex int) *synchronization.BlockTracker {
	return n.Nodes[nodeIndex].GetTransactionPoolBlockHeightTracker()
}

func (n *acceptanceNetworkHarness) BlockPersistence(nodeIndex int) blockStorageAdapter.TamperingInMemoryBlockPersistence {
	return n.tamperingBlockPersistences[nodeIndex]
}

func (n *acceptanceNetworkHarness) GetStatePersistence(i int) testStateStorageAdapter.DumpingStatePersistence {
	return n.dumpingStatePersistences[i]
}

func (n *acceptanceNetworkHarness) Size() int {
	return len(n.Nodes)
}

func (n *acceptanceNetworkHarness) DumpState() {
	for i := range n.Nodes {
		n.Logger.Info("state dump", log.Int("node", i), log.String("data", n.GetStatePersistence(i).Dump()))
	}
}
