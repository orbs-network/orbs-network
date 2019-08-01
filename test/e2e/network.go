// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package e2e

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/orbs-network/orbs-network-go/bootstrap"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/config/paths"
	"github.com/orbs-network/orbs-network-go/test"
	"github.com/orbs-network/orbs-network-go/test/crypto/keys"
	"github.com/orbs-network/orbs-spec/types/go/protocol/consensus"
	"github.com/orbs-network/scribe/log"
	"os"
	"time"
)

var OwnerOfAllSupply = keys.Ed25519KeyPairForTests(5) // needs to be a constant across all e2e tests since we deploy the contract only once

// LOCAL_NETWORK_SIZE must remain identical to number of configured nodes in docker/test/benchmark-config
// Also Lean Helix consensus algo requires it to be >= 4 or it will panic
const LOCAL_NETWORK_SIZE = 4

type inProcessE2ENetwork struct {
	nodes []bootstrap.Node
}

func NewInProcessE2ENetwork() *inProcessE2ENetwork {
	cleanNativeProcessorCache()
	cleanBlockStorage()

	return &inProcessE2ENetwork{bootstrapE2ENetwork()}
}

func (h *inProcessE2ENetwork) GracefulShutdownAndWipeDisk() {
	for _, node := range h.nodes {
		node.GracefulShutdown(0) // meaning don't have a deadline timeout so allowing enough time for shutdown to free port
	}

	cleanNativeProcessorCache()
	cleanBlockStorage()
}

type staticConfigLoader struct {
	staticConfig *config.MapBasedConfig
}

func (s *staticConfigLoader) Load() (*config.MapBasedConfig, error) {
	return s.staticConfig, nil
}

func (s *staticConfigLoader) OnConfigChanged(handler config.ChangeHandler) {

}

func (s *staticConfigLoader) ListenForChanges(ctx context.Context, logger log.Logger, pollInterval time.Duration, onShutdown func()) {

}

func bootstrapE2ENetwork() (nodes []bootstrap.Node) {
	gossipPortByNodeIndex := []int{}
	genesisValidatorNodes := make(map[string]config.ValidatorNode)
	gossipPeers := make(map[string]config.GossipPeer)

	for i := 0; i < LOCAL_NETWORK_SIZE; i++ {
		gossipPortByNodeIndex = append(gossipPortByNodeIndex, test.RandomPort())
		nodeAddress := keys.EcdsaSecp256K1KeyPairForTests(i).NodeAddress()
		genesisValidatorNodes[nodeAddress.KeyForMap()] = config.NewHardCodedValidatorNode(nodeAddress)
		gossipPeers[nodeAddress.KeyForMap()] = config.NewHardCodedGossipPeer(gossipPortByNodeIndex[i], "127.0.0.1", hex.EncodeToString(nodeAddress))
	}

	ethereumEndpoint := os.Getenv("ETHEREUM_ENDPOINT") //TODO v1 unite how this config is fetched

	_ = os.MkdirAll(paths.GetProjectSourceRootPath()+"/_logs", 0755)
	console := log.NewFormattingOutput(os.Stdout, log.NewHumanReadableFormatter())

	logger := log.GetLogger().WithTags(
		log.String("_test", "e2e"),
		log.String("_branch", os.Getenv("GIT_BRANCH")),
		log.String("_commit", os.Getenv("GIT_COMMIT"))).
		WithOutput(console)
	leaderKeyPair := keys.EcdsaSecp256K1KeyPairForTests(0)
	for i := 0; i < LOCAL_NETWORK_SIZE; i++ {
		nodeKeyPair := keys.EcdsaSecp256K1KeyPairForTests(i)

		logFile, err := os.OpenFile(
			fmt.Sprintf("%s/_logs/node%d-%v.log", paths.GetProjectSourceRootPath(), i+1, time.Now().Format(time.RFC3339Nano)),
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0644)
		if err != nil {
			panic(err)
		}

		nodeLogger := logger.WithOutput(console, log.NewFormattingOutput(logFile, log.NewJsonFormatter()))
		processorArtifactPath, _ := getProcessorArtifactPath()

		cfg := config.
			ForE2E(
				fmt.Sprintf(":%d", START_HTTP_PORT+i),
				gossipPortByNodeIndex[i],
				nodeKeyPair.NodeAddress(),
				nodeKeyPair.PrivateKey(),
				gossipPeers,
				genesisValidatorNodes,
				blockStorageDataDirPrefix,
				processorArtifactPath,
				ethereumEndpoint,
				leaderKeyPair.NodeAddress(),
				consensus.CONSENSUS_ALGO_TYPE_BENCHMARK_CONSENSUS,
			)
		deployBlockStorageFiles(cfg.BlockStorageFileSystemDataDir(), logger)

		node := bootstrap.NewNode(&staticConfigLoader{cfg}, nodeLogger)

		nodes = append(nodes, node)
	}
	return nodes
}
