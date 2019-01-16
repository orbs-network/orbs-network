package gamma

import (
	"context"
	"github.com/orbs-network/orbs-network-go/instrumentation/log"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/test"
	"github.com/orbs-network/orbs-network-go/test/contracts"
	"github.com/orbs-network/orbs-network-go/test/harness/callcontract"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestNonLeaderDeploysNativeContract(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping compilation of contracts in short mode")
	}

	test.WithContext(func(ctx context.Context) {
		network := NewDevelopmentNetwork(ctx, log.GetLogger(), metric.NewRegistry())
		contract := callcontract.NewContractClient(network)

		counterStart := contracts.MOCK_COUNTER_CONTRACT_START_FROM

		t.Log("deploying contract")

		contract.DeployCounterContract(ctx, 1) // leader is nodeIndex 0, validator is nodeIndex 1

		require.True(t, test.Eventually(3*time.Second, func() bool {
			return counterStart == contract.CounterGet(ctx, 0)

		}), "expected counter value to equal it's initial value")

		t.Log("transacting with contract")

		contract.CounterAdd(ctx, 1, 17)

		require.True(t, test.Eventually(3*time.Second, func() bool {
			return counterStart+17 == contract.CounterGet(ctx, 0)
		}), "expected counter value to be incremented by transaction")

	})
	time.Sleep(5 * time.Millisecond) // give context dependent goroutines 5 ms to terminate gracefully
}
