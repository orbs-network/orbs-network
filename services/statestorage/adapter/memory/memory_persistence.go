// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package memory

import (
	"bytes"
	"fmt"
	"github.com/orbs-network/crypto-lib-go/crypto/merkle"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/services/statestorage/adapter"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"sort"
	"strings"
	"sync"
)

type metrics struct {
	numberOfKeys      *metric.Gauge
	numberOfContracts *metric.Gauge
}

func newMetrics(m metric.Factory) *metrics {
	return &metrics{
		numberOfKeys:      m.NewGauge("StateStoragePersistence.TotalNumberOfKeys.Count"),
		numberOfContracts: m.NewGauge("StateStoragePersistence.TotalNumberOfContracts.Count"),
	}
}

type InMemoryStatePersistence struct {
	metrics     *metrics
	mutex       sync.RWMutex
	fullState   adapter.ChainState
	height      primitives.BlockHeight
	ts          primitives.TimestampNano
	refTime     primitives.TimestampSeconds
	prevRefTime primitives.TimestampSeconds
	proposer    primitives.NodeAddress
	merkleRoot  primitives.Sha256
}

func NewStatePersistence(metricFactory metric.Factory) *InMemoryStatePersistence {
	_, merkleRoot := merkle.NewForest()
	// TODO(https://github.com/orbs-network/orbs-network-go/issues/582) - this is our hard coded Genesis block (height 0). Move this to a more dignified place or load from a file
	return &InMemoryStatePersistence{
		metrics:     newMetrics(metricFactory),
		mutex:       sync.RWMutex{},
		fullState:   adapter.ChainState{},
		height:      0,
		ts:          0,
		refTime:     0,
		prevRefTime: 0,
		proposer:    []byte{},
		merkleRoot:  merkleRoot,
	}
}

func (sp *InMemoryStatePersistence) reportSize() {
	nContracts := 0
	nKeys := 0
	for _, records := range sp.fullState {
		nContracts++
		nKeys = nKeys + len(records)
	}
	sp.metrics.numberOfKeys.Update(int64(nKeys))
	sp.metrics.numberOfContracts.Update(int64(nContracts))
}

func (sp *InMemoryStatePersistence) Write(height primitives.BlockHeight, ts primitives.TimestampNano, refTime primitives.TimestampSeconds, prevRefTime primitives.TimestampSeconds, proposer primitives.NodeAddress, root primitives.Sha256, diff adapter.ChainState) error {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	sp.height = height
	sp.refTime = refTime
	sp.prevRefTime = prevRefTime
	sp.proposer = proposer
	sp.ts = ts
	sp.merkleRoot = root

	for contract, records := range diff {
		for key, value := range records {
			sp._writeOneRecord(primitives.ContractName(contract), key, value)
		}
	}
	sp.reportSize()
	return nil
}

func (sp *InMemoryStatePersistence) _writeOneRecord(c primitives.ContractName, key string, value []byte) {
	if _, ok := sp.fullState[c]; !ok {
		sp.fullState[c] = map[string][]byte{}
	}

	if isZeroValue(value) {
		delete(sp.fullState[c], key)
		return
	}

	sp.fullState[c][key] = value
}

func (sp *InMemoryStatePersistence) Read(contract primitives.ContractName, key string) ([]byte, bool, error) {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()

	record, ok := sp.fullState[contract][key]
	return record, ok, nil
}

func (sp *InMemoryStatePersistence) ReadMetadata() (primitives.BlockHeight, primitives.TimestampNano, primitives.TimestampSeconds, primitives.TimestampSeconds, primitives.NodeAddress, primitives.Sha256, error) {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()

	return sp.height, sp.ts, sp.refTime, sp.prevRefTime, sp.proposer, sp.merkleRoot, nil
}

func (sp *InMemoryStatePersistence) Dump() string {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()

	output := strings.Builder{}
	output.WriteString("{")
	output.WriteString(fmt.Sprintf("height: %v, data: {", sp.height))
	contracts := make([]primitives.ContractName, 0, len(sp.fullState))
	for c := range sp.fullState {
		contracts = append(contracts, c)
	}
	sort.Slice(contracts, func(i, j int) bool { return contracts[i] < contracts[j] })
	for _, currentContract := range contracts {
		keys := make([]string, 0, len(sp.fullState[currentContract]))
		for k := range sp.fullState[currentContract] {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

		output.WriteString(string(currentContract) + ":{")
		for _, k := range keys {
			output.WriteString(k)
			output.WriteString(":")
			output.WriteString(string(sp.fullState[currentContract][k]))
			output.WriteString(",")
		}
		output.WriteString("},")
	}
	output.WriteString("}}")
	return output.String()
}

func (sp *InMemoryStatePersistence) FullState() adapter.ChainState {
	return sp.fullState
}

func isZeroValue(value []byte) bool {
	return bytes.Equal(value, []byte{})
}
