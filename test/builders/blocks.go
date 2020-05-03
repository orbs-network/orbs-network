// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package builders

import (
	"fmt"
	"github.com/orbs-network/crypto-lib-go/crypto/digest"
	"github.com/orbs-network/crypto-lib-go/crypto/hash"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/test/crypto/keys"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"time"
)

/// Test builders for: protocol.BlockPairContainer

type blockPair struct {
	txHeader      *protocol.TransactionsBlockHeaderBuilder
	txMetadata    *protocol.TransactionsBlockMetadataBuilder
	transactions  []*protocol.SignedTransaction
	txProof       *protocol.TransactionsBlockProofBuilder
	rxHeader      *protocol.ResultsBlockHeaderBuilder
	receipts      []*protocol.TransactionReceipt
	sdiffs        []*protocol.ContractStateDiff
	rxProof       *protocol.ResultsBlockProofBuilder
	blockProofKey primitives.EcdsaSecp256K1PrivateKey
}

func BlockPair() *blockPair {
	// allocate size for empty fields or you'll get "size mismatch" errors from membuffers
	empty32ByteHash := hash.Make32EmptyBytes()
	createdDate := time.Now()
	transactions := []*protocol.SignedTransaction{
		(TransferTransaction().WithAmountAndTargetAddress(10, ClientAddressForEd25519SignerForTests(6))).Build(),
	}

	b := &blockPair{
		txHeader: &protocol.TransactionsBlockHeaderBuilder{
			ProtocolVersion:            config.MAXIMAL_PROTOCOL_VERSION_SUPPORTED_VALUE,
			VirtualChainId:             DEFAULT_TEST_VIRTUAL_CHAIN_ID,
			BlockHeight:                1,
			PrevBlockHashPtr:           empty32ByteHash,
			Timestamp:                  primitives.TimestampNano(createdDate.UnixNano()),
			TransactionsMerkleRootHash: empty32ByteHash,
			MetadataHash:               empty32ByteHash,
			NumSignedTransactions:      1,
			BlockProposerAddress:       empty32ByteHash,
		},
		txMetadata:   &protocol.TransactionsBlockMetadataBuilder{},
		transactions: transactions,
		txProof:      nil,
		rxHeader: &protocol.ResultsBlockHeaderBuilder{
			ProtocolVersion:                 config.MAXIMAL_PROTOCOL_VERSION_SUPPORTED_VALUE,
			VirtualChainId:                  DEFAULT_TEST_VIRTUAL_CHAIN_ID,
			BlockHeight:                     1,
			PrevBlockHashPtr:                empty32ByteHash,
			Timestamp:                       primitives.TimestampNano(createdDate.UnixNano()),
			ReceiptsMerkleRootHash:          empty32ByteHash,
			StateDiffHash:                   empty32ByteHash,
			TransactionsBlockHashPtr:        empty32ByteHash,
			PreExecutionStateMerkleRootHash: empty32ByteHash,
			NumContractStateDiffs:           1,
			NumTransactionReceipts:          1,
			BlockProposerAddress:            empty32ByteHash,
		},
		receipts: []*protocol.TransactionReceipt{
			(TransactionReceipt().Build()),
		},
		sdiffs: []*protocol.ContractStateDiff{
			(ContractStateDiff().Build()),
		},
		rxProof: nil,
	}
	return b.WithBenchmarkConsensusBlockProof(keys.EcdsaSecp256K1KeyPairForTests(0))
}

func (b *blockPair) Build() *protocol.BlockPairContainer {
	txHeaderBuilt := b.txHeader.Build()
	rxHeaderBuilt := b.rxHeader.Build()

	if b.rxProof.Type == protocol.RESULTS_BLOCK_PROOF_TYPE_BENCHMARK_CONSENSUS {
		b.buildBenchmarkConsensusBlockProof(txHeaderBuilt, rxHeaderBuilt)
	}

	return &protocol.BlockPairContainer{
		TransactionsBlock: &protocol.TransactionsBlockContainer{
			Header:             txHeaderBuilt,
			Metadata:           b.txMetadata.Build(),
			SignedTransactions: b.transactions,
			BlockProof:         b.txProof.Build(),
		},
		ResultsBlock: &protocol.ResultsBlockContainer{
			Header:              rxHeaderBuilt,
			TransactionReceipts: b.receipts,
			ContractStateDiffs:  b.sdiffs,
			BlockProof:          b.rxProof.Build(),
		},
	}
}

