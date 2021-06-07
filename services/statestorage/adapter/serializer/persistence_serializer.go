// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package serializer

import (
	"github.com/orbs-network/orbs-network-go/services/statestorage/adapter"
	"github.com/orbs-network/orbs-network-go/services/statestorage/adapter/memory"
)

type StatePersistenceSerializer interface {
	adapter.StatePersistence
	Dump() ([]byte, error)
}

type statePersistenceSerializer struct {
	*memory.InMemoryStatePersistence
}

func NewStatePersistenceSerializer(persistence *memory.InMemoryStatePersistence) StatePersistenceSerializer {
	result := &statePersistenceSerializer{
		InMemoryStatePersistence: persistence,
	}
	return result
}

func (s *statePersistenceSerializer) Dump() ([]byte, error) {
	blockHeight, timestamp, refTime, prevRefTime, proposer, merkleRoot, err := s.ReadMetadata()
	if err != nil {
		return nil, err
	}

	persistence := &SerializedMemoryPersistenceBuilder{
		BlockHeight:           blockHeight,
		Timestamp:             timestamp,
		ReferenceTime:         refTime,
		PreviousReferenceTime: prevRefTime,
		Proposer:              proposer,
		MerkleRootHash:        merkleRoot,
	}

	for contract, state := range s.FullState() {
		for key, value := range state {
			persistence.Entries = append(persistence.Entries, &SerializedContractKeyValueEntryBuilder{
				ContractName: contract,
				Key:          []byte(key),
				Value:        value,
			})
		}
	}

	//fmt.Println(fmt.Sprintf("%+v", persistence))
	//fmt.Println(persistence.Build().String())

	return persistence.Build().Raw(), nil
}
