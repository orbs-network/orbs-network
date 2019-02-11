package config

import (
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol/consensus"
	"path/filepath"
	"time"
)

// all other configs are variations from the production one
func defaultProductionConfig() mutableNodeConfig {
	cfg := emptyConfig()

	cfg.SetUint32(PROTOCOL_VERSION, 1)
	cfg.SetUint32(VIRTUAL_CHAIN_ID, 42)
	cfg.SetUint32(GOSSIP_LISTEN_PORT, 4400)
	cfg.SetDuration(BENCHMARK_CONSENSUS_RETRY_INTERVAL, 2*time.Second)
	cfg.SetDuration(LEAN_HELIX_CONSENSUS_ROUND_TIMEOUT_INTERVAL, 4*time.Second)
	cfg.SetBool(LEAN_HELIX_SHOW_DEBUG, false)
	cfg.SetDuration(TRANSACTION_POOL_TIME_BETWEEN_EMPTY_BLOCKS, 1*time.Second)
	cfg.SetUint32(BENCHMARK_CONSENSUS_REQUIRED_QUORUM_PERCENTAGE, 66)
	cfg.SetUint32(CONSENSUS_CONTEXT_MAXIMUM_TRANSACTIONS_IN_BLOCK, 100)
	cfg.SetDuration(CONSENSUS_CONTEXT_SYSTEM_TIMESTAMP_ALLOWED_JITTER, 2*time.Second)
	cfg.SetUint32(LEAN_HELIX_CONSENSUS_MINIMUM_COMMITTEE_SIZE, 4)
	cfg.SetUint32(BLOCK_TRACKER_GRACE_DISTANCE, 3)
	cfg.SetDuration(BLOCK_TRACKER_GRACE_TIMEOUT, 100*time.Millisecond)
	cfg.SetUint32(BLOCK_SYNC_NUM_BLOCKS_IN_BATCH, 10000)
	cfg.SetDuration(BLOCK_SYNC_NO_COMMIT_INTERVAL, 8*time.Second)
	cfg.SetDuration(BLOCK_SYNC_COLLECT_RESPONSE_TIMEOUT, 3*time.Second)
	cfg.SetDuration(BLOCK_SYNC_COLLECT_CHUNKS_TIMEOUT, 5*time.Second)
	cfg.SetDuration(PUBLIC_API_SEND_TRANSACTION_TIMEOUT, 30*time.Second)
	cfg.SetDuration(PUBLIC_API_NODE_SYNC_WARNING_TIME, 300*time.Second)
	cfg.SetDuration(BLOCK_STORAGE_TRANSACTION_RECEIPT_QUERY_TIMESTAMP_GRACE, 5*time.Second)
	cfg.SetUint32(STATE_STORAGE_HISTORY_SNAPSHOT_NUM, 5)
	cfg.SetUint32(TRANSACTION_POOL_PENDING_POOL_SIZE_IN_BYTES, 20*1024*1024)
	cfg.SetDuration(TRANSACTION_EXPIRATION_WINDOW, 30*time.Minute)
	cfg.SetDuration(TRANSACTION_POOL_NODE_SYNC_REJECT_TIME, 2*time.Minute)
	cfg.SetDuration(TRANSACTION_POOL_FUTURE_TIMESTAMP_GRACE_TIMEOUT, 1*time.Minute)
	cfg.SetDuration(TRANSACTION_POOL_PENDING_POOL_CLEAR_EXPIRED_INTERVAL, 10*time.Second)
	cfg.SetDuration(TRANSACTION_POOL_COMMITTED_POOL_CLEAR_EXPIRED_INTERVAL, 30*time.Second)
	cfg.SetUint32(TRANSACTION_POOL_PROPAGATION_BATCH_SIZE, 100)
	cfg.SetDuration(TRANSACTION_POOL_PROPAGATION_BATCHING_TIMEOUT, 100*time.Millisecond)
	cfg.SetDuration(GOSSIP_CONNECTION_KEEP_ALIVE_INTERVAL, 1*time.Second)
	cfg.SetDuration(GOSSIP_NETWORK_TIMEOUT, 30*time.Second)
	cfg.SetDuration(METRICS_REPORT_INTERVAL, 30*time.Second)

	cfg.SetActiveConsensusAlgo(consensus.CONSENSUS_ALGO_TYPE_BENCHMARK_CONSENSUS)
	cfg.SetString(ETHEREUM_ENDPOINT, "http://localhost:8545")
	cfg.SetString(PROCESSOR_ARTIFACT_PATH, filepath.Join(GetProjectSourceTmpPath(), "processor-artifacts"))
	cfg.SetString(BLOCK_STORAGE_FILE_SYSTEM_DATA_DIR, "/usr/local/var/orbs") // TODO V1 use build tags to replace with /var/lib/orbs for linux
	cfg.SetUint32(BLOCK_STORAGE_FILE_SYSTEM_MAX_BLOCK_SIZE_IN_BYTES, 64*1024*1024)

	cfg.SetDuration(LOGGER_FILE_TRUNCATION_INTERVAL, 24*time.Hour)

	cfg.SetBool(PROFILING, false)
	cfg.SetString(HTTP_ADDRESS, ":8080")

	return cfg
}

