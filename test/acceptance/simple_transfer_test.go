package acceptance

import (
	"context"
	"github.com/orbs-network/orbs-network-go/services/gossip/adapter/testkit"
	"github.com/orbs-network/orbs-network-go/services/processor/native/repository/BenchmarkToken"
	"github.com/orbs-network/orbs-spec/types/go/protocol/consensus"
	"github.com/orbs-network/orbs-spec/types/go/protocol/gossipmessages"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestCommitTransactionWithLeanHelix(t *testing.T) {
	newHarness(t).
		WithNumNodes(4).
		WithConsensusAlgos(consensus.CONSENSUS_ALGO_TYPE_LEAN_HELIX).
		Start(func(ctx context.Context, network NetworkHarness) {
			contract := network.DeployBenchmarkTokenContract(ctx, 5)
			// leader is nodeIndex 0, validator is nodeIndex 1
			_, txHash := contract.Transfer(ctx, 0, 17, 5, 6)

			network.WaitForTransactionInNodeState(ctx, txHash, 0)
			t.Log("finished waiting for tx")

			require.EqualValues(t, benchmarktoken.TOTAL_SUPPLY-17, contract.GetBalance(ctx, 0, 5), "getBalance result for the sender on gateway node")
			require.EqualValues(t, 17, contract.GetBalance(ctx, 0, 6), "getBalance result for the receiver on gateway node")
			t.Log("test done")
		})
}

func TestLeaderCommitsTransactionsAndSkipsInvalidOnes(t *testing.T) {
	newHarness(t).
		Start(func(parent context.Context, network NetworkHarness) {
			ctx, cancel := context.WithTimeout(parent, 2*time.Second)
			defer cancel()

			contract := network.DeployBenchmarkTokenContract(ctx, 5)

			// In benchmark consensus, leader is nodeIndex 0, validator is nodeIndex 1
			// In Lean Helix, leader and validators are random

			_, txHash1 := contract.Transfer(ctx, 0, 17, 5, 6)
			contract.InvalidTransfer(ctx, 0, 5, 6)
			_, txHash2 := contract.Transfer(ctx, 0, 22, 5, 6)

			t.Log("waiting for node 0")

			network.WaitForTransactionInNodeState(ctx, txHash1, 0)
			network.WaitForTransactionInNodeState(ctx, txHash2, 0)
			require.EqualValues(t, benchmarktoken.TOTAL_SUPPLY-39, contract.GetBalance(ctx, 0, 5), "getBalance result on leader")
			require.EqualValues(t, 39, contract.GetBalance(ctx, 0, 6), "getBalance result on leader")

			t.Log("waiting for node 1")

			network.WaitForTransactionInNodeState(ctx, txHash1, 1)
			network.WaitForTransactionInNodeState(ctx, txHash2, 1)
			require.EqualValues(t, benchmarktoken.TOTAL_SUPPLY-39, contract.GetBalance(ctx, 1, 5), "getBalance result on non leader")
			require.EqualValues(t, 39, contract.GetBalance(ctx, 1, 6), "getBalance result on non leader")
		})
}

func TestNonLeaderPropagatesTransactionsToLeader(t *testing.T) {
	newHarness(t).
		WithConsensusAlgos(consensus.CONSENSUS_ALGO_TYPE_BENCHMARK_CONSENSUS).
		Start(func(parent context.Context, network NetworkHarness) {
			ctx, cancel := context.WithTimeout(parent, 1*time.Second)
			defer cancel()

			contract := network.DeployBenchmarkTokenContract(ctx, 5)

			// leader is nodeIndex 0, validator is nodeIndex 1

			pausedTxForwards := network.TransportTamperer().Pause(testkit.TransactionRelayMessage(gossipmessages.TRANSACTION_RELAY_FORWARDED_TRANSACTIONS))
			txHash := contract.TransferInBackground(ctx, 1, 17, 5, 6)

			if err := network.BlockPersistence(0).GetBlockTracker().WaitForBlock(ctx, 2); err != nil {
				t.Errorf("failed waiting for block on node 0: %s", err)
			}
			if err := network.BlockPersistence(1).GetBlockTracker().WaitForBlock(ctx, 2); err != nil {
				t.Errorf("failed waiting for block on node 1: %s", err)
			}

			pausedTxForwards.StopTampering(ctx)
			network.WaitForTransactionInNodeState(ctx, txHash, 0)
			require.EqualValues(t, 17, contract.GetBalance(ctx, 0, 6), "eventual getBalance result on leader")
			network.WaitForTransactionInNodeState(ctx, txHash, 1)
			require.EqualValues(t, 17, contract.GetBalance(ctx, 1, 6), "eventual getBalance result on non leader")
		})
}

func TestLeaderCommitsTwoTransactionsInOneBlock(t *testing.T) {
	newHarness(t).Start(func(parent context.Context, network NetworkHarness) {
		ctx, cancel := context.WithTimeout(parent, 1*time.Second)
		defer cancel()

		contract := network.DeployBenchmarkTokenContract(ctx, 5)

		// leader is nodeIndex 0, validator is nodeIndex 1

		txHash1 := contract.TransferInBackground(ctx, 0, 17, 5, 6)
		txHash2 := contract.TransferInBackground(ctx, 0, 22, 5, 6)

		t.Log("waiting for node 0")

		network.WaitForTransactionInNodeState(ctx, txHash1, 0)
		network.WaitForTransactionInNodeState(ctx, txHash2, 0)
		require.EqualValues(t, benchmarktoken.TOTAL_SUPPLY-39, contract.GetBalance(ctx, 0, 5), "getBalance result on leader")
		require.EqualValues(t, 39, contract.GetBalance(ctx, 0, 6), "getBalance result on leader")

		t.Log("waiting for node 1")

		network.WaitForTransactionInNodeState(ctx, txHash1, 1)
		network.WaitForTransactionInNodeState(ctx, txHash2, 1)
		require.EqualValues(t, benchmarktoken.TOTAL_SUPPLY-39, contract.GetBalance(ctx, 1, 5), "getBalance result on non leader")
		require.EqualValues(t, 39, contract.GetBalance(ctx, 1, 6), "getBalance result on non leader")
	})
}
