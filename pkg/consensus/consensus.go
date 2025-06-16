package consensus

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"go-blockchain/pkg/blockchain"
	"go-blockchain/pkg/p2p"
	"go-blockchain/pkg/storage"
)

// ConsensusEngine implements the consensus mechanism
type ConsensusEngine struct {
	nodeID       string
	isLeader     bool
	blockchain   *blockchain.Blockchain
	storage      *storage.LevelDBStorage
	p2pService   p2p.P2PService
	proposals    map[int64]*ProposalState
	proposalsMux sync.RWMutex
	nextProposalID int64
	config       *ConsensusConfig
}

// ConsensusConfig holds consensus configuration
type ConsensusConfig struct {
	NodeID           string
	IsLeader         bool
	BlockTimeout     time.Duration
	VotingTimeout    time.Duration
	MinVotesRequired int
	MaxBlockSize     int
}

// ProposalState tracks the state of a block proposal
type ProposalState struct {
	ProposalID int64
	Block      *blockchain.Block
	Votes      map[string]bool
	StartTime  time.Time
	Status     ProposalStatus
}

// ProposalStatus represents the status of a proposal
type ProposalStatus int

const (
	ProposalStatusPending ProposalStatus = iota
	ProposalStatusApproved
	ProposalStatusRejected
	ProposalStatusCommitted
)

// NewConsensusEngine creates a new consensus engine
func NewConsensusEngine(config *ConsensusConfig, bc *blockchain.Blockchain, storage *storage.LevelDBStorage, p2pService p2p.P2PService) *ConsensusEngine {
	return &ConsensusEngine{
		nodeID:         config.NodeID,
		isLeader:       config.IsLeader,
		blockchain:     bc,
		storage:        storage,
		p2pService:     p2pService,
		proposals:      make(map[int64]*ProposalState),
		nextProposalID: 1,
		config:         config,
	}
}

// Start starts the consensus engine
func (ce *ConsensusEngine) Start() error {
	log.Printf("Starting consensus engine for node %s (Leader: %t)", ce.nodeID, ce.isLeader)
	
	if ce.isLeader {
		// Start block proposal routine
		go ce.runLeaderConsensus()
	}
	
	// Start proposal cleanup routine
	go ce.cleanupExpiredProposals()
	
	return nil
}

// Stop stops the consensus engine
func (ce *ConsensusEngine) Stop() error {
	log.Printf("Stopping consensus engine for node %s", ce.nodeID)
	return nil
}

// runLeaderConsensus runs the leader consensus routine
func (ce *ConsensusEngine) runLeaderConsensus() {
	ticker := time.NewTicker(ce.config.BlockTimeout)
	defer ticker.Stop()
	
	for range ticker.C {
		if err := ce.proposeNewBlock(); err != nil {
			log.Printf("Failed to propose new block: %v", err)
		}
	}
}

