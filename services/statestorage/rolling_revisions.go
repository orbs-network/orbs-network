// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package statestorage

import (
	"bytes"
	"fmt"
	"github.com/orbs-network/crypto-lib-go/crypto/hash"
	"github.com/orbs-network/crypto-lib-go/crypto/merkle"
	"github.com/orbs-network/orbs-network-go/instrumentation/logfields"
	"github.com/orbs-network/orbs-network-go/services/statestorage/adapter"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/scribe/log"
	"github.com/pkg/errors"
)

type merkleRevisions interface {
	Update(rootMerkle primitives.Sha256, diffs merkle.TrieDiffs) (primitives.Sha256, error)
	Forget(rootHash primitives.Sha256)
}

type revisionDiff struct {
	diff       adapter.ChainState
	merkleRoot primitives.Sha256
	height     primitives.BlockHeight
	ts         primitives.TimestampNano
	ref        primitives.TimestampSeconds
	prevRef    primitives.TimestampSeconds
	proposer   primitives.NodeAddress
}

type rollingRevisions struct {
	logger               log.Logger
	persist              adapter.StatePersistence
	transientRevisions   int
	revisions            []*revisionDiff
	merkle               merkleRevisions
	currentHeight        primitives.BlockHeight
	currentTs            primitives.TimestampNano
	currentMerkleRoot    primitives.Sha256
	currentProposer      primitives.NodeAddress
	currentRefTime       primitives.TimestampSeconds
	currentNumKeys		 primitives.StorageKeys
	currentSize			 uint64
	prevRefTime          primitives.TimestampSeconds
	persistedHeight      primitives.BlockHeight
	persistedRoot        primitives.Sha256
	persistedTs          primitives.TimestampNano
	persistedProposer    primitives.NodeAddress
	persistedRefTime     primitives.TimestampSeconds
	persistedPrevRefTime primitives.TimestampSeconds
}

func newRollingRevisions(logger log.Logger, persist adapter.StatePersistence, transientRevisions int, merkle merkleRevisions, root primitives.Sha256) *rollingRevisions {
	h, ts, ref, prevRef, pa, r, err := persist.ReadMetadata()
	if err != nil {
		panic(fmt.Sprintf("could not load state metadata, err=%s", err.Error()))
	}

	newRoot, err := merkle.Update(root, toMerkleInput(persist.FullState()))
	if err != nil {
		panic(fmt.Sprintf("could not calculate merkle from chain state"))
	}
	if !bytes.Equal(newRoot, r) {
		panic(fmt.Sprintf("merle root of state storage is corrupted: %s vs %s", newRoot, r))
	}

	result := &rollingRevisions{
		logger:               logger,
		persist:              persist,
		transientRevisions:   transientRevisions,
		merkle:               merkle,
		currentHeight:        h,
		currentTs:            ts,
		currentProposer:      pa,
		currentMerkleRoot:    r,
		currentRefTime:       ref,
		prevRefTime:          prevRef,
		persistedHeight:      h,
		persistedTs:          ts,
		persistedProposer:    pa,
		persistedRoot:        r,
		persistedRefTime:     ref,
		persistedPrevRefTime: prevRef,
	}

	return result
}

func (ls *rollingRevisions) getCurrentHeight() primitives.BlockHeight {
	return ls.currentHeight
}

func (ls *rollingRevisions) getCurrentTimestamp() primitives.TimestampNano {
	return ls.currentTs
}

func (ls *rollingRevisions) getCurrentReferenceTime() primitives.TimestampSeconds {
	return ls.currentRefTime
}

func (ls *rollingRevisions) getPrevReferenceTime() primitives.TimestampSeconds {
	return ls.prevRefTime
}

func (ls *rollingRevisions) getCurrentProposerAddress() primitives.NodeAddress {
	return ls.currentProposer
}

func (ls *rollingRevisions) getCurrentNumKeys() primitives.StorageKeys {
	return ls.currentNumKeys
}

func (ls *rollingRevisions) getCurrentSize() primitives.StorageSizeMegabyte {
	return primitives.StorageSizeMegabyte(ls.currentSize / 1048576)
}