func (b *blockPair) WithHeight(blockHeight primitives.BlockHeight) *blockPair {
	b.txHeader.BlockHeight = blockHeight
	b.rxHeader.BlockHeight = blockHeight
	return b
}

func (b *blockPair) WithPrevBlock(prevBlock *protocol.BlockPairContainer) *blockPair {
	if prevBlock != nil {
		b.txHeader.PrevBlockHashPtr = digest.CalcTransactionsBlockHash(prevBlock.TransactionsBlock)
		b.rxHeader.PrevBlockHashPtr = digest.CalcResultsBlockHash(prevBlock.ResultsBlock)
	}
	return b
}

func (b *blockPair) WithPrevBlockHash(hash primitives.Sha256) *blockPair {
	b.txHeader.PrevBlockHashPtr = hash
	b.rxHeader.PrevBlockHashPtr = hash
	return b
}

func (b *blockPair) WithBlockCreated(time time.Time) *blockPair {
	b.txHeader.Timestamp = primitives.TimestampNano(time.UnixNano())
	b.rxHeader.Timestamp = primitives.TimestampNano(time.UnixNano())
	return b
}

func (b *blockPair) WithProtocolVersion(version primitives.ProtocolVersion) *blockPair {
	b.txHeader.ProtocolVersion = version
	b.rxHeader.ProtocolVersion = version
	return b
}

func (b *blockPair) WithVirtualChainId(virtualChainId primitives.VirtualChainId) *blockPair {
	b.txHeader.VirtualChainId = virtualChainId
	b.rxHeader.VirtualChainId = virtualChainId
	return b
}

func (b *blockPair) WithTransactionsRootHash(txRootHash []byte) *blockPair {
	b.txHeader.TransactionsMerkleRootHash = txRootHash
	return b
}

func (b *blockPair) WithMetadata(txMetadata *protocol.TransactionsBlockMetadataBuilder) *blockPair {
	b.txMetadata = txMetadata
	return b
}

func (b *blockPair) WithMetadataHash(metadataHash primitives.Sha256) *blockPair {
	b.txHeader.MetadataHash = metadataHash
	return b
}

func (b *blockPair) WithBlockProposerAddress(bpa primitives.NodeAddress) *blockPair {
	b.txHeader.BlockProposerAddress = bpa
	b.rxHeader.BlockProposerAddress = bpa
	return b
}

func (b *blockPair) WithTransactionsArray(txs []*protocol.SignedTransaction) *blockPair {
	b.transactions = txs
	b.txHeader.NumSignedTransactions = uint32(len(txs))
	return b
}

func (b *blockPair) WithTransactions(num uint32) *blockPair {
	b.transactions = make([]*protocol.SignedTransaction, 0, num)
	for i := uint32(0); i < num; i++ {
		b.transactions = append(b.transactions, TransferTransaction().WithAmountAndTargetAddress(uint64(10*num), ClientAddressForEd25519SignerForTests(6)).Build())
	}
	b.txHeader.NumSignedTransactions = num
	return b
}

func (b *blockPair) WithTransaction(tx *protocol.SignedTransaction) *blockPair {
	b.transactions = append(b.transactions, tx)
	b.txHeader.NumSignedTransactions = uint32(len(b.transactions))

	return b
}

func (b *blockPair) WithReceiptsForTransactions() *blockPair {
	b.receipts = make([]*protocol.TransactionReceipt, 0, len(b.transactions))
	for _, t := range b.transactions {
		b.receipts = append(b.receipts, TransactionReceipt().WithTransaction(t.Transaction()).Build())
	}
	b.rxHeader.NumTransactionReceipts = uint32(len(b.transactions))
	return b
}

func (b *blockPair) WithReceipts(num uint32) *blockPair {
	b.receipts = make([]*protocol.TransactionReceipt, 0, num)
	for i := uint32(0); i < num; i++ {
		b.receipts = append(b.receipts, TransactionReceipt().Build())
	}
	b.rxHeader.NumTransactionReceipts = num
	return b
}

