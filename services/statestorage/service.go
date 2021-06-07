// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package statestorage

import (
	"context"
	"fmt"
	"github.com/orbs-network/crypto-lib-go/crypto/merkle"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/instrumentation/logfields"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/instrumentation/trace"
	"github.com/orbs-network/orbs-network-go/services/statestorage/adapter"
	"github.com/orbs-network/orbs-network-go/synchronization"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/scribe/log"
	"github.com/pkg/errors"
	"sync"
)

var LogTag = log.Service("state-storage")

type metrics struct {
	readKeys       *metric.Rate
	writeKeys      *metric.Rate
	blockHeight    *metric.Gauge
	currentNumKeys *metric.Gauge
	currentSizeMB  *metric.Gauge
}

func newMetrics(m metric.Factory) *metrics {
	return &metrics{
		readKeys:       m.NewRate("StateStorage.ReadRequestedKeys"),
		writeKeys:      m.NewRate("StateStorage.WriteRequestedKeys"),
		blockHeight:    m.NewGauge("StateStorage.BlockHeight"),
		currentNumKeys: m.NewGaugeWithValue("StateStorage.CurrentNumKeys", 0),
		currentSizeMB:  m.NewGaugeWithValue("StateStorage.CurrentSizeMB", 0),
	}
}

type service struct {
	config         config.StateStorageConfig
	blockTracker   *synchronization.BlockTracker
	heightReporter adapter.BlockHeightReporter
	logger         log.Logger
	metrics        *metrics

	mutex     sync.RWMutex
	revisions *rollingRevisions
}

func NewStateStorage(config config.StateStorageConfig, persistence adapter.StatePersistence, heightReporter adapter.BlockHeightReporter, parent log.Logger, metricFactory metric.Factory) services.StateStorage {
	forest, merkleRoot := merkle.NewForest()
	logger := parent.WithTags(LogTag)
	if heightReporter == nil {
		heightReporter = synchronization.NopHeightReporter{}
	}

	blockHeight, _, _, _, _, _, err := persistence.ReadMetadata()
	if err != nil {
		panic("failed to read metadata from the state storage")
	}

	return &service{
		config:         config,
		blockTracker:   synchronization.NewBlockTracker(logger, uint64(blockHeight), uint16(config.BlockTrackerGraceDistance())),
		heightReporter: heightReporter,
		logger:         logger,
		metrics:        newMetrics(metricFactory),

		mutex:     sync.RWMutex{},
		revisions: newRollingRevisions(logger, persistence, int(config.StateStorageHistorySnapshotNum()), forest, merkleRoot),
	}
}

func (s *service) CommitStateDiff(ctx context.Context, input *services.CommitStateDiffInput) (*services.CommitStateDiffOutput, error) {
	logger := s.logger.WithTags(trace.LogFieldFrom(ctx))

	if input.ResultsBlockHeader == nil || input.ContractStateDiffs == nil {
		panic(fmt.Sprintf("CommitStateDiff received corrupt args, input=%+v", input))
	}

	commitBlockHeight := input.ResultsBlockHeader.BlockHeight()
	commitTimestamp := input.ResultsBlockHeader.Timestamp()
	commitRefTime := input.ResultsBlockHeader.ReferenceTime()
	commitPorposerAddress := input.ResultsBlockHeader.BlockProposerAddress()

	s.mutex.Lock()
	defer s.mutex.Unlock()

	logger.Info("trying to commit state diff", logfields.BlockHeight(commitBlockHeight), log.Int("number-of-state-diffs", len(input.ContractStateDiffs)))

	currentHeight := s.revisions.getCurrentHeight()
	if currentHeight+1 != commitBlockHeight {
		return &services.CommitStateDiffOutput{NextDesiredBlockHeight: currentHeight + 1}, nil
	}

	// TODO(v1) assert input.ResultsBlockHeader.PreExecutionStateRootHash() == s.revisions.getRevisionHash(commitBlockHeight - 1)

	err := s.revisions.addRevision(commitBlockHeight, commitTimestamp, commitRefTime, commitPorposerAddress, inflateChainState(input.ContractStateDiffs))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to write state for block height %d", commitBlockHeight)
	}

	s.metrics.writeKeys.Measure(int64(len(input.ContractStateDiffs)))

	s.blockTracker.IncrementTo(commitBlockHeight)
	s.heightReporter.IncrementTo(commitBlockHeight)
	s.metrics.blockHeight.Update(int64(commitBlockHeight))
	s.metrics.currentNumKeys.Update(int64(s.revisions.getCurrentNumKeys()))
	s.metrics.currentSizeMB.Update(int64(s.revisions.getCurrentSize()))

	return &services.CommitStateDiffOutput{NextDesiredBlockHeight: commitBlockHeight + 1}, nil
}

