package test

import (
	"fmt"
	"github.com/orbs-network/go-mock"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/stretchr/testify/require"
	"testing"
)

func (h *harness) verifyHandlerRegistrations(t *testing.T) {
	for key, processor := range h.processors {
		ok, err := processor.Verify()
		if !ok {
			t.Fatal("Did not register with processor", key.String(), ":", err)
		}
	}
}

// each f() given is a different transaction in the set
func (h *harness) expectNativeProcessorCalled(fs ...func(primitives.ExecutionContextId) (protocol.ExecutionResult, error)) {
	for i, _ := range fs {
		i := i // needed for avoiding incorrect closure capture
		h.processors[protocol.PROCESSOR_TYPE_NATIVE].When("ProcessCall", mock.Any).Call(func(input *services.ProcessCallInput) (*services.ProcessCallOutput, error) {
			callResult, err := fs[i](input.ContextId)
			return &services.ProcessCallOutput{
				OutputArguments: []*protocol.MethodArgument{},
				CallResult:      callResult,
			}, err
		}).Times(1)
	}
}

func (h *harness) verifyNativeProcessorCalled(t *testing.T) {
	ok, err := h.processors[protocol.PROCESSOR_TYPE_NATIVE].Verify()
	require.True(t, ok, "did not call processor: %v", err)
}

func (h *harness) expectStateStorageBlockHeightRequested(returnValue primitives.BlockHeight) {
	outputToReturn := &services.GetStateStorageBlockHeightOutput{
		LastCommittedBlockHeight:    returnValue,
		LastCommittedBlockTimestamp: 1234,
	}

	h.stateStorage.When("GetStateStorageBlockHeight", mock.Any).Return(outputToReturn, nil).Times(1)
}

func (h *harness) verifyStateStorageBlockHeightRequested(t *testing.T) {
	ok, err := h.stateStorage.Verify()
	require.True(t, ok, "did not read from state storage: %v", err)
}

func (h *harness) expectStateStorageRead(expectedHeight primitives.BlockHeight, expectedKey []byte, returnValue []byte) {
	stateReadMatcher := func(i interface{}) bool {
		input, ok := i.(*services.ReadKeysInput)
		return ok &&
			input.BlockHeight == expectedHeight &&
			len(input.Keys) == 1 &&
			input.Keys[0].Equal(expectedKey)
	}

	outputToReturn := &services.ReadKeysOutput{
		StateRecords: []*protocol.StateRecord{(&protocol.StateRecordBuilder{
			Key:   expectedKey,
			Value: returnValue,
		}).Build()},
	}

	h.stateStorage.When("ReadKeys", mock.AnyIf(fmt.Sprintf("ReadKeys height equals %s and key equals %x", expectedHeight, expectedKey), stateReadMatcher)).Return(outputToReturn, nil).Times(1)
}

func (h *harness) verifyStateStorageRead(t *testing.T) {
	ok, err := h.stateStorage.Verify()
	require.True(t, ok, "did not read from state storage: %v", err)
}

func (h *harness) expectStateStorageNotRead() {
	h.stateStorage.When("ReadKeys", mock.Any).Return(&services.ReadKeysOutput{}, nil).Times(0)
}