// proposeNewBlock creates and proposes a new block
func (ce *ConsensusEngine) proposeNewBlock() error {
	pendingTxs := ce.blockchain.GetPendingTransactions()
	
	log.Printf("DEBUG: Checking pending transactions, found: %d", len(pendingTxs))
	
	if len(pendingTxs) == 0 {
		log.Printf("No pending transactions, skipping block proposal")
		return nil
	}
	
	// Limit transactions per block
	maxTxs := ce.config.MaxBlockSize
	if len(pendingTxs) > maxTxs {
		pendingTxs = pendingTxs[:maxTxs]
	}
	
	// Create new block
	block, err := ce.blockchain.CreateBlock(maxTxs)
	if err != nil {
		return fmt.Errorf("failed to create block: %w", err)
	}
	
	// ⭐ ADD DEBUG AFTER CreateBlock
	pendingTxsAfterCreate := ce.blockchain.GetPendingTransactions()
	log.Printf("DEBUG: After CreateBlock, pending transactions: %d", len(pendingTxsAfterCreate))
	
	log.Printf("Leader proposing block %d with %d transactions", block.Index, len(block.Transactions))
	
	// Create proposal
	proposalID := ce.getNextProposalID()
	proposal := &ProposalState{
		ProposalID: proposalID,
		Block:      block,
		Votes:      make(map[string]bool),
		StartTime:  time.Now(),
		Status:     ProposalStatusPending,
	}
	
	// Store proposal
	ce.proposalsMux.Lock()
	ce.proposals[proposalID] = proposal
	ce.proposalsMux.Unlock()
	
	// Send proposal to all peers
	ctx, cancel := context.WithTimeout(context.Background(), ce.config.VotingTimeout)
	defer cancel()
	
	votes, err := ce.p2pService.ProposeBlock(ctx, block, proposalID)
	if err != nil {
		log.Printf("Failed to send block proposal: %v", err)
		return err
	}
	
	// Count votes
	acceptedVotes := 0
	totalVotes := len(votes)
	
	for peerAddr, vote := range votes {
		proposal.Votes[peerAddr] = vote
		if vote {
			acceptedVotes++
		}
	}
	
	log.Printf("Received %d votes, %d accepted out of %d total", totalVotes, acceptedVotes, totalVotes)
	
	// Check if we have majority
	if acceptedVotes >= ce.config.MinVotesRequired {
		log.Printf("Block %d approved by majority (%d/%d votes)", block.Index, acceptedVotes, totalVotes)
		return ce.commitBlock(proposal)
	} else {
		log.Printf("Block %d rejected by majority (%d/%d votes)", block.Index, acceptedVotes, totalVotes)
		proposal.Status = ProposalStatusRejected
		
		// ⭐ ADD DEBUG: Check pending after rejection
		pendingTxsAfterReject := ce.blockchain.GetPendingTransactions()
		log.Printf("DEBUG: After rejection, pending transactions: %d", len(pendingTxsAfterReject))
		
		return nil
	}
}

// commitBlock commits an approved block
func (ce *ConsensusEngine) commitBlock(proposal *ProposalState) error {
	block := proposal.Block
	
	// Add block to blockchain
	if err := ce.blockchain.AddBlock(block); err != nil {
		return fmt.Errorf("failed to add block to blockchain: %w", err)
	}
	
	// Save block to storage
	if err := ce.storage.SaveBlock(block); err != nil {
		return fmt.Errorf("failed to save block to storage: %w", err)
	}
	
	// Notify all peers about the committed block
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := ce.p2pService.CommitBlock(ctx, block, proposal.ProposalID); err != nil {
		log.Printf("Failed to notify peers about committed block: %v", err)
	}
	
	proposal.Status = ProposalStatusCommitted
	log.Printf("Block %d committed successfully", block.Index)
	
	return nil
}

// getNextProposalID returns the next proposal ID
func (ce *ConsensusEngine) getNextProposalID() int64 {
	ce.nextProposalID++
	return ce.nextProposalID
}

// cleanupExpiredProposals removes expired proposals
func (ce *ConsensusEngine) cleanupExpiredProposals() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		ce.proposalsMux.Lock()
		now := time.Now()
		for id, proposal := range ce.proposals {
			if now.Sub(proposal.StartTime) > ce.config.VotingTimeout*2 {
				delete(ce.proposals, id)
			}
		}
		ce.proposalsMux.Unlock()
	}
}

// HandleBlockProposal handles incoming block proposals (for followers)
func (ce *ConsensusEngine) HandleBlockProposal(proposal *p2p.BlockProposalMessage) (bool, error) {
	if ce.isLeader {
		log.Printf("Received block proposal but I'm the leader, ignoring")
		return false, nil
	}
	
	log.Printf("Received block proposal %d from %s", proposal.ProposalID, proposal.ProposerID)
	
	// Validate the proposed block
	latestBlock := ce.blockchain.GetLatestBlock()
	if err := proposal.Block.ValidateBlock(latestBlock); err != nil {
		log.Printf("Block validation failed: %v", err)
		return false, nil
	}
	
	// Additional validation: check if all transactions are valid
	for i, tx := range proposal.Block.Transactions {
		if !tx.IsValid() {
			log.Printf("Invalid transaction at index %d", i)
			return false, nil
		}
	}
	
	log.Printf("Block proposal %d is valid, voting to accept", proposal.ProposalID)
	return true, nil
}

