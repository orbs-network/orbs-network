package test

import (
	"context"
	"github.com/orbs-network/orbs-network-go/test"
	"testing"
)

func newLeaderHarnessAndInit(t *testing.T, ctx context.Context) *harness {
	h := newHarness(true)
	h.expectNewBlockProposalNotRequested()
	h.expectCommitSent(0, h.config.NodePublicKey())
	h.createService(ctx)
	h.verifyCommitSent(t)
	return h
}

func TestLeaderInit(t *testing.T) {
	test.WithContext(func(ctx context.Context) {
		h := newLeaderHarnessAndInit(t, ctx)
		h.verifyNewBlockProposalNotRequested(t)
		h.verifyHandlerRegistrations(t)
	})
}

func TestLeaderCommitsNextBlockAfterEnoughConfirmations(t *testing.T) {
	test.WithContext(func(ctx context.Context) {
		h := newLeaderHarnessAndInit(t, ctx)

		h.expectNewBlockProposalRequestedAndSaved(1)
		h.expectCommitSent(1, h.config.NodePublicKey())
		h.receivedCommittedViaGossipFromSeveral(3, 0, true, true)
		h.verifyNewBlockProposalRequestedAndSaved(t)
		h.verifyCommitSent(t)

		h.expectNewBlockProposalRequestedAndSaved(2)
		h.expectCommitSent(2, h.config.NodePublicKey())
		h.receivedCommittedViaGossipFromSeveral(3, 1, true, true)
		h.verifyNewBlockProposalRequestedAndSaved(t)
		h.verifyCommitSent(t)
	})
}

func TestLeaderRetriesCommitOnError(t *testing.T) {
	test.WithContext(func(ctx context.Context) {
		h := newLeaderHarnessAndInit(t, ctx)

		h.expectNewBlockProposalRequestedToFail()
		h.expectCommitNotSent()
		h.receivedCommittedViaGossipFromSeveral(3, 0, true, true)
		h.verifyNewBlockProposalRequestedAndNotSaved(t)
		h.verifyCommitNotSent(t)
	})
}

func TestLeaderRetriesCommitAfterNotEnoughConfirmations(t *testing.T) {
	test.WithContext(func(ctx context.Context) {
		h := newLeaderHarnessAndInit(t, ctx)

		h.expectNewBlockProposalRequestedAndSaved(1)
		h.expectCommitSent(1, h.config.NodePublicKey())
		h.receivedCommittedViaGossipFromSeveral(3, 0, true, true)
		h.verifyNewBlockProposalRequestedAndSaved(t)
		h.verifyCommitSent(t)

		h.expectNewBlockProposalNotRequested()
		h.expectCommitSent(1, h.config.NodePublicKey())
		h.receivedCommittedViaGossipFromSeveral(2, 1, true, true)
		h.verifyNewBlockProposalNotRequested(t)
		h.verifyCommitSent(t)
	})
}

func TestLeaderIgnoresBadSignatures(t *testing.T) {
	test.WithContext(func(ctx context.Context) {
		h := newLeaderHarnessAndInit(t, ctx)

		h.expectNewBlockProposalNotRequested()
		h.expectCommitSent(0, h.config.NodePublicKey())
		h.receivedCommittedViaGossipFromSeveral(3, 0, false, true)
		h.verifyNewBlockProposalNotRequested(t)
		h.verifyCommitSent(t)
	})
}

func TestLeaderIgnoresNonFederationSigners(t *testing.T) {
	test.WithContext(func(ctx context.Context) {
		h := newLeaderHarnessAndInit(t, ctx)

		h.expectNewBlockProposalNotRequested()
		h.expectCommitSent(0, h.config.NodePublicKey())
		h.receivedCommittedViaGossipFromSeveral(3, 0, true, false)
		h.verifyNewBlockProposalNotRequested(t)
		h.verifyCommitSent(t)
	})
}

func TestLeaderIgnoresOldConfirmations(t *testing.T) {
	test.WithContext(func(ctx context.Context) {
		h := newLeaderHarnessAndInit(t, ctx)

		h.expectNewBlockProposalRequestedAndSaved(1)
		h.expectCommitSent(1, h.config.NodePublicKey())
		h.receivedCommittedViaGossipFromSeveral(3, 0, true, true)
		h.verifyNewBlockProposalRequestedAndSaved(t)
		h.verifyCommitSent(t)

		h.expectNewBlockProposalNotRequested()
		h.expectCommitSent(1, h.config.NodePublicKey())
		h.receivedCommittedViaGossipFromSeveral(3, 0, true, true)
		h.verifyNewBlockProposalNotRequested(t)
		h.verifyCommitSent(t)
	})
}

func TestLeaderIgnoresFutureConfirmations(t *testing.T) {
	test.WithContext(func(ctx context.Context) {
		h := newLeaderHarnessAndInit(t, ctx)

		h.expectNewBlockProposalNotRequested()
		h.expectCommitSent(0, h.config.NodePublicKey())
		h.receivedCommittedViaGossipFromSeveral(3, 1000, true, true)
		h.verifyNewBlockProposalNotRequested(t)
		h.verifyCommitSent(t)
	})
}
