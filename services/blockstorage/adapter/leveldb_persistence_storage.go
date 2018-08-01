package adapter

import (
	"fmt"
	"github.com/orbs-network/orbs-network-go/instrumentation"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/syndtr/goleveldb/leveldb/util"
	"strconv"
)

func (bp *levelDbBlockPersistence) put(key string, value []byte) error {
	return bp.db.Put([]byte(key), value, nil)
}

func (bp *levelDbBlockPersistence) retrieve(key string) ([]byte, error) {
	return bp.db.Get([]byte(key), nil)
}

func (bp *levelDbBlockPersistence) retrieveByPrefix(prefix string) (results [][]byte) {
	iter := bp.db.NewIterator(util.BytesPrefix([]byte(prefix)), nil)

	for iter.Next() {
		results = append(results, iter.Value())
	}

	iter.Release()

	return results
}

func (bp *levelDbBlockPersistence) revert(key string) error {
	return bp.db.Delete([]byte(key), nil)
}

func (bp *levelDbBlockPersistence) putTxBlock(txBlock *protocol.TransactionsBlockContainer) (errors []error, keys []string) {
	blockHeight := txBlock.Header.BlockHeight().String()

	txBlockHeaderKey := TX_BLOCK_HEADER + blockHeight
	txBlockProofKey := TX_BLOCK_PROOF + blockHeight
	txBlockMetadataKey := TX_BLOCK_METADATA + blockHeight

	txBlockHeaderError := bp.put(txBlockHeaderKey, txBlock.Header.Raw())
	txBlockProofError := bp.put(txBlockProofKey, txBlock.BlockProof.Raw())
	txBlockMetadataError := bp.put(txBlockMetadataKey, txBlock.Metadata.Raw())

	keys = append(keys, txBlockHeaderKey, txBlockProofKey, txBlockMetadataKey)
	errors = append(errors, txBlockHeaderError, txBlockProofError, txBlockMetadataError)

	for i, tx := range txBlock.SignedTransactions {
		txBlockSignedTransactionKey := TX_BLOCK_SIGNED_TRANSACTION + blockHeight + "-" + formatInt(i)
		txBlockSignedTransactionError := bp.put(txBlockSignedTransactionKey, tx.Raw())

		keys = append(keys, txBlockSignedTransactionKey)
		errors = append(errors, txBlockSignedTransactionError)
	}

	return errors, keys
}

func (bp *levelDbBlockPersistence) putResultsBlock(rsBlock *protocol.ResultsBlockContainer) (errors []error, keys []string) {
	blockHeight := rsBlock.Header.BlockHeight().String()

	rsBlockHeaderKey := RS_BLOCK_HEADER + blockHeight
	rsBlockProofKey := RS_BLOCK_PROOF + blockHeight

	rsBlockHeaderError := bp.put(rsBlockHeaderKey, rsBlock.Header.Raw())
	rsBlockProofError := bp.put(rsBlockProofKey, rsBlock.BlockProof.Raw())

	keys = append(keys, rsBlockHeaderKey, rsBlockProofKey)
	errors = append(errors, rsBlockHeaderError, rsBlockProofError)

	for i, sd := range rsBlock.ContractStateDiffs {
		rsBlockContractStatesDiffsKey := RS_BLOCK_CONTRACT_STATE_DIFFS + blockHeight + "-" + formatInt(i)
		rsBlockContractStatesDiffsError := bp.put(rsBlockContractStatesDiffsKey, sd.Raw())

		keys = append(keys, rsBlockContractStatesDiffsKey)
		errors = append(errors, rsBlockContractStatesDiffsError)
	}

	for i, tr := range rsBlock.TransactionReceipts {
		rsBlockTransactionReceiptsKey := RS_BLOCK_TRANSACTION_RECEIPTS + blockHeight + "-" + formatInt(i)
		rsBlockTransactionReceiptsError := bp.put(rsBlockTransactionReceiptsKey, tr.Raw())

		keys = append(keys, rsBlockTransactionReceiptsKey)
		errors = append(errors, rsBlockTransactionReceiptsError)
	}

	return errors, keys
}

func constructTxBlockFromStorage(txBlockHeaderRaw []byte, txBlockProofRaw []byte, txBlockMetadataRaw []byte,
	txBlockSignedTransactionsRaw [][]byte) *protocol.TransactionsBlockContainer {
	var signedTransactions []*protocol.SignedTransaction

	for _, txRaw := range txBlockSignedTransactionsRaw {
		signedTransactions = append(signedTransactions, protocol.SignedTransactionReader(txRaw))
	}

	transactionsBlock := &protocol.TransactionsBlockContainer{
		Header:             protocol.TransactionsBlockHeaderReader(txBlockHeaderRaw),
		BlockProof:         protocol.TransactionsBlockProofReader(txBlockProofRaw),
		Metadata:           protocol.TransactionsBlockMetadataReader(txBlockMetadataRaw),
		SignedTransactions: signedTransactions,
	}

	return transactionsBlock
}

