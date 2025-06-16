package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"go-blockchain/pkg/blockchain"
	"go-blockchain/pkg/consensus"
	"go-blockchain/pkg/p2p"
	"go-blockchain/pkg/storage"
)

// Config holds node configuration
type Config struct {
	NodeID      string
	IsLeader    bool
	Port        int
	Peers       []string
	DataDir     string
	LogLevel    string
}

// Node represents a blockchain node
type Node struct {
	config     *Config
	blockchain *blockchain.Blockchain
	storage    *storage.LevelDBStorage
	p2pService *p2p.HTTPP2PService
	consensus  *consensus.ConsensusEngine
	handler    *consensus.ConsensusMessageHandler
}

func main() {
	log.Println("Starting Go-Blockchain Node...")

	// Parse command line flags
	config := parseFlags()

	// Create and start node
	node, err := NewNode(config)
	if err != nil {
		log.Fatalf("Failed to create node: %v", err)
	}

	// Start the node
	if err := node.Start(); err != nil {
		log.Fatalf("Failed to start node: %v", err)
	}

	// Wait for shutdown signal
	node.WaitForShutdown()

	// Cleanup
	if err := node.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Node shutdown complete")
}

// NewNode creates a new blockchain node
func NewNode(config *Config) (*Node, error) {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Initialize storage
	dbPath := filepath.Join(config.DataDir, "blockchain.db")
	storage, err := storage.NewLevelDBStorage(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Load or create blockchain
	bc, err := storage.LoadBlockchain()
	if err != nil {
		log.Printf("Failed to load blockchain from storage, creating new: %v", err)
		bc = blockchain.NewBlockchain()
		
		// Save initial blockchain
		if err := storage.SaveBlockchain(bc); err != nil {
			return nil, fmt.Errorf("failed to save initial blockchain: %w", err)
		}
	}

	// Initialize P2P network configuration
	p2pConfig := &p2p.NetworkConfig{
		NodeID:   config.NodeID,
		Port:     config.Port,
		Peers:    config.Peers,
		MaxPeers: 10,
		Timeout:  10,
	}

	// Initialize consensus configuration  
	consensusConfig := &consensus.ConsensusConfig{
		NodeID:           config.NodeID,
		IsLeader:         config.IsLeader,
		BlockTimeout:     3 * time.Second,
		VotingTimeout:    5 * time.Second,
		MinVotesRequired: 1, // For 3 nodes, need at least 1 vote (2/3 majority would be 2, but for testing)
		MaxBlockSize:     10,
	}

	// Create message handler without consensus engine first
	messageHandler := consensus.NewConsensusMessageHandler(nil, bc)

	// Initialize P2P service
	p2pService := p2p.NewHTTPP2PService(p2pConfig, messageHandler)

	// Now create consensus engine with P2P service
	consensusEngine := consensus.NewConsensusEngine(consensusConfig, bc, storage, p2pService)

	// Update message handler with consensus engine
	messageHandler.SetConsensusEngine(consensusEngine)

	node := &Node{
		config:     config,
		blockchain: bc,
		storage:    storage,
		p2pService: p2pService,
		consensus:  consensusEngine,
		handler:    messageHandler,
	}

	return node, nil
}

// Start starts the blockchain node
func (n *Node) Start() error {
	log.Printf("Starting node %s on port %d", n.config.NodeID, n.config.Port)
	log.Printf("Leader: %t, Peers: %v", n.config.IsLeader, n.config.Peers)
	log.Printf("Data directory: %s", n.config.DataDir)

	// Print blockchain status
	latestBlock := n.blockchain.GetLatestBlock()
	log.Printf("Blockchain initialized - Latest block: %d", latestBlock.Index)
	log.Printf("Total blocks: %d", n.blockchain.GetBlockCount())
	log.Printf("Pending transactions: %d", len(n.blockchain.GetPendingTransactions()))

	// Start P2P service first
	if err := n.p2pService.Start(); err != nil {
		return fmt.Errorf("failed to start P2P service: %w", err)
	}

	// Connect to peers
	if len(n.config.Peers) > 0 {
		log.Printf("Connecting to peers: %v", n.config.Peers)
		if err := n.p2pService.ConnectToPeers(n.config.Peers); err != nil {
			log.Printf("Failed to connect to some peers: %v", err)
		}
		
		// Wait for peer connections
		time.Sleep(3 * time.Second)
		
		// ⭐ PERFORM SYNC ON STARTUP
		if !n.config.IsLeader {
			log.Printf("Performing initial synchronization...")
			if err := n.consensus.SyncWithPeers(); err != nil {
				log.Printf("Initial sync failed: %v", err)
				// Continue anyway - node can still participate
			}
		}
	}

	// Start consensus engine after sync
	if err := n.consensus.Start(); err != nil {
		return fmt.Errorf("failed to start consensus engine: %w", err)
	}

	// Start background tasks (this will be handled by consensus engine now)
	// The old block creation is replaced by consensus mechanism

	log.Printf("Node %s started successfully", n.config.NodeID)
	return nil
}

// Stop stops the blockchain node
func (n *Node) Stop() error {
	log.Printf("Stopping node %s...", n.config.NodeID)

	// Stop consensus engine
	if n.consensus != nil {
		if err := n.consensus.Stop(); err != nil {
			log.Printf("Failed to stop consensus engine: %v", err)
		}
	}

	// Stop P2P service
	if n.p2pService != nil {
		if err := n.p2pService.Stop(); err != nil {
			log.Printf("Failed to stop P2P service: %v", err)
		}
	}

	// Save blockchain state
	if err := n.storage.SaveBlockchain(n.blockchain); err != nil {
		log.Printf("Failed to save blockchain: %v", err)
	}

	// Close storage
	if err := n.storage.Close(); err != nil {
		log.Printf("Failed to close storage: %v", err)
	}

	return nil
}

// WaitForShutdown waits for shutdown signal
func (n *Node) WaitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Node running... Press Ctrl+C to shutdown")
	<-sigChan
	log.Println("Shutdown signal received")
}