// config for a production node (either main net or test net)
func ForProduction(processorArtifactPath string) mutableNodeConfig {
	cfg := defaultProductionConfig()

	if processorArtifactPath != "" {
		cfg.SetString(PROCESSOR_ARTIFACT_PATH, processorArtifactPath)
	}
	return cfg
}

// config for end-to-end tests (very similar to production but slightly faster)
func ForE2E(
	processorArtifactPath string,
	federationNodes map[string]FederationNode,
	gossipPeers map[string]GossipPeer,
	constantConsensusLeader primitives.NodeAddress,
	activeConsensusAlgo consensus.ConsensusAlgoType,
	ethereumEndpoint string,
) mutableNodeConfig {
	cfg := defaultProductionConfig()

	cfg.SetDuration(BENCHMARK_CONSENSUS_RETRY_INTERVAL, 250*time.Millisecond)
	cfg.SetDuration(LEAN_HELIX_CONSENSUS_ROUND_TIMEOUT_INTERVAL, 500*time.Millisecond)
	cfg.SetBool(LEAN_HELIX_SHOW_DEBUG, false)
	cfg.SetDuration(TRANSACTION_POOL_TIME_BETWEEN_EMPTY_BLOCKS, 100*time.Millisecond) // this is the time between empty blocks when no transactions, need to be large so we don't close infinite blocks on idle
	cfg.SetDuration(PUBLIC_API_SEND_TRANSACTION_TIMEOUT, 10*time.Second)
	cfg.SetDuration(PUBLIC_API_NODE_SYNC_WARNING_TIME, 100*time.Second)
	cfg.SetUint32(CONSENSUS_CONTEXT_MAXIMUM_TRANSACTIONS_IN_BLOCK, 100)
	cfg.SetUint32(TRANSACTION_POOL_PROPAGATION_BATCH_SIZE, 100)
	cfg.SetDuration(TRANSACTION_POOL_PROPAGATION_BATCHING_TIMEOUT, 50*time.Millisecond)
	cfg.SetDuration(BLOCK_SYNC_NO_COMMIT_INTERVAL, 1000*time.Millisecond)
	cfg.SetDuration(GOSSIP_CONNECTION_KEEP_ALIVE_INTERVAL, 500*time.Millisecond)
	cfg.SetDuration(GOSSIP_NETWORK_TIMEOUT, 2*time.Second)

	cfg.SetString(ETHEREUM_ENDPOINT, ethereumEndpoint)
	cfg.SetUint32(BLOCK_STORAGE_FILE_SYSTEM_MAX_BLOCK_SIZE_IN_BYTES, 64*1024*1024)

	cfg.SetGossipPeers(gossipPeers)
	cfg.SetFederationNodes(federationNodes)
	cfg.SetActiveConsensusAlgo(activeConsensusAlgo)
	cfg.SetBenchmarkConsensusConstantLeader(constantConsensusLeader)
	if processorArtifactPath != "" {
		cfg.SetString(PROCESSOR_ARTIFACT_PATH, processorArtifactPath)
	}
	return cfg
}

