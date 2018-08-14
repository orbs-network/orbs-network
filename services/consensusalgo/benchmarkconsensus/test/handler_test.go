package test

import (
	"context"
	"github.com/orbs-network/orbs-network-go/test"
	"github.com/orbs-network/orbs-network-go/test/builders"
	"testing"
)

func TestHandlerOfLeaderSynchronizesToFutureValidBlock(t *testing.T) {
	t.Parallel()
	test.WithContext(func(ctx context.Context) {
		h := newLeaderHarnessWaitingForCommittedMessages(t, ctx)
		aBlockFromLeader := builders.BlockPair().WithBenchmarkConsensusBlockProof(leaderKeyPair())

		t.Log("Handle block consensus (ie due to block sync) of height 1002")

		b1001 := aBlockFromLeader.WithHeight(1001).Build()
		b1002 := aBlockFromLeader.WithHeight(1002).WithPrevBlockHash(b1001).Build()
		h.expectNewBlockProposalNotRequested()
		h.expectCommitBroadcastViaGossip(1002, h.config.NodePublicKey())

		err := h.handleBlockConsensus(b1002, b1001)
		if err != nil {
			t.Fatal("handle did not validate valid block:", err)
		}
		h.verifyNewBlockProposalNotRequested(t)
		h.verifyCommitBroadcastViaGossip(t)
	})
}

func TestHandlerOfNonLeaderSynchronizesToFutureValidBlock(t *testing.T) {
	t.Parallel()
	test.WithContext(func(ctx context.Context) {
		h := newNonLeaderHarness(t, ctx)
		aBlockFromLeader := builders.BlockPair().WithBenchmarkConsensusBlockProof(leaderKeyPair())

		t.Log("Handle block consensus (ie due to block sync) of height 1002")

		b1001 := aBlockFromLeader.WithHeight(1001).Build()
		b1002 := aBlockFromLeader.WithHeight(1002).WithPrevBlockHash(b1001).Build()

		err := h.handleBlockConsensus(b1002, b1001)
		if err != nil {
			t.Fatal("handle did not validate valid block:", err)
		}

		t.Log("Leader commits height 1003, confirm height 1003")

		b1003 := aBlockFromLeader.WithHeight(1003).WithPrevBlockHash(b1002).Build()
		h.expectCommitSaveAndReply(b1003, 1003, h.config.ConstantConsensusLeader(), h.config.NodePublicKey())

		h.receivedCommitViaGossip(b1003)
		h.verifyCommitSaveAndReply(t)
	})
}

func TestHandlerForBlockConsensusWithBadPrevBlockHashPointer(t *testing.T) {
	t.Parallel()
	test.WithContext(func(ctx context.Context) {
		h := newNonLeaderHarness(t, ctx)
		aBlockFromLeader := builders.BlockPair().WithBenchmarkConsensusBlockProof(leaderKeyPair())

		t.Log("Handle block consensus (ie due to block sync) of height 2 without hash pointer")

		b1 := aBlockFromLeader.WithHeight(1).Build()
		b2 := aBlockFromLeader.WithHeight(2).WithPrevBlockHash(nil).Build()

		err := h.handleBlockConsensus(b2, b1)
		if err == nil {
			t.Fatal("handle did not discover blocks with bad hash pointers:", err)
		}
	})
}

func TestHandlerForBlockConsensusWithBadSignature(t *testing.T) {
	t.Parallel()
	test.WithContext(func(ctx context.Context) {
		h := newNonLeaderHarness(t, ctx)
		aBlockFromLeader := builders.BlockPair().WithBenchmarkConsensusBlockProof(leaderKeyPair())

		t.Log("Handle block consensus (ie due to block sync) of height 2 with bad signature")

		b1 := aBlockFromLeader.WithHeight(1).Build()
		b2 := builders.BlockPair().
			WithHeight(2).
			WithPrevBlockHash(b1).
			WithInvalidBenchmarkConsensusBlockProof(leaderKeyPair()).
			Build()

		err := h.handleBlockConsensus(b2, b1)
		if err == nil {
			t.Fatal("handle did not discover blocks with bad signature:", err)
		}
	})
}

func TestHandlerForBlockConsensusFromNonLeader(t *testing.T) {
	t.Parallel()
	test.WithContext(func(ctx context.Context) {
		h := newNonLeaderHarness(t, ctx)
		aBlockFromNonLeader := builders.BlockPair().WithBenchmarkConsensusBlockProof(otherNonLeaderKeyPair())

		t.Log("Handle block consensus (ie due to block sync) of height 2 from non leader")

		b1 := aBlockFromNonLeader.WithHeight(1).Build()
		b2 := aBlockFromNonLeader.WithHeight(2).WithPrevBlockHash(b1).Build()

		err := h.handleBlockConsensus(b2, b1)
		if err == nil {
			t.Fatal("handle did not discover blocks not from the leader:", err)
		}
	})
}
