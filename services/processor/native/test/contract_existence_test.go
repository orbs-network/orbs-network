package test

import (
	"github.com/orbs-network/orbs-network-go/services/processor/native/repository/_Deployments"
	"github.com/orbs-network/orbs-network-go/test/builders"
	"github.com/orbs-network/orbs-network-go/test/contracts"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestProcessCallWithUnknownContractFails(t *testing.T) {
	h := newHarness()
	input := processCallInput().WithUnknownContract().Build()
	h.expectSdkCallMadeWithServiceCallMethod(deployments.CONTRACT.Name, deployments.METHOD_GET_CODE.Name, builders.MethodArgumentsArray(string(input.ContractName)), nil, errors.New("code not found error"))

	_, err := h.service.ProcessCall(input)
	require.Error(t, err, "call should fail")

	h.verifySdkCallMade(t)
}

func TestGetContractInfoWithUnknownContractFails(t *testing.T) {
	h := newHarness()
	input := getContractInfoInput().WithUnknownContract().Build()
	h.expectSdkCallMadeWithServiceCallMethod(deployments.CONTRACT.Name, deployments.METHOD_GET_CODE.Name, builders.MethodArgumentsArray(string(input.ContractName)), nil, errors.New("code not found error"))

	_, err := h.service.GetContractInfo(input)
	require.Error(t, err, "GetContractInfo should fail")

	h.verifySdkCallMade(t)
}

func TestProcessCallWithDeployableContractThatCompiles(t *testing.T) {
	h := newHarness()
	input := processCallInput().WithDeployableCounterContract(contracts.MOCK_COUNTER_CONTRACT_START_FROM).Build()
	codeOutput := builders.MethodArgumentsArray([]byte(contracts.SourceCodeForCounter(contracts.MOCK_COUNTER_CONTRACT_START_FROM)))
	h.expectSdkCallMadeWithServiceCallMethod(deployments.CONTRACT.Name, deployments.METHOD_GET_CODE.Name, builders.MethodArgumentsArray(string(input.ContractName)), codeOutput, nil)

	output, err := h.service.ProcessCall(input)
	require.NoError(t, err, "call should succeed")
	require.Equal(t, contracts.MOCK_COUNTER_CONTRACT_START_FROM, output.OutputArgumentArray.ArgumentsIterator().NextArguments().Uint64Value(), "call return value should be counter value")

	t.Log("First call (not compiled) should getCode for compilation")
	h.verifySdkCallMade(t)

	output, err = h.service.ProcessCall(input)
	require.NoError(t, err, "call should succeed")
	require.Equal(t, contracts.MOCK_COUNTER_CONTRACT_START_FROM, output.OutputArgumentArray.ArgumentsIterator().NextArguments().Uint64Value(), "call return value should be counter value")

	t.Log("Make sure second call (already compiled) does not getCode again")
	h.verifySdkCallMade(t)
}
