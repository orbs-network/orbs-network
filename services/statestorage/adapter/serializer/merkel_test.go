package serializer

import (
	"context"
	"fmt"
	"github.com/orbs-network/crypto-lib-go/crypto/hash"
	"github.com/orbs-network/crypto-lib-go/crypto/merkle"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/services/statestorage/adapter"
	"github.com/orbs-network/orbs-network-go/services/statestorage/adapter/memory"
	"github.com/orbs-network/orbs-network-go/test/with"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"testing"
	"time"
)

func TestMerkle(t *testing.T) {
	with.Context(func(ctx context.Context) {
		metricFactory := metric.NewRegistry()
		inmemory := memory.NewStatePersistence(metricFactory)

		timeDeserialize := time.Now()
		dump, err := ioutil.ReadFile("./dump.bin")
		require.NoError(t, err)

		err = NewStatePersistenceDeserializer(inmemory).Deserialize(dump)
		fmt.Println("loading + deserialization time", time.Since(timeDeserialize))
		require.NoError(t, err)

		forest, root := merkle.NewForest()
		fullStateMerkle, err := forest.Update(root, toMerkleInput(inmemory.FullState()))
		require.NoError(t, err)

		_, _, _, _, _, deserializedMerkleRoot, _ := inmemory.ReadMetadata()
		require.EqualValues(t, fullStateMerkle, deserializedMerkleRoot)
	})
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