func constructResultsBlockFromStorage(rsBlockHeaderRaw []byte, rsBlockProofRaw []byte, rsBlockStateDiffsRaw [][]byte, rsTransactionReceiptsRaw [][]byte) *protocol.ResultsBlockContainer {
	var transactionReceipts []*protocol.TransactionReceipt
	var stateDiffs []*protocol.ContractStateDiff

	for _, trRaw := range rsTransactionReceiptsRaw {
		transactionReceipts = append(transactionReceipts, protocol.TransactionReceiptReader(trRaw))
	}

	for _, sdRaw := range rsBlockStateDiffsRaw {
		stateDiffs = append(stateDiffs, protocol.ContractStateDiffReader(sdRaw))
	}

	resultsBlock := &protocol.ResultsBlockContainer{
		Header:              protocol.ResultsBlockHeaderReader(rsBlockHeaderRaw),
		BlockProof:          protocol.ResultsBlockProofReader(rsBlockProofRaw),
		ContractStateDiffs:  stateDiffs,
		TransactionReceipts: transactionReceipts,
	}

	return resultsBlock
}

func (bp *levelDbBlockPersistence) loadLastBlockHeight() (primitives.BlockHeight, error) {
	val, err := bp.db.Get([]byte(LAST_BLOCK_HEIGHT), nil)

	if err != nil {
		return 0, nil
	}

	result, err := strconv.ParseUint(string(val), 16, 64)
	return primitives.BlockHeight(result), err
}

func (bp *levelDbBlockPersistence) loadLastBlockTimestamp() (primitives.TimestampNano, error) {
	val, err := bp.db.Get([]byte(LAST_BLOCK_TIMESTAMP), nil)

	if err != nil {
		return 0, nil
	}

	result, err := strconv.ParseUint(string(val), 16, 64)
	return primitives.TimestampNano(result), err
}

func (bp *levelDbBlockPersistence) saveLastBlockHeight(height primitives.BlockHeight) error {
	return bp.db.Put([]byte(LAST_BLOCK_HEIGHT), []byte(height.String()), nil)
}

func basicValidation(blockPair *protocol.BlockPairContainer) bool {
	var validations []bool

	validations = append(validations, blockPair.TransactionsBlock.Header.IsValid(), blockPair.TransactionsBlock.BlockProof.IsValid(), blockPair.TransactionsBlock.Metadata.IsValid())

	for _, tx := range blockPair.TransactionsBlock.SignedTransactions {
		validations = append(validations, tx.IsValid())
	}

	return anyConditions(validations)
}

func (bp *levelDbBlockPersistence) loadTransactionsBlock(height primitives.BlockHeight) (*protocol.TransactionsBlockContainer, error) {
	blockHeightAsString := height.String()

	txBlockHeaderRaw, txBlockHeaderRawError := bp.retrieve(TX_BLOCK_HEADER + blockHeightAsString)
	txBlockProofRaw, txBlockProofError := bp.retrieve(TX_BLOCK_PROOF + blockHeightAsString)
	txBlockMetadataRaw, txBlockMetadataError := bp.retrieve(TX_BLOCK_METADATA + blockHeightAsString)

	txSignedTransactionsRaw := bp.retrieveByPrefix(TX_BLOCK_SIGNED_TRANSACTION + blockHeightAsString + "-")

	if hasErrors, firstError := anyErrors(txBlockHeaderRawError, txBlockProofError, txBlockMetadataError); hasErrors {
		errorMessage := "failed to retrieve transactions block from storage"
		bp.reporting.Error(errorMessage, instrumentation.Error(firstError))
		return nil, fmt.Errorf(errorMessage)
	}

	bp.reporting.Info("Retrieved transactions block from storage", instrumentation.BlockHeight(height))

	return constructTxBlockFromStorage(txBlockHeaderRaw, txBlockProofRaw, txBlockMetadataRaw, txSignedTransactionsRaw), nil
}

func (bp *levelDbBlockPersistence) loadResultsBlock(height primitives.BlockHeight) (*protocol.ResultsBlockContainer, error) {
	blockHeightAsString := height.String()

	rsBlockHeaderRaw, rsBlockHeaderRawError := bp.retrieve(RS_BLOCK_HEADER + blockHeightAsString)
	rsBlockProofRaw, rsBlockProofRawError := bp.retrieve(RS_BLOCK_PROOF + blockHeightAsString)

	rsTransactionReceipts := bp.retrieveByPrefix(RS_BLOCK_TRANSACTION_RECEIPTS + blockHeightAsString + "-")
	rsStateDiffs := bp.retrieveByPrefix(RS_BLOCK_CONTRACT_STATE_DIFFS + blockHeightAsString + "-")

	if hasErrors, firstError := anyErrors(rsBlockHeaderRawError, rsBlockProofRawError); hasErrors {
		errorMessage := "failed to retrieve results block from storage"
		bp.reporting.Error(errorMessage, instrumentation.Error(firstError))
		return nil, fmt.Errorf(errorMessage)
	}

	bp.reporting.Info("Retrieved results block from storage", instrumentation.BlockHeight(height))

	return constructResultsBlockFromStorage(rsBlockHeaderRaw, rsBlockProofRaw, rsStateDiffs, rsTransactionReceipts), nil
}