func (b *blockPair) WithReceipt(receipt *protocol.TransactionReceipt) *blockPair {
	b.receipts = append(b.receipts, receipt)
	b.rxHeader.NumTransactionReceipts = uint32(len(b.receipts))

	return b
}

func (b *blockPair) WithStateDiffs(num uint32) *blockPair {
	b.sdiffs = make([]*protocol.ContractStateDiff, 0, num)
	for i := uint32(0); i < num; i++ {
		k := fmt.Sprintf("k_%d", i)
		v := fmt.Sprintf("v_%d", i)
		b.sdiffs = append(b.sdiffs, ContractStateDiff().WithStringRecord(k, v).Build())
	}
	b.rxHeader.NumContractStateDiffs = num
	return b
}

func (b *blockPair) WithTimestamp(timeToUse time.Time) *blockPair {
	tNano := primitives.TimestampNano(timeToUse.UnixNano())
	b.txHeader.Timestamp = tNano
	b.rxHeader.Timestamp = tNano
	return b
}

func (b *blockPair) WithTimestampNow() *blockPair {
	return b.WithTimestamp(time.Now())
}

func (b *blockPair) WithTimestampAheadBy(duration time.Duration) *blockPair {
	return b.WithTimestamp(time.Now().Add(duration))
}

func (b *blockPair) WithReceiptProofHash(hash primitives.Sha256) *blockPair {
	b.rxProof = &protocol.ResultsBlockProofBuilder{
		TransactionsBlockHash: hash,
		Type:                  protocol.RESULTS_BLOCK_PROOF_TYPE_LEAN_HELIX,
		BenchmarkConsensus:    nil,
		LeanHelix:             nil,
	}
	return b
}

func (b *blockPair) WithCorruptNumTransactions(num uint32) *blockPair {
	b.transactions = []*protocol.SignedTransaction{}
	b.txHeader.NumSignedTransactions = num
	return b
}

func (b *blockPair) WithCorruptNumReceipts(num uint32) *blockPair {
	b.receipts = []*protocol.TransactionReceipt{}
	b.rxHeader.NumTransactionReceipts = num
	return b
}

func (b *blockPair) WithCorruptNumStateDiffs(num uint32) *blockPair {
	b.sdiffs = []*protocol.ContractStateDiff{}
	b.rxHeader.NumContractStateDiffs = num
	return b
}

type corruptBlockPair struct {
	txContainer *protocol.TransactionsBlockContainer
	rxContainer *protocol.ResultsBlockContainer
}

func CorruptBlockPair() *corruptBlockPair {
	return &corruptBlockPair{}
}

func (c *corruptBlockPair) Build() *protocol.BlockPairContainer {
	return &protocol.BlockPairContainer{
		TransactionsBlock: c.txContainer,
		ResultsBlock:      c.rxContainer,
	}
}

func (c *corruptBlockPair) WithMissingTransactionsBlock() *corruptBlockPair {
	c.txContainer = nil
	return c
}

func (c *corruptBlockPair) WithMissingResultsBlock() *corruptBlockPair {
	c.rxContainer = nil
	return c
}

func (c *corruptBlockPair) WithEmptyTransactionsBlock() *corruptBlockPair {
	c.txContainer = &protocol.TransactionsBlockContainer{}
	return c
}

func (c *corruptBlockPair) WithEmptyResultsBlock() *corruptBlockPair {
	c.rxContainer = &protocol.ResultsBlockContainer{}
	return c
}

type blockPairBuilder struct {
	protocolVersion    primitives.ProtocolVersion
	virtualChainId     primitives.VirtualChainId
	currentBlockHeight primitives.BlockHeight
	blockProposer      primitives.NodeAddress
	tiggerEnabled      bool
}

func BlockPairBuilder() *blockPairBuilder {
	return &blockPairBuilder{
		protocolVersion:    config.MAXIMAL_PROTOCOL_VERSION_SUPPORTED_VALUE,
		virtualChainId:     DEFAULT_TEST_VIRTUAL_CHAIN_ID,
		currentBlockHeight: 1000,
		blockProposer:      hash.Make32EmptyBytes(),
		tiggerEnabled:      true,
	}
}