func (s *service) ReadKeys(ctx context.Context, input *services.ReadKeysInput) (*services.ReadKeysOutput, error) {
	if input.ContractName == "" {
		return nil, errors.Errorf("missing contract name")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, s.config.BlockTrackerGraceTimeout())
	defer cancel()

	if err := s.blockTracker.WaitForBlock(timeoutCtx, input.BlockHeight); err != nil {
		return nil, errors.Wrapf(err, "unsupported block height: block %d is not yet committed", input.BlockHeight)
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	currentHeight := s.revisions.getCurrentHeight()
	if input.BlockHeight+primitives.BlockHeight(s.config.StateStorageHistorySnapshotNum()) <= currentHeight {
		return nil, errors.Errorf("unsupported block height: block %v too old. currently at %v. keeping %v back", input.BlockHeight, currentHeight, primitives.BlockHeight(s.config.StateStorageHistorySnapshotNum()))
	}

	records := make([]*protocol.StateRecord, 0, len(input.Keys))
	for _, key := range input.Keys {
		record, ok, err := s.revisions.getRevisionRecord(input.BlockHeight, input.ContractName, string(key))
		if err != nil {
			return nil, errors.Wrap(err, "persistence layer error")
		}
		if ok {
			records = append(records, (&protocol.StateRecordBuilder{Key: key, Value: record}).Build())
		} else { // implicitly return the zero value if key is missing in db
			records = append(records, (&protocol.StateRecordBuilder{Key: key, Value: newZeroValue()}).Build())
		}
	}

	s.metrics.readKeys.Measure(int64(len(input.Keys)))

	output := &services.ReadKeysOutput{StateRecords: records}
	if len(output.StateRecords) == 0 {
		return output, errors.Errorf("no value found for input key(s)")
	}
	return output, nil
}

func (s *service) GetLastCommittedBlockInfo(ctx context.Context, input *services.GetLastCommittedBlockInfoInput) (*services.GetLastCommittedBlockInfoOutput, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := &services.GetLastCommittedBlockInfoOutput{
		BlockHeight:          s.revisions.getCurrentHeight(),
		BlockTimestamp:       s.revisions.getCurrentTimestamp(),
		CurrentReferenceTime: s.revisions.getCurrentReferenceTime(),
		PrevReferenceTime:    s.revisions.getPrevReferenceTime(),
		BlockProposerAddress: s.revisions.getCurrentProposerAddress(),
		CurrentNumKeys: 	  s.revisions.getCurrentNumKeys(),
		CurrentSize: 		  s.revisions.getCurrentSize(),
	}
	s.logger.Info("state storage block height requested", logfields.BlockHeight(result.BlockHeight))
	return result, nil
}

func (s *service) GetStateHash(ctx context.Context, input *services.GetStateHashInput) (*services.GetStateHashOutput, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, s.config.BlockTrackerGraceTimeout())
	defer cancel()
	if err := s.blockTracker.WaitForBlock(timeoutCtx, input.BlockHeight); err != nil {
		return nil, errors.Wrapf(err, "GetStateHash(): unsupported block height: block %d is not yet committed", input.BlockHeight)
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	currentHeight := s.revisions.getCurrentHeight()
	if input.BlockHeight+primitives.BlockHeight(s.config.StateStorageHistorySnapshotNum()) <= currentHeight {
		return nil, errors.Errorf("unsupported block height: block %v too old. currently at %v. keeping %v back", input.BlockHeight, currentHeight, primitives.BlockHeight(s.config.StateStorageHistorySnapshotNum()))
	}

	value, err := s.revisions.getRevisionHash(input.BlockHeight)
	if err != nil {
		return nil, errors.Wrapf(err, "could not find a merkle root for block height %d", input.BlockHeight)
	}
	output := &services.GetStateHashOutput{StateMerkleRootHash: value}

	return output, nil
}

func inflateChainState(csd []*protocol.ContractStateDiff) adapter.ChainState {
	result := make(adapter.ChainState)
	for _, stateDiffs := range csd {
		// copying here is very important to free up the underlying structures
		contract := primitives.ContractName(stateDiffs.ContractName().String())
		contractMap, ok := result[contract]
		if !ok {
			contractMap = make(map[string][]byte)
			result[contract] = contractMap
		}
		for i := stateDiffs.StateDiffsIterator(); i.HasNext(); {
			r := i.NextStateDiffs()

			// copying here is very important to free up the underlying structures
			detachedBuffer := make([]byte, len(r.Raw()))
			copy(detachedBuffer, r.Raw())

			diffToApply := protocol.StateRecordReader(detachedBuffer)
			contractMap[string(diffToApply.Key())] = append([]byte{}, diffToApply.Value()...)
		}
	}
	return result
}

func newZeroValue() []byte {
	return []byte{}
}
