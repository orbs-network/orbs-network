// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package adapter

import (
	"github.com/orbs-network/orbs-spec/types/go/primitives"
)

// ContractState is a state.key->state.value
type ContractState map[string][]byte
type ChainState map[primitives.ContractName]ContractState

type StatePersistence interface {
	Write(height primitives.BlockHeight, ts primitives.TimestampNano, refTime primitives.TimestampSeconds, prevRefTime primitives.TimestampSeconds, proposer primitives.NodeAddress, root primitives.Sha256, diff ChainState) error
	Read(contract primitives.ContractName, key string) ([]byte, bool, error)
	ReadMetadata() (primitives.BlockHeight, primitives.TimestampNano, primitives.TimestampSeconds, primitives.TimestampSeconds, primitives.NodeAddress, primitives.Sha256, error)
	FullState() ChainState
}