func (b *blockPairBuilder) Build() *protocol.BlockPairContainer {
	blockTime := time.Now()
	validPrevBlock := BlockPair().WithHeight(b.currentBlockHeight - 1).Build()
	validPrevBlockHash := digest.CalcTransactionsBlockHash(validPrevBlock.TransactionsBlock)
	transaction := TransferTransaction().WithProtocolVersion(b.protocolVersion).WithVirtualChainId(b.virtualChainId).WithAmountAndTargetAddress(10, ClientAddressForEd25519SignerForTests(6)).Build()
	transactionArray := []*protocol.SignedTransaction{transaction}
	if b.tiggerEnabled {
		transactionArray = append(transactionArray, TriggerTransaction().WithTimestamp(blockTime).WithProtocolVersion(b.protocolVersion).WithVirtualChainId(b.virtualChainId).Build())
	}
	txMetadata := &protocol.TransactionsBlockMetadataBuilder{}
	txRootHashForValidBlock, _ := digest.CalcTransactionsMerkleRoot(transactionArray)
	validMetadataHash := digest.CalcTransactionMetaDataHash(txMetadata.Build())

	block := BlockPair().
		WithHeight(b.currentBlockHeight).
		WithProtocolVersion(b.protocolVersion).
		WithVirtualChainId(b.virtualChainId).
		WithTransactionsArray(transactionArray).
		WithPrevBlock(validPrevBlock).
		WithPrevBlockHash(validPrevBlockHash).
		WithMetadata(txMetadata).
		WithMetadataHash(validMetadataHash).
		WithBlockProposerAddress(b.blockProposer).
		WithTransactionsRootHash(txRootHashForValidBlock).
		WithTimestamp(blockTime).
		Build()

	txBlockHashPtr := digest.CalcTransactionsBlockHash(block.TransactionsBlock)
	receiptMerkleRoot, _ := digest.CalcReceiptsMerkleRoot(block.ResultsBlock.TransactionReceipts)
	stateDiffHash, _ := digest.CalcStateDiffHash(block.ResultsBlock.ContractStateDiffs)
	preExecutionRootHash := &services.GetStateHashOutput{
		StateMerkleRootHash: hash.CalcSha256([]byte{1, 2, 3, 4, 5}),
	}

	block.ResultsBlock.Header.MutateTransactionsBlockHashPtr(txBlockHashPtr)
	block.ResultsBlock.Header.MutateReceiptsMerkleRootHash(receiptMerkleRoot)
	block.ResultsBlock.Header.MutateStateDiffHash(stateDiffHash)
	block.ResultsBlock.Header.MutatePreExecutionStateMerkleRootHash(preExecutionRootHash.StateMerkleRootHash)

	return block
}

type blockPairBuilderConfig interface {
	VirtualChainId() primitives.VirtualChainId
	ConsensusContextTriggersEnabled() bool
}

func (b *blockPairBuilder) WithCfg(cfg blockPairBuilderConfig) *blockPairBuilder {
	b.protocolVersion = config.MAXIMAL_PROTOCOL_VERSION_SUPPORTED_VALUE
	b.virtualChainId = cfg.VirtualChainId()
	b.tiggerEnabled = cfg.ConsensusContextTriggersEnabled()
	return b
}

func (b *blockPairBuilder) WithBlockProposerAddress(bpa primitives.NodeAddress) *blockPairBuilder {
	b.blockProposer = bpa
	return b
}

func MockCalcReceiptsMerkleRootThatReturns(root primitives.Sha256, err error) func(receipts []*protocol.TransactionReceipt) (primitives.Sha256, error) {
	return func(receipts []*protocol.TransactionReceipt) (primitives.Sha256, error) {
		return root, err
	}
}

func MockCalcStateDiffHashThatReturns(root primitives.Sha256, err error) func(stateDiffs []*protocol.ContractStateDiff) (primitives.Sha256, error) {
	return func(stateDiffs []*protocol.ContractStateDiff) (primitives.Sha256, error) {
		return root, err
	}
}