// runBlockCreation runs periodic block creation (for leader)
func (n *Node) runBlockCreation() {
	if !n.config.IsLeader {
		return
	}

	ticker := time.NewTicker(10 * time.Second) // Create block every 10 seconds
	defer ticker.Stop()

	for range ticker.C {
		pendingTxs := n.blockchain.GetPendingTransactions()
		if len(pendingTxs) > 0 {
			log.Printf("Creating new block with %d transactions", len(pendingTxs))
			
			block, err := n.blockchain.CreateBlock(10) // Max 10 transactions per block
			if err != nil {
				log.Printf("Failed to create block: %v", err)
				continue
			}

			// Add block to blockchain
			if err := n.blockchain.AddBlock(block); err != nil {
				log.Printf("Failed to add block: %v", err)
				continue
			}

			// Save block to storage
			if err := n.storage.SaveBlock(block); err != nil {
				log.Printf("Failed to save block: %v", err)
			}

			log.Printf("Created block %d with %d transactions", 
				block.Index, len(block.Transactions))
		}
	}
}

// parseFlags parses command line flags and environment variables
func parseFlags() *Config {
	var (
		nodeID   = flag.String("node-id", getEnv("NODE_ID", "node1"), "Node ID")
		isLeader = flag.Bool("leader", getEnvBool("IS_LEADER", false), "Is this node a leader")
		port     = flag.Int("port", getEnvInt("PORT", 50051), "gRPC port")
		peers    = flag.String("peers", getEnv("PEERS", ""), "Comma-separated list of peer addresses")
		dataDir  = flag.String("data-dir", getEnv("DATA_DIR", "./data"), "Data directory")
		logLevel = flag.String("log-level", getEnv("LOG_LEVEL", "info"), "Log level")
	)

	flag.Parse()

	var peersList []string
	if *peers != "" {
		peersList = strings.Split(*peers, ",")
	}

	return &Config{
		NodeID:   *nodeID,
		IsLeader: *isLeader,
		Port:     *port,
		Peers:    peersList,
		DataDir:  *dataDir,
		LogLevel: *logLevel,
	}
}

// Helper functions for environment variables
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return strings.ToLower(value) == "true"
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := parseInt(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func parseInt(s string) (int, error) {
	var result int
	for _, char := range s {
		if char < '0' || char > '9' {
			return 0, fmt.Errorf("invalid integer: %s", s)
		}
		result = result*10 + int(char-'0')
	}
	return result, nil
} 