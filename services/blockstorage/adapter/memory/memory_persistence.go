// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package memory

import (
	"fmt"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/services/blockstorage/adapter"
	"github.com/orbs-network/orbs-network-go/services/blockstorage/internodesync"
	"github.com/orbs-network/orbs-network-go/synchronization"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/scribe/log"
	"github.com/pkg/errors"
	"sync"
	"unsafe"
)

type memMetrics struct {
	size *metric.Gauge
}

type aChainOfBlocks struct {
	sync.RWMutex
	blocks             map[primitives.BlockHeight]*protocol.BlockPairContainer
	sequentialTopBlock *protocol.BlockPairContainer
	topBlock           *protocol.BlockPairContainer
	lastWrittenBlock   *protocol.BlockPairContainer
}

type InMemoryBlockPersistence struct {
	blockChain aChainOfBlocks

	tracker *synchronization.BlockTracker
	Logger  log.Logger

	metrics *memMetrics
}

func NewBlockPersistence(parent log.Logger, metricFactory metric.Factory, preloadedBlocks ...*protocol.BlockPairContainer) *InMemoryBlockPersistence {
	logger := parent.WithTags(log.String("adapter", "block-storage"))
	p := &InMemoryBlockPersistence{
		Logger:  logger,
		metrics: &memMetrics{size: metricFactory.NewGauge("BlockStorage.InMemoryBlockPersistenceSize.Bytes")},
	}
	p.createChainOfBlocks(preloadedBlocks) // this is needed so that each instance of BlockPersistence has its own copy of the block chain
	startingHeight := uint64(getBlockHeight(p.blockChain.sequentialTopBlock))
	p.tracker = synchronization.NewBlockTracker(logger, startingHeight, 5)
	return p
}

func (bp *InMemoryBlockPersistence) GetSyncState() internodesync.SyncState {
	bp.blockChain.RLock()
	defer bp.blockChain.RUnlock()

	return internodesync.SyncState{
		TopBlock:        bp.blockChain.topBlock,
		InOrderBlock:    bp.blockChain.sequentialTopBlock,
		LastSyncedBlock: bp.blockChain.lastWrittenBlock,
	}
}

func (bp *InMemoryBlockPersistence) createChainOfBlocks(blocks []*protocol.BlockPairContainer) {

	bp.blockChain = aChainOfBlocks{
		RWMutex: sync.RWMutex{},
		blocks:  make(map[primitives.BlockHeight]*protocol.BlockPairContainer),
	}
	count := len(blocks)
	if count > 0 {
		for _, block := range blocks {
			bp.WriteNextBlock(block)
		}
	}
}

func getBlockHeight(block *protocol.BlockPairContainer) primitives.BlockHeight {
	if block == nil {
		return 0
	}
	return block.TransactionsBlock.Header.BlockHeight()
}

func (bp *InMemoryBlockPersistence) GetBlockTracker() *synchronization.BlockTracker {
	return bp.tracker
}

func (bp *InMemoryBlockPersistence) GetLastBlock() (*protocol.BlockPairContainer, error) {
	bp.blockChain.RLock()
	defer bp.blockChain.RUnlock()

	return bp.blockChain.sequentialTopBlock, nil
}

func (bp *InMemoryBlockPersistence) GetLastBlockHeight() (primitives.BlockHeight, error) {
	bp.blockChain.RLock()
	defer bp.blockChain.RUnlock()

	return getBlockHeight(bp.blockChain.sequentialTopBlock), nil
}

func (bp *InMemoryBlockPersistence) WriteNextBlock(blockPair *protocol.BlockPairContainer) (bool, primitives.BlockHeight, error) {

	added, pHeight := bp.validateAndAddNextBlock(blockPair)

	if added {
		bp.metrics.size.Add(sizeOfBlock(blockPair))
	}

	return added, pHeight, nil
}