// HandleBlockCommit handles block commit messages
func (ce *ConsensusEngine) HandleBlockCommit(commit *p2p.BlockCommitMessage) error {
	if ce.isLeader {
		// Leaders don't need to handle commits from others
		return nil
	}
	
	log.Printf("Received block commit for proposal %d", commit.ProposalID)
	
	// Validate and add the block
	latestBlock := ce.blockchain.GetLatestBlock()
	if err := commit.Block.ValidateBlock(latestBlock); err != nil {
		return fmt.Errorf("invalid committed block: %w", err)
	}
	
	// Add block to blockchain
	if err := ce.blockchain.AddBlock(commit.Block); err != nil {
		return fmt.Errorf("failed to add committed block: %w", err)
	}
	
	// Save block to storage
	if err := ce.storage.SaveBlock(commit.Block); err != nil {
		return fmt.Errorf("failed to save committed block: %w", err)
	}
	
	log.Printf("Block %d committed successfully", commit.Block.Index)
	return nil
}

// HandleVote handles vote messages
func (ce *ConsensusEngine) HandleVote(vote *p2p.VoteMessage) error {
	if !ce.isLeader {
		return nil // Only leaders handle votes
	}
	
	ce.proposalsMux.Lock()
	defer ce.proposalsMux.Unlock()
	
	proposal, exists := ce.proposals[vote.ProposalID]
	if !exists {
		return fmt.Errorf("proposal %d not found", vote.ProposalID)
	}
	
	proposal.Votes[vote.VoterID] = vote.Accept
	log.Printf("Received vote from %s for proposal %d: %t", vote.VoterID, vote.ProposalID, vote.Accept)
	
	return nil
}

// HandleTransaction handles incoming transactions
func (ce *ConsensusEngine) HandleTransaction(tx *blockchain.Transaction) error {
	log.Printf("Received transaction: %s", tx.String())
	
	// Validate transaction
	if !tx.IsValid() {
		return fmt.Errorf("invalid transaction")
	}
	
	// Add to blockchain's pending transactions
	if err := ce.blockchain.AddTransaction(tx); err != nil {
		return fmt.Errorf("failed to add transaction: %w", err)
	}
	
	log.Printf("Transaction added to pending pool")
	
	// ⭐ ADD DEBUG: Check immediately after adding
	pendingTxs := ce.blockchain.GetPendingTransactions()
	log.Printf("DEBUG: After adding transaction, pending pool size: %d", len(pendingTxs))
	
	return nil
}

// GetProposalStatus returns the status of a proposal
func (ce *ConsensusEngine) GetProposalStatus(proposalID int64) (*ProposalState, bool) {
	ce.proposalsMux.RLock()
	defer ce.proposalsMux.RUnlock()
	
	proposal, exists := ce.proposals[proposalID]
	return proposal, exists
}

// GetActiveProposals returns all active proposals
func (ce *ConsensusEngine) GetActiveProposals() []*ProposalState {
	ce.proposalsMux.RLock()
	defer ce.proposalsMux.RUnlock()
	
	var proposals []*ProposalState
	for _, proposal := range ce.proposals {
		if proposal.Status == ProposalStatusPending {
			proposals = append(proposals, proposal)
		}
	}
	
	return proposals
}

