package test

import (
	"github.com/orbs-network/orbs-network-go/crypto/digest"
	"github.com/orbs-network/orbs-network-go/crypto/signature"
	"github.com/orbs-network/orbs-network-go/services/transactionpool"
	"github.com/orbs-network/orbs-network-go/test/builders"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestForwardsANewValidTransactionUsingGossip(t *testing.T) {
	t.Parallel()
	h := newHarness()

	tx := builders.TransferTransaction().Build()
	sig, _ := signature.SignEd25519(thisNodeKeyPair.PrivateKey(), tx.Raw())
	h.expectTransactionToBeForwarded(tx, sig)

	_, err := h.addNewTransaction(tx)
	require.NoError(t, err, "a valid transaction was not added to pool")
	require.NoError(t, h.verifyMocks(), "mocks were not called as expected")
}

func TestDoesNotForwardInvalidTransactionsUsingGossip(t *testing.T) {
	t.Parallel()
	h := newHarness()

	tx := builders.TransferTransaction().WithInvalidContent().Build()
	h.expectNoTransactionsToBeForwarded()

	_, err := h.addNewTransaction(tx)

	require.Error(t, err, "an invalid transaction was added to the pool")
	require.NoError(t, h.verifyMocks(), "mocks were not called as expected")
}

func TestDoesNotAddTransactionsThatFailedPreOrderChecks(t *testing.T) {
	t.Parallel()
	h := newHarness()
	tx := builders.TransferTransaction().Build()
	h.failPreOrderCheckFor(func(t *protocol.SignedTransaction) bool {
		return t == tx
	})

	h.ignoringForwardMessages()

	blockHeight := primitives.BlockHeight(3)
	blockTime := primitives.TimestampNano(time.Now().UnixNano())
	h.goToBlock(blockHeight, blockTime)

	out, err := h.addNewTransaction(tx)

	require.NotNil(t, out, "output must not be nil even on errors")
	require.Equal(t, blockHeight, out.BlockHeight)
	require.Equal(t, blockTime, out.BlockTimestamp)

	require.Error(t, err, "an transaction that failed pre-order checks was added to the pool")
	require.IsType(t, &transactionpool.ErrTransactionRejected{}, err, "error was not of the expected type")

	typedError := err.(*transactionpool.ErrTransactionRejected)
	require.Equal(t, protocol.TRANSACTION_STATUS_REJECTED_SMART_CONTRACT_PRE_ORDER, typedError.TransactionStatus, "error did not contain expected transaction status")

	require.NoError(t, h.verifyMocks(), "mocks were not called as expected")

}

func TestDoesNotAddTheSameTransactionTwice(t *testing.T) {
	t.Parallel()
	h := newHarness()

	tx := builders.TransferTransaction().Build()
	h.ignoringForwardMessages()

	h.addNewTransaction(tx)

	receipt, err := h.addNewTransaction(tx)

	require.Error(t, err, "a transaction was added twice to the pool")
	require.NotNil(t, receipt, "receipt should never be nil")
	require.Equal(t, protocol.TRANSACTION_STATUS_REJECTED_DUPLCIATE_PENDING_TRANSACTION, receipt.TransactionStatus, "expected transaction status: duplicate pending")
	require.NoError(t, h.verifyMocks(), "mocks were not called as expected")
}

func TestReturnsReceiptForTransactionThatHasAlreadyBeenCommitted(t *testing.T) {
	t.Parallel()
	h := newHarness()

	tx := builders.TransferTransaction().Build()
	h.ignoringForwardMessages()
	h.ignoringTransactionResults()

	h.addNewTransaction(tx)
	h.assumeBlockStorageAtHeight(1)
	_, err := h.reportTransactionsAsCommitted(tx)
	require.NoError(t, err, "committing a transaction returned an unexpected error")

	receipt, err := h.addNewTransaction(tx)

	require.NoError(t, err, "a committed transaction that was added again was wrongly rejected")
	require.Equal(t, protocol.TRANSACTION_STATUS_DUPLCIATE_TRANSACTION_ALREADY_COMMITTED, receipt.TransactionStatus, "expected transaction status to be committed")
	require.Equal(t, digest.CalcTxHash(tx.Transaction()), receipt.TransactionReceipt.Txhash(), "expected transaction receipt to contain transaction hash")

	require.NoError(t, h.verifyMocks(), "mocks were not called as expected")
}

func TestDoesNotAddTransactionIfPoolIsFull(t *testing.T) {
	t.Parallel()
	h := newHarnessWithSizeLimit(1)

	h.expectNoTransactionsToBeForwarded()

	tx := builders.TransferTransaction().Build()
	receipt, err := h.addNewTransaction(tx)

	require.Error(t, err, "a transaction was added to a full pool")
	require.NotNil(t, receipt, "receipt should never be nil")
	require.Equal(t, protocol.TRANSACTION_STATUS_REJECTED_CONGESTION, receipt.TransactionStatus, "expected transaction status: congestion")
	require.NoError(t, h.verifyMocks(), "mocks were not called as expected")
}
