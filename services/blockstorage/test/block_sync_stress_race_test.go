// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package test

import (
	"context"
	"github.com/orbs-network/go-mock"
	"github.com/orbs-network/orbs-network-go/services/blockstorage/internodesync"
	"github.com/orbs-network/orbs-network-go/test/with"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/protocol/gossipmessages"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/orbs-spec/types/go/services/gossiptopics"
	"github.com/orbs-network/orbs-spec/types/go/services/handlers"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
	"time"
)

func TestSyncPetitioner_Stress_CommitsDuringSync(t *testing.T) {
	with.Concurrency(t, func(ctx context.Context, parent *with.ConcurrencyHarness) {
		harness := newBlockStorageHarness(parent).
			withSyncNoCommitTimeout(10 * time.Millisecond).
			withSyncCollectResponsesTimeout(10 * time.Millisecond).
			withSyncCollectChunksTimeout(50 * time.Millisecond).
			withBlockSyncDescendingEnabled(false) // ascending order

		testSyncPetitionerStressCommitsDuringSync(ctx, t, harness)
	})
}

func TestSyncPetitioner_Stress_CommitsDuringSync_Descending(t *testing.T) {
	with.Concurrency(t, func(ctx context.Context, parent *with.ConcurrencyHarness) {
		harness := newBlockStorageHarness(parent).
			withSyncNoCommitTimeout(10 * time.Millisecond).
			withSyncCollectResponsesTimeout(10 * time.Millisecond).
			withSyncCollectChunksTimeout(50 * time.Millisecond).
			withBlockSyncDescendingEnabled(true) // descending order

		testSyncPetitionerStressCommitsDuringSync(ctx, t, harness)
	})
}

func testSyncPetitionerStressCommitsDuringSync(ctx context.Context, t *testing.T, harness *harness) {
	const NUM_BLOCKS = 50
	blockChain := generateInMemoryBlockChain(NUM_BLOCKS)
	done := make(chan struct{})

	harness.gossip.When("BroadcastBlockAvailabilityRequest", mock.Any, mock.Any).Call(func(ctx context.Context, input *gossiptopics.BlockAvailabilityRequestInput) (*gossiptopics.EmptyOutput, error) {
		respondToBroadcastAvailabilityRequest(ctx, harness, input, NUM_BLOCKS, 7)
		return nil, nil
	})

	harness.gossip.When("SendBlockSyncRequest", mock.Any, mock.Any).Call(func(ctx context.Context, input *gossiptopics.BlockSyncRequestInput) (*gossiptopics.EmptyOutput, error) {
		blocksOrder := input.Message.SignedChunkRange.BlocksOrder()
		fromBlock := input.Message.SignedChunkRange.FirstBlockHeight()
		toBlock := input.Message.SignedChunkRange.LastBlockHeight()

		if blocksOrder == gossipmessages.SYNC_BLOCKS_ORDER_DESCENDING {
			if toBlock == 1 && fromBlock > internodesync.UNKNOWN_BLOCK_HEIGHT {
				done <- struct{}{}
			}
		} else if toBlock >= NUM_BLOCKS {
			done <- struct{}{}
		}
		respondToBlockSyncRequestWithConcurrentCommit(t, ctx, harness, input, blockChain)
		return nil, nil
	})

	harness.consensus.When("HandleBlockConsensus", mock.Any, mock.Any).Call(func(ctx context.Context, input *handlers.HandleBlockConsensusInput) (*handlers.HandleBlockConsensusOutput, error) {
		if input.Mode == handlers.HANDLE_BLOCK_CONSENSUS_MODE_VERIFY_AND_UPDATE && input.PrevBlockPair != nil {
			currHeight := input.BlockPair.TransactionsBlock.Header.BlockHeight()
			prevHeight := input.PrevBlockPair.TransactionsBlock.Header.BlockHeight()
			if currHeight != prevHeight+1 {
				done <- struct{}{}
				require.Failf(t, "HandleBlockConsensus given invalid args", "called with height %d and prev height %d", currHeight, prevHeight)
			}
		}
		return nil, nil
	})
	harness.start(ctx)

	select {
	case <-done:
		// test passed
	case <-ctx.Done():
		t.Fatalf("timed out waiting for sync flow to complete")
	}
}

// this would attempt to commit the same blocks at the same time from the sync flow and directly (simulating blocks arriving from consensus)
func respondToBlockSyncRequestWithConcurrentCommit(t testing.TB, ctx context.Context, harness *harness, input *gossiptopics.BlockSyncRequestInput, blockChainSlice []*protocol.BlockPairContainer) {
	response := createBlockSyncResponse(input, blockChainSlice, harness.config.syncBatchSize)
	go func() {
		time.Sleep(time.Duration(rand.Intn(1000)) * time.Nanosecond)
		_, err := harness.blockStorage.HandleBlockSyncResponse(ctx, response)
		require.NoError(t, err, "failed handling block sync response")

	}()

	go func() {
		time.Sleep(time.Duration(rand.Intn(1000)) * time.Nanosecond)
		if len(response.Message.BlockPairs) > 0 {
			_, err := harness.blockStorage.CommitBlock(ctx, &services.CommitBlockInput{
				BlockPair: response.Message.BlockPairs[0],
			})
			require.NoError(t, err, "failed committing first block in parallel to sync")
		}
		if len(response.Message.BlockPairs) > 1 {
			_, err := harness.blockStorage.CommitBlock(ctx, &services.CommitBlockInput{
				BlockPair: response.Message.BlockPairs[1],
			})
			require.NoError(t, err, "failed committing second block in parallel to sync")
		}
	}()
}
