// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package test

import (
	"context"
	"github.com/orbs-network/crypto-lib-go/crypto/hash"
	"github.com/orbs-network/orbs-network-go/test/with"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestReturnAllAvailableTransactionsFromTransactionPool(t *testing.T) {
	with.Context(func(ctx context.Context) {
		with.Logging(t, func(harness *with.LoggingHarness) {
			h := newHarness(harness.Logger, false)
			txCount := uint32(2)
			h.expectTxPoolToReturnXTransactions(txCount)

			txBlock, err := h.requestTransactionsBlock(ctx)

			require.NoError(t, err, "request transactions block failed")
			require.Len(t, txBlock.SignedTransactions, int(txCount), "wrong number of txs")

			h.verifyTransactionsRequestedFromTransactionPool(t)
		})
	})
}

// TODO v1 Decouple this test from TestReturnAllAvailableTransactionsFromTransactionPool()
// Presently if the latter fails, this test will fail too
func TestCreateBlock_HappyFlow(t *testing.T) {
	with.Context(func(ctx context.Context) {
		with.Logging(t, func(harness *with.LoggingHarness) {
			h := newHarness(harness.Logger, false)
			txCount := 2

			h.expectTxPoolToReturnXTransactions(uint32(txCount))
			h.expectStateHashToReturn([]byte{1, 2, 3, 4, 5})

			txBlock, err := h.requestTransactionsBlock(ctx)
			require.Nil(t, err, "request transactions block failed")
			h.expectVirtualMachineToReturnXTransactionReceipts(len(txBlock.SignedTransactions))
			rxBlock, err := h.requestResultsBlock(ctx, txBlock)
			require.Nil(t, err, "request results block failed")
			require.Equal(t, txCount, len(rxBlock.TransactionReceipts))
			h.verifyTransactionsRequestedFromTransactionPool(t)
		})
	})
}

func TestReturnAllAvailableTransactionsFromTransactionPool_WithTriggers(t *testing.T) {
	with.Context(func(ctx context.Context) {
		with.Logging(t, func(harness *with.LoggingHarness) {
			h := newHarness(harness.Logger, true)
			txCount := uint32(2)
			txCountWithTrigger := txCount + 1
			h.expectTxPoolToReturnXTransactions(txCount)

			txBlock, err := h.requestTransactionsBlock(ctx)

			require.NoError(t, err, "request transactions block failed")
			require.Len(t, txBlock.SignedTransactions, int(txCountWithTrigger), "wrong number of txs")

			h.verifyTransactionsRequestedFromTransactionPool(t)
		})
	})
}

func TestCreateBlock_CreateTxsBlockFailsWithBadGenesis(t *testing.T) {
	with.Context(func(ctx context.Context) {
		with.Logging(t, func(harness *with.LoggingHarness) {
			h := newHarness(harness.Logger, false)
			h.management.Reset()
			setManagementValues(h.management, 1, primitives.TimestampSeconds(time.Now().Unix()), primitives.TimestampSeconds(time.Now().Unix()+5000))
			txCount := uint32(2)
			h.expectTxPoolToReturnXTransactions(txCount)

			_, err := h.requestTransactionsBlock(ctx)
			require.Error(t, err, "request transactions block failed")
		})
	})
}

func TestCreateBlock_TimeRefOfLeaderShouldBeMonotonous_MaxOfLeaderManagementTimeRefAndPrevBlocTimeRef(t *testing.T) {
	with.Context(func(ctx context.Context) {
		with.Logging(t, func(harness *with.LoggingHarness) {
			h := newHarness(harness.Logger, false)
			h.management.Reset()
			leaderRef := primitives.TimestampSeconds(5000) + primitives.TimestampSeconds(h.config.ManagementConsensusGraceTimeout()/time.Second)
			prevRef := primitives.TimestampSeconds(5005)

			setManagementValues(h.management, 1, leaderRef, primitives.TimestampSeconds(1000))
			txCount := uint32(2)
			h.expectTxPoolToReturnXTransactions(txCount)

			txBlock, err := h.service.RequestNewTransactionsBlock(ctx, &services.RequestNewTransactionsBlockInput{
				CurrentBlockHeight:     2,
				PrevBlockHash:          hash.CalcSha256([]byte{1}),
				PrevBlockTimestamp:     primitives.TimestampNano(time.Now().UnixNano() - 100),
				PrevBlockReferenceTime: prevRef,
			})

			require.NoError(t, err, "request transactions block failed")
			require.EqualValues(t, prevRef, txBlock.TransactionsBlock.Header.ReferenceTime())
		})
	})
}

func TestCreateBlock_CreateResultsBlockFailsWithBadGenesis(t *testing.T) {
	with.Context(func(ctx context.Context) {
		with.Logging(t, func(harness *with.LoggingHarness) {
			h := newHarness(harness.Logger, false)
			txCount := uint32(2)
			h.expectTxPoolToReturnXTransactions(txCount)

			txBlock, err := h.requestTransactionsBlock(ctx)
			require.NoError(t, err, "request transactions block failed")

			h.management.Reset()
			setManagementValues(h.management, 1, primitives.TimestampSeconds(time.Now().Unix()), primitives.TimestampSeconds(time.Now().Unix()+5000))
			_, err = h.requestResultsBlock(ctx, txBlock)

			require.Error(t, err, "request results block should fail")
			require.Contains(t, err.Error(), "failed genesis time reference")
		})
	})
}
