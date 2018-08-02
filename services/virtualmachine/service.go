package virtualmachine

import (
	"encoding/binary"
	"fmt"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/orbs-spec/types/go/services/handlers"
)

type service struct {
	blockStorage         services.BlockStorage
	stateStorage         services.StateStorage
	processors           map[protocol.ProcessorType]services.Processor
	crosschainConnectors map[protocol.CrosschainConnectorType]services.CrosschainConnector
}

func NewVirtualMachine(
	blockStorage services.BlockStorage,
	stateStorage services.StateStorage,
	processors map[protocol.ProcessorType]services.Processor,
	crosschainConnectors map[protocol.CrosschainConnectorType]services.CrosschainConnector,
) services.VirtualMachine {

	return &service{
		blockStorage:         blockStorage,
		processors:           processors,
		crosschainConnectors: crosschainConnectors,
		stateStorage:         stateStorage,
	}
}

func (s *service) ProcessTransactionSet(input *services.ProcessTransactionSetInput) (*services.ProcessTransactionSetOutput, error) {
	panic("Not implemented")
}

func (s *service) RunLocalMethod(input *services.RunLocalMethodInput) (*services.RunLocalMethodOutput, error) {

	// TODO XXX this implementation bakes an implementation of an arbitraty contract function.
	// The function scans a set of keys derived from the current block height and sums up all their values.

	// todo get list of keys to read from "hard codded contract func"
	blockHeight, _ := s.blockStorage.GetLastCommittedBlockHeight(&services.GetLastCommittedBlockHeightInput{})
	keys := make([]primitives.Ripmd160Sha256, 0, blockHeight.LastCommittedBlockHeight)
	for i := uint64(0); i < uint64(blockHeight.LastCommittedBlockHeight)+uint64(1); i++ {
		keys = append(keys, primitives.Ripmd160Sha256(fmt.Sprintf("balance%v", i)))
	}

	readKeys := &services.ReadKeysInput{ContractName: "BenchmarkToken", Keys: keys}
	results, _ := s.stateStorage.ReadKeys(readKeys)
	sum := uint64(0)
	for _, t := range results.StateRecords {
		if len(t.Value()) > 0 {
			sum += binary.LittleEndian.Uint64(t.Value())
		}
	}
	arg := (&protocol.MethodArgumentBuilder{
		Name:        "balance",
		Type:        protocol.METHOD_ARGUMENT_TYPE_UINT_64_VALUE,
		Uint64Value: sum,
	}).Build()
	return &services.RunLocalMethodOutput{
		OutputArguments: []*protocol.MethodArgument{arg},
	}, nil
}

func (s *service) TransactionSetPreOrder(input *services.TransactionSetPreOrderInput) (*services.TransactionSetPreOrderOutput, error) {
	panic("Not implemented")
}

func (s *service) HandleSdkCall(input *handlers.HandleSdkCallInput) (*handlers.HandleSdkCallOutput, error) {
	panic("Not implemented")
}
