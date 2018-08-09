package virtualmachine

import (
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/services"
)

func (s *service) getRecentBlockHeight() (primitives.BlockHeight, primitives.TimestampNano, error) {
	output, err := s.stateStorage.GetStateStorageBlockHeight(&services.GetStateStorageBlockHeightInput{})
	if err != nil {
		return 0, 0, err
	}
	return output.LastCommittedBlockHeight, output.LastCommittedBlockTimestamp, nil
}

func (s *service) runLocalMethod(
	blockHeight primitives.BlockHeight,
	contractName primitives.ContractName,
	methodName primitives.MethodName,
	argsIterator *protocol.TransactionInputArgumentsIterator,
	signer *protocol.Signer,
) (protocol.ExecutionResult, []*protocol.MethodArgument, error) {

	// create execution context
	contextId, executionContext := s.contexts.allocateExecutionContext(blockHeight, protocol.ACCESS_SCOPE_READ_ONLY)
	defer s.contexts.destroyExecutionContext(contextId)
	executionContext.serviceStackPush(contractName)

	// TODO: might need to change protos to avoid this copy
	args := []*protocol.MethodArgument{}
	for i := argsIterator; i.HasNext(); {
		args = append(args, i.NextInputArguments())
	}
	output, err := s.processors[protocol.PROCESSOR_TYPE_NATIVE].ProcessCall(&services.ProcessCallInput{
		ContextId:         contextId,
		ContractName:      contractName,
		MethodName:        methodName,
		InputArguments:    args,
		AccessScope:       protocol.ACCESS_SCOPE_READ_ONLY,
		PermissionScope:   protocol.PERMISSION_SCOPE_SERVICE, // TODO: improve
		CallingService:    contractName,
		TransactionSigner: signer,
	})
	if err != nil {
		return protocol.EXECUTION_RESULT_ERROR_UNEXPECTED, nil, err
	}

	return output.CallResult, output.OutputArguments, nil
}
