package adapter

import (
	"context"
	"fmt"
	"github.com/orbs-network/orbs-network-go/crypto/digest"
	"github.com/orbs-network/orbs-network-go/services/blockstorage/adapter"
	"github.com/orbs-network/orbs-network-go/synchronization"
	"github.com/orbs-network/orbs-network-go/test"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/pkg/errors"
	"sync"
	"time"
)

type InMemoryBlockPersistence interface {
	adapter.BlockPersistence
	FailNextBlocks()
	WaitForTransaction(ctx context.Context, txhash primitives.Sha256) primitives.BlockHeight
}

type blockHeightChan chan primitives.BlockHeight

type inMemoryBlockPersistence struct {
	blockChain struct {
		sync.RWMutex
		blocks []*protocol.BlockPairContainer
	}

	failNextBlocks bool
	tracker        *synchronization.BlockTracker

	blockHeightsPerTxHash struct {
		sync.Mutex
		channels map[string]blockHeightChan
	}
}

func NewInMemoryBlockPersistence() InMemoryBlockPersistence {
	p := &inMemoryBlockPersistence{
		failNextBlocks: false,
		tracker:        synchronization.NewBlockTracker(0, 5),
	}

	p.blockHeightsPerTxHash.channels = make(map[string]blockHeightChan)

	return p
}

func (bp *inMemoryBlockPersistence) GetBlockTracker() *synchronization.BlockTracker {
	return bp.tracker
}

func (bp *inMemoryBlockPersistence) WaitForTransaction(ctx context.Context, txhash primitives.Sha256) primitives.BlockHeight {
	ch := bp.getChanFor(txhash)

	select {
	case h := <-ch:
		return h
	case <-ctx.Done():
		test.DebugPrintGoroutineStacks() // since test timed out, help find deadlocked goroutines
		panic(fmt.Sprintf("timed out waiting for transaction with hash %s", txhash))
	}
}

func (bp *inMemoryBlockPersistence) WriteBlock(blockPair *protocol.BlockPairContainer) error {
	if bp.failNextBlocks {
		return errors.New("could not write a block")
	}

	bp.addBlockToInMemoryChain(blockPair)

	bp.tracker.IncrementHeight()

	bp.advertiseAllTransactions(blockPair.TransactionsBlock)

	return nil
}

func (bp *inMemoryBlockPersistence) ReadAllBlocks() []*protocol.BlockPairContainer {
	return bp.getBlockPairs()
}

func (bp *inMemoryBlockPersistence) GetReceiptRelevantBlocks(txTimeStamp primitives.TimestampNano, rules adapter.BlockSearchRules) []*protocol.BlockPairContainer {
	start := txTimeStamp - primitives.TimestampNano(rules.StartGraceNano)
	end := txTimeStamp + primitives.TimestampNano(rules.EndGraceNano+rules.TransactionExpireNano)

	if end < start {
		return nil
	}
	var relevantBlocks []*protocol.BlockPairContainer
	interval := end - start
	// TODO: FIXME: sanity check, this is really useless here right now, but we are going to refactor this in about two-three weeks, and when we do, this is here to remind us to have a sanity check on this query
	if interval > primitives.TimestampNano(time.Hour.Nanoseconds()) {
		return nil
	}

	blockPairs := bp.getBlockPairs()

	for _, blockPair := range blockPairs {
		delta := end - blockPair.TransactionsBlock.Header.Timestamp()
		if delta > 0 && interval > delta {
			relevantBlocks = append(relevantBlocks, blockPair)
		}
	}
	return relevantBlocks
}

func (bp *inMemoryBlockPersistence) GetTransactionsBlock(height primitives.BlockHeight) (*protocol.TransactionsBlockContainer, error) {
	blockPairs := bp.getBlockPairs()

	for _, blockPair := range blockPairs {
		if blockPair.TransactionsBlock.Header.BlockHeight() == height {
			return blockPair.TransactionsBlock, nil
		}
	}

	return nil, fmt.Errorf("transactions block header with height %v not found", height)
}

func (bp *inMemoryBlockPersistence) GetResultsBlock(height primitives.BlockHeight) (*protocol.ResultsBlockContainer, error) {
	blockPairs := bp.getBlockPairs()

	for _, blockPair := range blockPairs {
		if blockPair.TransactionsBlock.Header.BlockHeight() == height {
			return blockPair.ResultsBlock, nil
		}
	}

	return nil, fmt.Errorf("results block header with height %v not found", height)
}

func (bp *inMemoryBlockPersistence) FailNextBlocks() {
	bp.failNextBlocks = true
}

// Is covered by the mutex in WriteBlock
func (bp *inMemoryBlockPersistence) getChanFor(txhash primitives.Sha256) blockHeightChan {
	bp.blockHeightsPerTxHash.Lock()
	defer bp.blockHeightsPerTxHash.Unlock()

	ch, ok := bp.blockHeightsPerTxHash.channels[txhash.KeyForMap()]
	if !ok {
		ch = make(blockHeightChan, 1)
		bp.blockHeightsPerTxHash.channels[txhash.KeyForMap()] = ch
	}

	return ch
}

func (bp *inMemoryBlockPersistence) advertiseAllTransactions(block *protocol.TransactionsBlockContainer) {
	for _, tx := range block.SignedTransactions {
		ch := bp.getChanFor(digest.CalcTxHash(tx.Transaction()))
		select {
		case ch <- block.Header.BlockHeight():
		default:
			// FIXME: this happens when two txid are in different blocks (or same block), this should never happen and we do not log it here (too low) and we also do not want to stop the loop (break/return error)
			continue
		}
	}
}

func (bp *inMemoryBlockPersistence) GetLastBlock() (*protocol.BlockPairContainer, error) {
	blockPairs := bp.getBlockPairs()

	count := len(blockPairs)

	if count == 0 {
		return nil, nil
	}

	return blockPairs[count-1], nil
}

func (bp *inMemoryBlockPersistence) getBlockPairs() []*protocol.BlockPairContainer {
	bp.blockChain.RLock()
	defer bp.blockChain.RUnlock()

	return bp.blockChain.blocks
}

func (bp *inMemoryBlockPersistence) addBlockToInMemoryChain(blockPair *protocol.BlockPairContainer) {
	bp.blockChain.Lock()
	defer bp.blockChain.Unlock()

	bp.blockChain.blocks = append(bp.blockChain.blocks, blockPair)
}

func (bp *inMemoryBlockPersistence) GetBlocks(first primitives.BlockHeight, last primitives.BlockHeight) ([]*protocol.BlockPairContainer, error) {
	if first > last {
		return nil, errors.Errorf("invalid block range: %s > %s", first, last)
	}

	blockPairs := bp.getBlockPairs()
	size := primitives.BlockHeight(len(blockPairs))
	if size < last {
		last = size
	}

	return blockPairs[first-1 : last], nil
}