// SyncWithPeers synchronizes missing blocks with peers
func (ce *ConsensusEngine) SyncWithPeers() error {
	log.Printf("Starting synchronization with peers...")
	
	// Get current blockchain height
	currentHeight := ce.blockchain.GetBlockCount() - 1
	log.Printf("Current blockchain height: %d", currentHeight)
	
	// Get maximum height from peers
	maxHeight := ce.getMaxHeightFromPeers()
	if maxHeight <= currentHeight {
		log.Printf("Blockchain is up to date (height %d)", currentHeight)
		return nil
	}
	
	log.Printf("Synchronizing from height %d to %d", currentHeight+1, maxHeight)
	
	// Fetch missing blocks in batches
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	for height := currentHeight + 1; height <= maxHeight; height += 10 {
		limit := int(min(10, maxHeight-height+1))
		
		blocks, err := ce.fetchBlocksFromAnyPeer(ctx, height, limit)
		if err != nil {
			log.Printf("Failed to fetch blocks from height %d: %v", height, err)
			continue
		}
		
		// Add blocks sequentially
		for _, block := range blocks {
			if err := ce.addSyncedBlock(block); err != nil {
				log.Printf("Failed to add synced block %d: %v", block.Index, err)
				return err
			}
			log.Printf("Synced block %d", block.Index)
		}
	}
	
	log.Printf("Synchronization completed successfully")
	return nil
}

// getMaxHeightFromPeers gets max height from all peers
func (ce *ConsensusEngine) getMaxHeightFromPeers() int64 {
	maxHeight := int64(0)
	
	// Simple HTTP requests to peer blockchain endpoints
	peers := []string{"node1:50051", "node2:50051", "node3:50051"}
	
	for _, peer := range peers {
		if peer == fmt.Sprintf("%s:50051", ce.nodeID) {
			continue // Skip self
		}
		
		height := ce.getPeerHeight(peer)
		if height > maxHeight {
			maxHeight = height
		}
	}
	
	return maxHeight
}

// getPeerHeight gets blockchain height from peer
func (ce *ConsensusEngine) getPeerHeight(peerAddr string) int64 {
	url := fmt.Sprintf("http://%s/blockchain", peerAddr)
	
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	
	var response struct {
		Total int `json:"total"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return 0
	}
	
	return int64(response.Total - 1) // Convert to height
}

// fetchBlocksFromAnyPeer fetches blocks from any available peer
func (ce *ConsensusEngine) fetchBlocksFromAnyPeer(ctx context.Context, startIndex int64, limit int) ([]*blockchain.Block, error) {
	peers := []string{"node1:50051", "node2:50051", "node3:50051"}
	
	for _, peer := range peers {
		if peer == fmt.Sprintf("%s:50051", ce.nodeID) {
			continue // Skip self
		}
		
		blocks, err := ce.fetchBlocksFromPeer(peer, startIndex, limit)
		if err == nil && len(blocks) > 0 {
			return blocks, nil
		}
	}
	
	return nil, fmt.Errorf("no peers available")
}

// fetchBlocksFromPeer fetches blocks from specific peer
func (ce *ConsensusEngine) fetchBlocksFromPeer(peerAddr string, startIndex int64, limit int) ([]*blockchain.Block, error) {
	url := fmt.Sprintf("http://%s/blocks/sync?start=%d&limit=%d", peerAddr, startIndex, limit)
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var response p2p.SyncResponseMessage
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	
	return response.Blocks, nil
}

// addSyncedBlock adds a synced block to blockchain
func (ce *ConsensusEngine) addSyncedBlock(block *blockchain.Block) error {
	// Basic validation
	if block.Index > 0 {
		prevBlock := ce.blockchain.GetBlock(block.Index - 1)
		if prevBlock == nil {
			return fmt.Errorf("previous block not found")
		}
		
		if err := block.ValidateBlock(prevBlock); err != nil {
			return fmt.Errorf("block validation failed: %w", err)
		}
	}
	
	// Add to blockchain
	if err := ce.blockchain.AddBlock(block); err != nil {
		return fmt.Errorf("failed to add block: %w", err)
	}
	
	// Save to storage
	if err := ce.storage.SaveBlock(block); err != nil {
		return fmt.Errorf("failed to save block: %w", err)
	}
	
	return nil
}

// min helper function
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
} 