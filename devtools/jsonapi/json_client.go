package jsonapi

import (
	"bytes"
	"github.com/orbs-network/orbs-network-go/crypto/digest"
	"github.com/orbs-network/orbs-network-go/crypto/signature"
	"github.com/orbs-network/orbs-network-go/instrumentation/log"
	"github.com/orbs-network/orbs-network-go/test/builders"
	"github.com/orbs-network/orbs-network-go/test/crypto/keys"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/protocol/client"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"time"
)

func ConvertAndSignTransaction(tx *Transaction, keyPair *keys.Ed25519KeyPair) (*protocol.SignedTransactionBuilder, error) {
	transaction := ConvertTransaction(tx)
	transaction.Signer = &protocol.SignerBuilder{
		Scheme: protocol.SIGNER_SCHEME_EDDSA, //TODO move to Transaction
		Eddsa: &protocol.EdDSA01SignerBuilder{
			NetworkType:     protocol.NETWORK_TYPE_TEST_NET, //TODO move to Transaction
			SignerPublicKey: primitives.Ed25519PublicKey(keyPair.PublicKey()),
		},
	}

	signedTransaction := &protocol.SignedTransactionBuilder{
		Transaction: transaction,
	}

	txHash := digest.CalcTxHash(transaction.Build())
	if sig, err := signature.SignEd25519(keyPair.PrivateKey(), txHash); err != nil {
		return nil, err
	} else {
		signedTransaction.Signature = sig
		return signedTransaction, nil
	}

}

func ConvertTransaction(tx *Transaction) *protocol.TransactionBuilder {
	var inputArguments []*protocol.MethodArgumentBuilder
	for _, arg := range tx.Arguments {
		inputArguments = append(inputArguments, &protocol.MethodArgumentBuilder{
			Name:        arg.Name,
			Type:        arg.Type,
			BytesValue:  arg.BytesValue,
			StringValue: arg.StringValue,
			Uint32Value: arg.Uint32Value,
			Uint64Value: arg.Uint64Value,
		})
	}
	inputArgumentArray := (&protocol.MethodArgumentArrayBuilder{Arguments: inputArguments}).Build()

	return &protocol.TransactionBuilder{
		ProtocolVersion:    1,
		VirtualChainId:     builders.DEFAULT_TEST_VIRTUAL_CHAIN_ID, //TODO move to Transaction
		ContractName:       primitives.ContractName(tx.ContractName),
		MethodName:         primitives.MethodName(tx.MethodName),
		Timestamp:          primitives.TimestampNano(time.Now().UnixNano()),
		InputArgumentArray: inputArgumentArray.RawArgumentsArray(),
	}

}

func ConvertSendTransactionOutput(sto *client.SendTransactionResponse) *SendTransactionOutput {
	outputArgsIterator := builders.TransactionReceiptOutputArgumentsParse(sto.TransactionReceipt())
	var outputArguments []MethodArgument
	for iter := outputArgsIterator; iter.HasNext(); {
		arg := iter.NextArguments()
		methodArg := convertMethodArgument(arg)
		outputArguments = append(outputArguments, methodArg)
	}

	return &SendTransactionOutput{
		BlockHeight:       sto.BlockHeight(),
		BlockTimestamp:    sto.BlockTimestamp(),
		TransactionStatus: sto.TransactionStatus(),
		TransactionReceipt: TransactionReceipt{
			Txhash:          sto.TransactionReceipt().Txhash(),
			ExecutionResult: sto.TransactionReceipt().ExecutionResult(),
			OutputArguments: outputArguments,
		},
	}
}

func ConvertCallMethodOutput(cmo *client.CallMethodResponse) *CallMethodOutput {
	outputArgsIterator := builders.ClientCallMethodResponseOutputArgumentsParse(cmo)
	var outputArguments []MethodArgument
	for iter := outputArgsIterator; iter.HasNext(); {
		arg := iter.NextArguments()
		methodArg := convertMethodArgument(arg)
		outputArguments = append(outputArguments, methodArg)
	}

	return &CallMethodOutput{
		BlockHeight:     cmo.BlockHeight(),
		BlockTimestamp:  cmo.BlockTimestamp(),
		CallResult:      cmo.CallMethodResult(),
		OutputArguments: outputArguments,
	}
}

func convertMethodArgument(arg *protocol.MethodArgument) MethodArgument {
	methodArg := MethodArgument{
		Name: arg.Name(),
		Type: arg.Type(),
	}
	switch arg.Type() {
	case protocol.METHOD_ARGUMENT_TYPE_UINT_64_VALUE:
		methodArg.Uint64Value = arg.Uint64Value()
	case protocol.METHOD_ARGUMENT_TYPE_UINT_32_VALUE:
		methodArg.Uint32Value = arg.Uint32Value()
	case protocol.METHOD_ARGUMENT_TYPE_STRING_VALUE:
		methodArg.StringValue = arg.StringValue()
	case protocol.METHOD_ARGUMENT_TYPE_BYTES_VALUE:
		methodArg.BytesValue = arg.BytesValue()
	}
	return methodArg
}

func SendTransaction(transferJson *Transaction, keyPair *keys.Ed25519KeyPair, serverUrl string, logVerbose bool) (*SendTransactionOutput, error) {
	tx, err := ConvertAndSignTransaction(transferJson, keyPair)

	if logVerbose {
		log.GetLogger().Info("sending transaction", log.Stringable("transaction", tx.Build()))
	}

	sendTransactionRequest := (&client.SendTransactionRequestBuilder{SignedTransaction: tx}).Build()
	res, err := http.Post(serverUrl+"/api/v1/send-transaction", "application/octet-stream", bytes.NewReader(sendTransactionRequest.Raw()))
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.Errorf("got unexpected http status code %s", res.StatusCode)
	}

	readBytes, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return nil, err
	}

	return ConvertSendTransactionOutput(client.SendTransactionResponseReader(readBytes)), err
}

func CallMethod(transferJson *Transaction, serverUrl string, logVerbose bool) (*CallMethodOutput, error) {
	tx := ConvertTransaction(transferJson)

	if logVerbose {
		log.GetLogger().Info("calling method", log.Stringable("transaction", tx.Build()))
	}

	request := (&client.CallMethodRequestBuilder{Transaction: tx}).Build()
	res, err := http.Post(serverUrl+"/api/v1/call-method", "application/octet-stream", bytes.NewReader(request.Raw()))
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.Errorf("got unexpected http status code %s", res.StatusCode)
	}

	readBytes, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return nil, err
	}

	return ConvertCallMethodOutput(client.CallMethodResponseReader(readBytes)), err
}