func (ls *rollingRevisions) addRevision(height primitives.BlockHeight, ts primitives.TimestampNano, refTime primitives.TimestampSeconds, proposer primitives.NodeAddress, diff adapter.ChainState) error {
	newRoot, err := ls.merkle.Update(ls.currentMerkleRoot, toMerkleInput(diff))
	if err != nil {
		return errors.Wrapf(err, "failed to updated merkle tree")
	}

	newNumKeys, newSize, err2 := ls.calcNewSizes(diff)
	if err2 != nil {
		return errors.Wrapf(err, "failed to read current storage sizes")
	}

	ls.revisions = append(ls.revisions, &revisionDiff{
		diff:       diff,
		merkleRoot: newRoot,
		height:     height,
		ts:         ts,
		ref:        refTime,
		prevRef:    ls.currentRefTime, // one back
		proposer:   proposer,
	})
	ls.currentHeight = height
	ls.currentTs = ts
	ls.prevRefTime = ls.currentRefTime // one back
	ls.currentRefTime = refTime
	ls.currentProposer = proposer
	ls.currentMerkleRoot = newRoot
	ls.currentNumKeys = newNumKeys
	ls.currentSize = newSize

	ls.logger.Info("rollingRevisions received revision", logfields.BlockHeight(height))

	// TODO(v1) - move this a separate goroutine to prevent addRevision from blocking on IO
	// TODO(v1) - consider blocking the maximum length of revisions - to prevent crashing in case of failed flushes

	return ls.evictRevisions()
}

func (ls *rollingRevisions) calcNewSizes(diff adapter.ChainState) (primitives.StorageKeys, uint64, error) {
	currStorageKeys := ls.currentNumKeys
	currStorageSize := ls.currentSize

	for contractName, contractState := range diff {
		for key, value := range contractState {
			currentSize, err:= ls.getRevisionRecordCurrentSize(contractName, key)
			if err != nil {
				return 0, 0, err
			}
			newSize := len(value)
			if currentSize == 0 && newSize > 0 {
				currStorageKeys++
			} else if currentSize >0 && newSize == 0 {
				currStorageKeys--
			}
			currStorageSize = currStorageSize - uint64(currentSize) + uint64(newSize)
		}
	}

	return currStorageKeys, currStorageSize, nil
}

func toMerkleInput(diff adapter.ChainState) merkle.TrieDiffs {
	result := make(merkle.TrieDiffs, 0, len(diff))
	for contractName, contractState := range diff {
		for key, value := range contractState {
			result = append(result, &merkle.TrieDiff{
				Key:   hash.CalcSha256([]byte(contractName), []byte(key)),
				Value: hash.CalcSha256(value),
			})
		}
	}
	return result
}

func (ls *rollingRevisions) evictRevisions() error {
	for len(ls.revisions) > ls.transientRevisions {
		d := ls.revisions[0]
		err := ls.persist.Write(d.height, d.ts, d.ref, d.prevRef, d.proposer, d.merkleRoot, d.diff)
		if err != nil {
			return err
		}
		ls.merkle.Forget(ls.persistedRoot)

		ls.persistedHeight = d.height
		ls.persistedTs = d.ts
		ls.persistedRefTime = d.ref
		ls.persistedPrevRefTime = d.prevRef
		ls.persistedProposer = d.proposer
		ls.persistedRoot = d.merkleRoot
		ls.revisions = ls.revisions[1:]
	}
	return nil
}

func (ls *rollingRevisions) getRevisionRecord(height primitives.BlockHeight, contract primitives.ContractName, key string) ([]byte, bool, error) {
	if ls.currentHeight < height {
		return nil, false, errors.Errorf("requested height %d is too new. most recent available block height is %d", height, ls.currentHeight)
	}

	for i := len(ls.revisions) - 1; i >= 0; i-- {
		if ls.revisions[i].height > height {
			continue
		}
		if record, exists := ls.revisions[i].diff[contract][key]; exists {
			return record, !isZeroValue(record), nil // cached state increments must include zero values
		}
	}

	if ls.persistedHeight > height {
		return nil, false, errors.Errorf("requested height %d is too old. oldest available block height is %d", height, ls.persistedHeight)
	}
	return ls.persist.Read(contract, key)
}

func (ls *rollingRevisions) getRevisionHash(height primitives.BlockHeight) (primitives.Sha256, error) {
	for i := len(ls.revisions) - 1; i >= 0; i-- {
		if ls.revisions[i].height == height {
			return ls.revisions[i].merkleRoot, nil
		}
	}

	if height != ls.persistedHeight {
		return nil, fmt.Errorf("could not locate merkle hash for height %d. oldest available block height is %d", height, ls.persistedHeight)
	}

	return ls.persistedRoot, nil
}

func (ls *rollingRevisions) getRevisionRecordCurrentSize(contract primitives.ContractName, key string) (int, error) {
	for i := len(ls.revisions) - 1; i >= 0; i-- {
		if record, exists := ls.revisions[i].diff[contract][key]; exists {
			return len(record), nil
		}
	}

	record, exists, err := ls.persist.Read(contract, key)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, nil
	}
	return len(record), nil
}

func isZeroValue(value []byte) bool {
	return bytes.Equal(value, []byte{})
}