func (bp *InMemoryBlockPersistence) validateAndAddNextBlock(blockPair *protocol.BlockPairContainer) (bool, primitives.BlockHeight) {
	bp.blockChain.Lock()
	defer bp.blockChain.Unlock()

	newBlockHeight := getBlockHeight(blockPair)
	sequentialHeight := getBlockHeight(bp.blockChain.sequentialTopBlock)
	lastWrittenHeight := getBlockHeight(bp.blockChain.lastWrittenBlock)
	topHeight := getBlockHeight(bp.blockChain.topBlock)

	var err error
	if lastWrittenHeight > sequentialHeight && newBlockHeight != lastWrittenHeight-1 {
		err = fmt.Errorf("sync session in progress, expected block height %d; candidate block height %d", lastWrittenHeight-1, newBlockHeight)
	} else if sequentialHeight == topHeight && newBlockHeight <= sequentialHeight {
		err = fmt.Errorf("expected block height higher than current top %d; candidate block height %d", sequentialHeight, newBlockHeight)
	}

	if err != nil {
		bp.Logger.Info(err.Error())
		return false, sequentialHeight
	}

	bp.blockChain.blocks[newBlockHeight] = blockPair
	bp.blockChain.lastWrittenBlock = blockPair
	lastWrittenHeight = newBlockHeight
	if newBlockHeight > topHeight {
		bp.blockChain.topBlock = blockPair
		topHeight = newBlockHeight
	}

	if lastWrittenHeight == sequentialHeight+1 { // gap was closed storage holds consecutive blocks 1-topHeight
		for height := sequentialHeight + 1; height <= topHeight; height++ { // update indices and blockTracker
			if block, _ := bp.blockChain.blocks[height]; block == nil {
				bp.Logger.Error(fmt.Sprintf("missing block with height (%d) - should not happen", uint64(height)))
				bp.blockChain.lastWrittenBlock = bp.blockChain.topBlock
				return false, getBlockHeight(bp.blockChain.sequentialTopBlock)
			}
			if bp.tracker != nil {
				bp.tracker.IncrementTo(height)
			}
		}
		bp.blockChain.sequentialTopBlock = bp.blockChain.topBlock
		bp.blockChain.lastWrittenBlock = bp.blockChain.topBlock
	}

	return true, getBlockHeight(bp.blockChain.sequentialTopBlock)
}

func (bp *InMemoryBlockPersistence) GetBlockByTx(txHash primitives.Sha256, minBlockTs primitives.TimestampNano, maxBlockTs primitives.TimestampNano) (*protocol.BlockPairContainer, int, error) {

	bp.blockChain.RLock()
	defer bp.blockChain.RUnlock()

	var candidateBlocks []*protocol.BlockPairContainer
	sequentialHeight := getBlockHeight(bp.blockChain.sequentialTopBlock)
	for height := primitives.BlockHeight(1); height <= sequentialHeight; height++ {
		if blockPair, _ := bp.blockChain.blocks[height]; blockPair != nil {
			bts := blockPair.TransactionsBlock.Header.Timestamp()
			if maxBlockTs < bts {
				break
			} else if minBlockTs <= bts {
				candidateBlocks = append(candidateBlocks, blockPair)
			}
		}
	}

	if len(candidateBlocks) == 0 {
		return nil, 0, nil
	}

	for _, b := range candidateBlocks {
		for txi, txr := range b.ResultsBlock.TransactionReceipts {
			if txr.Txhash().Equal(txHash) {
				return b, txi, nil
			}
		}
	}
	return nil, 0, nil
}

func (bp *InMemoryBlockPersistence) getBlockPairAtHeight(height primitives.BlockHeight) (*protocol.BlockPairContainer, error) {
	bp.blockChain.RLock()
	defer bp.blockChain.RUnlock()

	if block, ok := bp.blockChain.blocks[height]; ok {
		return block, nil
	} else {
		return nil, errors.Errorf("block with height %d not found in block persistence", height)
	}
}

