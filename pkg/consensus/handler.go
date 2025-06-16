package consensus

import (
	"fmt"

	"go-blockchain/pkg/blockchain"
	"go-blockchain/pkg/p2p"
)

// ConsensusMessageHandler implements MessageHandler interface
type ConsensusMessageHandler struct {
	consensus  *ConsensusEngine
	blockchain *blockchain.Blockchain
}

// NewConsensusMessageHandler creates a new message handler
func NewConsensusMessageHandler(consensus *ConsensusEngine, blockchain *blockchain.Blockchain) *ConsensusMessageHandler {
	return &ConsensusMessageHandler{
		consensus:  consensus,
		blockchain: blockchain,
	}
}

// SetConsensusEngine updates the consensus engine (for resolving circular dependencies)
func (h *ConsensusMessageHandler) SetConsensusEngine(consensus *ConsensusEngine) {
	h.consensus = consensus
}

// HandleTransaction handles incoming transaction messages
func (h *ConsensusMessageHandler) HandleTransaction(tx *blockchain.Transaction) error {
	if h.consensus == nil {
		// If consensus is not yet initialized, just add to blockchain directly
		return h.blockchain.AddTransaction(tx)
	}
	return h.consensus.HandleTransaction(tx)
}

// HandleBlockProposal handles incoming block proposal messages
func (h *ConsensusMessageHandler) HandleBlockProposal(proposal *p2p.BlockProposalMessage) (bool, error) {
	if h.consensus == nil {
		return false, fmt.Errorf("consensus engine not initialized")
	}
	return h.consensus.HandleBlockProposal(proposal)
}

// HandleVote handles incoming vote messages
func (h *ConsensusMessageHandler) HandleVote(vote *p2p.VoteMessage) error {
	if h.consensus == nil {
		return fmt.Errorf("consensus engine not initialized")
	}
	return h.consensus.HandleVote(vote)
}

// HandleBlockCommit handles incoming block commit messages
func (h *ConsensusMessageHandler) HandleBlockCommit(commit *p2p.BlockCommitMessage) error {
	if h.consensus == nil {
		return fmt.Errorf("consensus engine not initialized")
	}
	return h.consensus.HandleBlockCommit(commit)
}

// HandleSyncRequest handles requests for blocks (for node synchronization)
func (h *ConsensusMessageHandler) HandleSyncRequest(request *p2p.SyncRequestMessage) (*p2p.SyncResponseMessage, error) {
	var blocks []*blockchain.Block
	
	// Get blocks starting from requested index
	currentIndex := request.StartIndex
	limit := request.Limit
	if limit <= 0 || limit > 100 {
		limit = 10 // Default/maximum limit
	}
	
	for i := 0; i < int(limit); i++ {
		block := h.blockchain.GetBlock(currentIndex + int64(i))
		if block == nil {
			break // No more blocks available
		}
		blocks = append(blocks, block)
	}
	
	return &p2p.SyncResponseMessage{
		Blocks: blocks,
	}, nil
}

// GetNodeStatus returns the current status of the node
func (h *ConsensusMessageHandler) GetNodeStatus() *p2p.NodeStatus {
	latestBlock := h.blockchain.GetLatestBlock()
	pendingTxs := h.blockchain.GetPendingTransactions()
	
	connectedPeers := 0
	
	status := "running"
	if h.consensus.isLeader {
		status = "leader"
	} else {
		status = "follower"
	}
	
	return &p2p.NodeStatus{
		NodeID:              h.consensus.nodeID,
		IsLeader:            h.consensus.isLeader,
		LatestBlockIndex:    latestBlock.Index,
		PendingTransactions: len(pendingTxs),
		Status:              status,
		ConnectedPeers:      connectedPeers,
	}
}

// GetBlock returns a specific block by index
func (h *ConsensusMessageHandler) GetBlock(index int64) *blockchain.Block {
	return h.blockchain.GetBlock(index)
}

// GetLatestBlock returns the latest block
func (h *ConsensusMessageHandler) GetLatestBlock() *blockchain.Block {
	return h.blockchain.GetLatestBlock()
}

// GetBlocksFromIndex returns blocks starting from index
func (h *ConsensusMessageHandler) GetBlocksFromIndex(startIndex int64, limit int) []*blockchain.Block {
	if h.blockchain == nil {
		return nil
	}
	
	var blocks []*blockchain.Block
	totalBlocks := h.blockchain.GetBlockCount()
	
	for i := startIndex; i < totalBlocks && len(blocks) < limit; i++ {
		if block := h.blockchain.GetBlock(i); block != nil {
			blocks = append(blocks, block)
		}
	}
	
	return blocks
}

// Enhanced P2P Service with blockchain access
type EnhancedHTTPP2PService struct {
	*p2p.HTTPP2PService
	handler *ConsensusMessageHandler
}

// NewEnhancedHTTPP2PService creates an enhanced P2P service with blockchain access
func NewEnhancedHTTPP2PService(config *p2p.NetworkConfig, handler *ConsensusMessageHandler) *EnhancedHTTPP2PService {
	baseService := p2p.NewHTTPP2PService(config, handler)
	
	return &EnhancedHTTPP2PService{
		HTTPP2PService: baseService,
		handler:        handler,
	}
}

// RegisterAdditionalEndpoints registers blockchain-specific endpoints
func (s *EnhancedHTTPP2PService) RegisterAdditionalEndpoints() {
	// This would extend the base HTTP service with blockchain-specific endpoints
	// For now, we'll handle this in the node integration
} 