func ForAcceptanceTestNetwork(
	federationNodes map[string]FederationNode,
	constantConsensusLeader primitives.NodeAddress,
	activeConsensusAlgo consensus.ConsensusAlgoType,
	maxTxPerBlock uint32,
	requiredQuorumPercentage uint32,
) mutableNodeConfig {
	cfg := defaultProductionConfig()

	cfg.SetDuration(BENCHMARK_CONSENSUS_RETRY_INTERVAL, 1*time.Millisecond)
	// TODO v1 How to express relations between config properties https://tree.taiga.io/project/orbs-network/us/647
	// LEAN_HELIX_CONSENSUS_ROUND_TIMEOUT_INTERVAL should be less than BLOCK_SYNC_NO_COMMIT_INTERVAL, or else node-sync will be triggered unnecessarily
	cfg.SetDuration(LEAN_HELIX_CONSENSUS_ROUND_TIMEOUT_INTERVAL, 100*time.Millisecond)
	cfg.SetBool(LEAN_HELIX_SHOW_DEBUG, true)
	cfg.SetDuration(TRANSACTION_POOL_TIME_BETWEEN_EMPTY_BLOCKS, 10*time.Millisecond)
	cfg.SetUint32(BENCHMARK_CONSENSUS_REQUIRED_QUORUM_PERCENTAGE, requiredQuorumPercentage)
	cfg.SetDuration(BLOCK_TRACKER_GRACE_TIMEOUT, 300*time.Millisecond)
	cfg.SetDuration(PUBLIC_API_SEND_TRANSACTION_TIMEOUT, 1000*time.Millisecond) // keep this high to reduce chance of test failure due to slow test machine
	cfg.SetDuration(PUBLIC_API_NODE_SYNC_WARNING_TIME, 3000*time.Millisecond)
	cfg.SetUint32(CONSENSUS_CONTEXT_MAXIMUM_TRANSACTIONS_IN_BLOCK, maxTxPerBlock)
	cfg.SetUint32(TRANSACTION_POOL_PROPAGATION_BATCH_SIZE, 5)
	cfg.SetDuration(TRANSACTION_POOL_PROPAGATION_BATCHING_TIMEOUT, 3*time.Millisecond)
	cfg.SetUint32(BLOCK_SYNC_NUM_BLOCKS_IN_BATCH, 5)
	cfg.SetDuration(BLOCK_SYNC_NO_COMMIT_INTERVAL, 200*time.Millisecond) // should be a factor more than average block time
	cfg.SetDuration(BLOCK_SYNC_COLLECT_RESPONSE_TIMEOUT, 15*time.Millisecond)
	cfg.SetDuration(BLOCK_SYNC_COLLECT_CHUNKS_TIMEOUT, 15*time.Millisecond)

	cfg.SetFederationNodes(federationNodes)
	cfg.SetBenchmarkConsensusConstantLeader(constantConsensusLeader)
	cfg.SetActiveConsensusAlgo(activeConsensusAlgo)
	return cfg
}

// config for gamma dev network that runs with in-memory adapters except for contract compilation
func TemplateForGamma(
	federationNodes map[string]FederationNode,
	constantConsensusLeader primitives.NodeAddress,
	activeConsensusAlgo consensus.ConsensusAlgoType,
) mutableNodeConfig {
	cfg := defaultProductionConfig()

	cfg.SetDuration(BENCHMARK_CONSENSUS_RETRY_INTERVAL, 1000*time.Millisecond)
	cfg.SetDuration(LEAN_HELIX_CONSENSUS_ROUND_TIMEOUT_INTERVAL, 1*time.Second)
	cfg.SetBool(LEAN_HELIX_SHOW_DEBUG, false)
	cfg.SetDuration(TRANSACTION_POOL_TIME_BETWEEN_EMPTY_BLOCKS, 500*time.Millisecond) // this is the time between empty blocks when no transactions, need to be large so we don't close infinite blocks on idle
	cfg.SetUint32(BENCHMARK_CONSENSUS_REQUIRED_QUORUM_PERCENTAGE, 100)
	cfg.SetDuration(BLOCK_TRACKER_GRACE_TIMEOUT, 100*time.Millisecond)
	cfg.SetDuration(PUBLIC_API_SEND_TRANSACTION_TIMEOUT, 10*time.Second)
	cfg.SetDuration(PUBLIC_API_NODE_SYNC_WARNING_TIME, 100*time.Second)
	cfg.SetUint32(CONSENSUS_CONTEXT_MAXIMUM_TRANSACTIONS_IN_BLOCK, 100)
	cfg.SetUint32(TRANSACTION_POOL_PROPAGATION_BATCH_SIZE, 5)
	cfg.SetDuration(TRANSACTION_POOL_PROPAGATION_BATCHING_TIMEOUT, 10*time.Millisecond)
	cfg.SetUint32(BLOCK_SYNC_NUM_BLOCKS_IN_BATCH, 5)
	cfg.SetDuration(BLOCK_SYNC_NO_COMMIT_INTERVAL, 2500*time.Millisecond)
	cfg.SetDuration(BLOCK_SYNC_COLLECT_RESPONSE_TIMEOUT, 15*time.Millisecond)
	cfg.SetDuration(BLOCK_SYNC_COLLECT_CHUNKS_TIMEOUT, 15*time.Millisecond)

	cfg.SetUint32(BLOCK_STORAGE_FILE_SYSTEM_MAX_BLOCK_SIZE_IN_BYTES, 64*1024*1024)
	cfg.SetString(ETHEREUM_ENDPOINT, "http://host.docker.internal:7545")

	cfg.SetFederationNodes(federationNodes)
	cfg.SetBenchmarkConsensusConstantLeader(constantConsensusLeader)
	cfg.SetActiveConsensusAlgo(activeConsensusAlgo)
	return cfg
}