func (bp *InMemoryBlockPersistence) GetBlock(height primitives.BlockHeight) (*protocol.BlockPairContainer, error) {
	return bp.getBlockPairAtHeight(height)
}

func (bp *InMemoryBlockPersistence) GetTransactionsBlock(height primitives.BlockHeight) (*protocol.TransactionsBlockContainer, error) {
	blockPair, err := bp.getBlockPairAtHeight(height)
	if err != nil {
		return nil, err
	}
	return blockPair.TransactionsBlock, nil
}

func (bp *InMemoryBlockPersistence) GetResultsBlock(height primitives.BlockHeight) (*protocol.ResultsBlockContainer, error) {
	blockPair, err := bp.getBlockPairAtHeight(height)
	if err != nil {
		return nil, err
	}
	return blockPair.ResultsBlock, nil
}

// supports two blockHeight ranges - (1-sequentialTop), (lastWritten-top)
func (bp *InMemoryBlockPersistence) ScanBlocks(from primitives.BlockHeight, pageSize uint8, f adapter.CursorFunc) error {
	bp.blockChain.RLock()
	defer bp.blockChain.RUnlock()

	sequentialHeight := getBlockHeight(bp.blockChain.sequentialTopBlock)
	if (sequentialHeight < from) || from == 0 {
		return fmt.Errorf("requested unsupported block height %d. Supported range for scan is determined by sequentialTop(%d)", from, sequentialHeight)
	}

	fromHeight := from
	wantsMore := true
	for fromHeight <= sequentialHeight && wantsMore {
		toHeight := fromHeight + primitives.BlockHeight(pageSize) - 1
		if toHeight > sequentialHeight {
			toHeight = sequentialHeight
		}
		page := make([]*protocol.BlockPairContainer, 0, pageSize)
		for height := fromHeight; height <= toHeight; height++ {
			aBlock, _ := bp.blockChain.blocks[height]
			if aBlock == nil {
				break
			}
			page = append(page, aBlock)
		}
		if len(page) > 0 {
			wantsMore = f(fromHeight, page)
		}
		fromHeight = toHeight + 1
		sequentialHeight = getBlockHeight(bp.blockChain.sequentialTopBlock)
	}
	return nil
}

func sizeOfBlock(block *protocol.BlockPairContainer) int64 {
	txBlock := block.TransactionsBlock
	txBlockSize := len(txBlock.Header.Raw()) + len(txBlock.BlockProof.Raw()) + len(txBlock.Metadata.Raw())

	rsBlock := block.ResultsBlock
	rsBlockSize := len(rsBlock.Header.Raw()) + len(rsBlock.BlockProof.Raw())

	txBlockPointers := unsafe.Sizeof(txBlock) + unsafe.Sizeof(txBlock.Header) + unsafe.Sizeof(txBlock.Metadata) + unsafe.Sizeof(txBlock.BlockProof) + unsafe.Sizeof(txBlock.SignedTransactions)
	rsBlockPointers := unsafe.Sizeof(rsBlock) + unsafe.Sizeof(rsBlock.Header) + unsafe.Sizeof(rsBlock.BlockProof) + unsafe.Sizeof(rsBlock.TransactionReceipts) + unsafe.Sizeof(rsBlock.ContractStateDiffs)

	for _, tx := range txBlock.SignedTransactions {
		txBlockSize += len(tx.Raw())
		txBlockPointers += unsafe.Sizeof(tx)
	}

	for _, diff := range rsBlock.ContractStateDiffs {
		rsBlockSize += len(diff.Raw())
		rsBlockPointers += unsafe.Sizeof(diff)
	}

	for _, receipt := range rsBlock.TransactionReceipts {
		rsBlockSize += len(receipt.Raw())
		rsBlockPointers += unsafe.Sizeof(receipt)
	}

	pointers := unsafe.Sizeof(block) + txBlockPointers + rsBlockPointers

	return int64(txBlockSize) + int64(rsBlockSize) + int64(pointers)
}
