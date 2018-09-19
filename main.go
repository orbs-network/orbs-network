package main

import (
	"encoding/hex"
	"encoding/json"
	"github.com/orbs-network/orbs-network-go/bootstrap"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/instrumentation/log"
	"github.com/orbs-network/orbs-spec/types/go/protocol/consensus"
	"io"
	"io/ioutil"
	"os"
	"strconv"
)

func getLogger(path string, silent bool) log.BasicLogger {
	if path == "" {
		path = "./orbs-network.log"
	}

	logFile, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}

	var stdout io.Writer
	stdout = os.Stdout

	if silent {
		stdout = ioutil.Discard
	}

	stdoutOutput := log.NewOutput(stdout).WithFormatter(log.NewHumanReadableFormatter())
	fileOutput := log.NewOutput(logFile).WithFormatter(log.NewHumanReadableFormatter()) // until system stabilizes we temporarily log files with human readable to assist debugging

	return log.GetLogger().WithOutput(stdoutOutput, fileOutput)
}

type peer struct {
	Key        string
	IP         string
	GossipPort uint16
}

func getPeers(logger log.BasicLogger, input string) (map[string]config.FederationNode, map[string]config.GossipPeer) {
	federationNodes := make(map[string]config.FederationNode)
	gossipPeers := make(map[string]config.GossipPeer)

	if input == "" {
		return federationNodes, gossipPeers
	}

	var peers []peer

	err := json.Unmarshal([]byte(input), &peers)
	if err != nil {
		logger.Error("Failed to parse peers configuration", log.Error(err))
		return federationNodes, gossipPeers
	}

	for _, peer := range peers {
		publicKey, _ := hex.DecodeString(peer.Key)
		federationNodes[string(publicKey)] = config.NewHardCodedFederationNode(publicKey)
		gossipPeers[string(publicKey)] = config.NewHardCodedGossipPeer(peer.GossipPort, peer.IP)
	}

	return federationNodes, gossipPeers
}

func main() {
	// TODO: change this to a config like HardCodedConfig that takes config from env or json
	httpPort, _ := strconv.ParseInt(os.Getenv("HTTP_PORT"), 10, 0)
	gossipPort, _ := strconv.ParseInt(os.Getenv("GOSSIP_PORT"), 10, 0)
	nodePublicKey, _ := hex.DecodeString(os.Getenv("NODE_PUBLIC_KEY"))
	nodePrivateKey, _ := hex.DecodeString(os.Getenv("NODE_PRIVATE_KEY"))
	peers := os.Getenv("PEERS") // TODO - maybe split this into 2 env vars (FEDERATION_NODES, GOSSIP_PEERS)
	consensusLeader, _ := hex.DecodeString(os.Getenv("CONSENSUS_LEADER"))
	httpAddress := ":" + strconv.FormatInt(httpPort, 10)
	logPath := os.Getenv("LOG_PATH")
	silentLog := os.Getenv("SILENT") == "true"

	logger := getLogger(logPath, silentLog)

	// TODO: move this code to the config we decided to add, the HardCodedConfig stuff is just placeholder

	federationNodes, gossipPeers := getPeers(logger, peers)

	bootstrap.NewNode(
		httpAddress,
		nodePublicKey,
		nodePrivateKey,
		federationNodes,
		gossipPeers,
		uint16(gossipPort),
		consensusLeader,
		consensus.CONSENSUS_ALGO_TYPE_BENCHMARK_CONSENSUS,
		logger,
	).WaitUntilShutdown()
}
