package test

import (
	"context"
	"github.com/orbs-network/orbs-network-go/test"
	"github.com/orbs-network/orbs-network-go/test/builders"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/protocol/client"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestGetTransactionReceiptProof_GetCommitStatusFromTxPool(t *testing.T) {
	test.WithContext(func(ctx context.Context) {
		harness := newPublicApiHarness(ctx, time.Second, time.Minute)

		harness.transactionHasProof()
		result, err := harness.papi.GetTransactionReceiptProof(ctx, &services.GetTransactionReceiptProofInput{
			ClientRequest: (&client.GetTransactionReceiptProofRequestBuilder{
				TransactionRef: builders.TransactionRef().Builder(),
			}).Build(),
		})

		harness.verifyMocks(t) // contract test

		// value test
		require.NoError(t, err, "error happened when it should not")
		require.NotNil(t, result, "get transaction receipt returned nil instead of object")
		require.Equal(t, protocol.TRANSACTION_STATUS_COMMITTED, result.ClientResponse.TransactionStatus(), "got wrong status")
		require.NotEmpty(t, result.ClientResponse.TransactionReceipt().Txhash(), "got empty receipt")
		require.NotEmpty(t, result.ClientResponse.PackedProof(), "got empty receipt proof")
	})
}

func TestGetTransactionReceiptProof_GetPendingStatusFromTxPool(t *testing.T) {
	test.WithContext(func(ctx context.Context) {
		harness := newPublicApiHarness(ctx, time.Second, time.Minute)

		harness.transactionPendingNoProofCalled()
		result, err := harness.papi.GetTransactionReceiptProof(ctx, &services.GetTransactionReceiptProofInput{
			ClientRequest: (&client.GetTransactionReceiptProofRequestBuilder{
				TransactionRef: builders.TransactionRef().Builder(),
			}).Build(),
		})

		harness.verifyMocks(t) // contract test

		// value test
		require.NoError(t, err, "error happened when it should not")
		require.NotNil(t, result, "get transaction receipt returned nil instead of object")
		require.Equal(t, protocol.TRANSACTION_STATUS_PENDING, result.ClientResponse.TransactionStatus(), "got wrong status")
		require.Empty(t, result.ClientResponse.TransactionReceipt().Txhash(), "receipt is not empty")
		require.Empty(t, result.ClientResponse.PackedProof(), "receipt proof is not empty")
	})
}

func TestGetTransactionReceiptProof_NoRecordsFound(t *testing.T) {
	test.WithContext(func(ctx context.Context) {
		harness := newPublicApiHarness(ctx, time.Second, time.Minute)

		harness.getTransactionStatusFailed()
		result, err := harness.papi.GetTransactionReceiptProof(ctx, &services.GetTransactionReceiptProofInput{
			ClientRequest: (&client.GetTransactionReceiptProofRequestBuilder{
				TransactionRef: builders.TransactionRef().Builder(),
			}).Build(),
		})

		harness.verifyMocks(t) // contract test

		// value test
		require.Error(t, err, "error did not happen when it should")
		require.NotNil(t, result, "get transaction receipt returned nil instead of object")
		require.Equal(t, protocol.TRANSACTION_STATUS_NO_RECORD_FOUND, result.ClientResponse.TransactionStatus(), "got wrong status")
		require.Empty(t, result.ClientResponse.TransactionReceipt().Txhash(), "receipt is not empty")
		require.Empty(t, result.ClientResponse.PackedProof(), "receipt proof is not empty")
	})
}
