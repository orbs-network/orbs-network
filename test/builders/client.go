package builders

import (
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/protocol/client"
)

func ClientCallMethodResponseOutputArgumentsParse(r *client.CallMethodResponse) *protocol.MethodArgumentArrayArgumentsIterator {
	argsArray := protocol.MethodArgumentArrayReader(r.RawOutputArgumentArrayWithHeader())
	return argsArray.ArgumentsIterator()
}

func ClientCallMethodResponseOutputArgumentsPrint(r *client.CallMethodResponse) string {
	argsArray := protocol.MethodArgumentArrayReader(r.RawOutputArgumentArrayWithHeader())
	return argsArray.String()
}
