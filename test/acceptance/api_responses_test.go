package acceptance

import (
	"context"
	"github.com/orbs-network/orbs-network-go/test/builders"
	"github.com/orbs-network/orbs-network-go/test/harness"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestResponseForTransactionOnValidContract(t *testing.T) {
	harness.Network(t).Start(func(parent context.Context, network harness.TestNetworkDriver) {
		ctx, cancel := context.WithTimeout(parent, 1*time.Second)
		defer cancel()

		t.Log("testing", network.Description())

		tx := builders.TransferTransaction()
		resp, _ := network.SendTransaction(ctx, tx.Builder(), 0)
		require.Equal(t, protocol.REQUEST_STATUS_COMPLETED, resp.RequestResult().RequestStatus())
		require.Equal(t, protocol.TRANSACTION_STATUS_COMMITTED, resp.TransactionStatus())
		require.Equal(t, protocol.EXECUTION_RESULT_SUCCESS, resp.TransactionReceipt().ExecutionResult())
	})
}

func TestResponseForTransactionOnContractNotDeployed(t *testing.T) {
	harness.Network(t).Start(func(parent context.Context, network harness.TestNetworkDriver) {
		ctx, cancel := context.WithTimeout(parent, 1*time.Second)
		defer cancel()

		t.Log("testing", network.Description())

		tx := builders.Transaction().WithContract("UnknownContract")
		resp, _ := network.SendTransaction(ctx, tx.Builder(), 0)
		require.Equal(t, protocol.REQUEST_STATUS_BAD_REQUEST, resp.RequestResult().RequestStatus())
		require.Equal(t, protocol.TRANSACTION_STATUS_COMMITTED, resp.TransactionStatus())
		require.Equal(t, protocol.EXECUTION_RESULT_ERROR_CONTRACT_NOT_DEPLOYED, resp.TransactionReceipt().ExecutionResult())
	})
}

func TestResponseForTransactionOnContractWithBadInput(t *testing.T) {
	harness.Network(t).Start(func(parent context.Context, network harness.TestNetworkDriver) {
		ctx, cancel := context.WithTimeout(parent, 1*time.Second)
		defer cancel()

		t.Log("testing", network.Description())

		tx := builders.TransferTransaction().WithArgs("bad", "types", "of", "args")
		resp, _ := network.SendTransaction(ctx, tx.Builder(), 0)
		require.Equal(t, protocol.REQUEST_STATUS_BAD_REQUEST, resp.RequestResult().RequestStatus())
		require.Equal(t, protocol.TRANSACTION_STATUS_COMMITTED, resp.TransactionStatus())
		require.Equal(t, protocol.EXECUTION_RESULT_ERROR_INPUT, resp.TransactionReceipt().ExecutionResult())
	})
}

func TestResponseForTransactionOnFailingContract(t *testing.T) {
	harness.Network(t).Start(func(parent context.Context, network harness.TestNetworkDriver) {
		ctx, cancel := context.WithTimeout(parent, 1*time.Second)
		defer cancel()

		t.Log("testing", network.Description())

		tx := builders.Transaction().WithMethod(primitives.ContractName("BenchmarkContract"), primitives.MethodName("throw")).WithArgs()
		resp, _ := network.SendTransaction(ctx, tx.Builder(), 0)
		require.Equal(t, protocol.REQUEST_STATUS_COMPLETED, resp.RequestResult().RequestStatus())
		require.Equal(t, protocol.TRANSACTION_STATUS_COMMITTED, resp.TransactionStatus())
		require.Equal(t, protocol.EXECUTION_RESULT_ERROR_SMART_CONTRACT, resp.TransactionReceipt().ExecutionResult())
	})
}

func TestResponseForTransactionWithInvalidProtocolVersion(t *testing.T) {
	harness.Network(t).Start(func(parent context.Context, network harness.TestNetworkDriver) {
		ctx, cancel := context.WithTimeout(parent, 1*time.Second)
		defer cancel()

		t.Log("testing", network.Description())

		tx := builders.Transaction().WithProtocolVersion(9999999)
		resp, _ := network.SendTransaction(ctx, tx.Builder(), 0)
		require.Equal(t, protocol.REQUEST_STATUS_BAD_REQUEST, resp.RequestResult().RequestStatus())
		require.Equal(t, protocol.TRANSACTION_STATUS_REJECTED_UNSUPPORTED_VERSION, resp.TransactionStatus())
		require.Equal(t, protocol.EXECUTION_RESULT_RESERVED, resp.TransactionReceipt().ExecutionResult())
	})
}
