package p2p

import (
	"context"
	"go-blockchain/pkg/blockchain"
)

// P2PMessage represents a generic P2P message
type P2PMessage struct {
	Type      MessageType `json:"type"`
	Sender    string      `json:"sender"`
	Data      []byte      `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

// MessageType defines the type of P2P message
type MessageType int

const (
	MessageTypeTransaction MessageType = iota
	MessageTypeBlockProposal
	MessageTypeVote
	MessageTypeBlockCommit
	MessageTypeSyncRequest
	MessageTypeSyncResponse
)

// TransactionMessage represents a transaction broadcast
type TransactionMessage struct {
	Transaction *blockchain.Transaction `json:"transaction"`
}

// BlockProposalMessage represents a block proposal from leader
type BlockProposalMessage struct {
	Block      *blockchain.Block `json:"block"`
	ProposerID string            `json:"proposer_id"`
	ProposalID int64             `json:"proposal_id"`
}

// VoteMessage represents a vote on a block proposal
type VoteMessage struct {
	ProposalID int64  `json:"proposal_id"`
	BlockHash  []byte `json:"block_hash"`
	Accept     bool   `json:"accept"`
	VoterID    string `json:"voter_id"`
}

// BlockCommitMessage represents a finalized block
type BlockCommitMessage struct {
	Block      *blockchain.Block `json:"block"`
	ProposalID int64             `json:"proposal_id"`
}

// SyncRequestMessage requests blocks from a specific index
type SyncRequestMessage struct {
	StartIndex int64 `json:"start_index"`
	Limit      int   `json:"limit"`
}

// SyncResponseMessage contains requested blocks
type SyncResponseMessage struct {
	Blocks []*blockchain.Block `json:"blocks"`
}

// NodeStatus represents the status of a node
type NodeStatus struct {
	NodeID               string `json:"node_id"`
	IsLeader             bool   `json:"is_leader"`
	LatestBlockIndex     int64  `json:"latest_block_index"`
	PendingTransactions  int    `json:"pending_transactions"`
	Status               string `json:"status"`
	ConnectedPeers       int    `json:"connected_peers"`
}

// P2PService defines the interface for P2P communication
type P2PService interface {
	// Transaction operations
	SendTransaction(ctx context.Context, tx *blockchain.Transaction) error
	BroadcastTransaction(tx *blockchain.Transaction) error
	
	// Block operations
	ProposeBlock(ctx context.Context, block *blockchain.Block, proposalID int64) (map[string]bool, error)
	Vote(ctx context.Context, vote *VoteMessage) error
	CommitBlock(ctx context.Context, block *blockchain.Block, proposalID int64) error
	
	// Sync operations
	GetBlock(ctx context.Context, nodeID string, index int64) (*blockchain.Block, error)
	GetLatestBlock(ctx context.Context, nodeID string) (*blockchain.Block, error)
	GetBlocksFromIndex(ctx context.Context, nodeID string, startIndex int64, limit int) ([]*blockchain.Block, error)
	
	// Node operations
	GetNodeStatus(ctx context.Context, nodeID string) (*NodeStatus, error)
	
	// Connection management
	ConnectToPeers(peers []string) error
	Disconnect() error
}

// MessageHandler defines handlers for different message types
type MessageHandler interface {
	HandleTransaction(tx *blockchain.Transaction) error
	HandleBlockProposal(proposal *BlockProposalMessage) (bool, error)
	HandleVote(vote *VoteMessage) error
	HandleBlockCommit(commit *BlockCommitMessage) error
	HandleSyncRequest(request *SyncRequestMessage) (*SyncResponseMessage, error)
}

// PeerInfo represents information about a peer node
type PeerInfo struct {
	NodeID    string `json:"node_id"`
	Address   string `json:"address"`
	Port      int    `json:"port"`
	Connected bool   `json:"connected"`
	LastSeen  int64  `json:"last_seen"`
}

// NetworkConfig holds network configuration
type NetworkConfig struct {
	NodeID      string   `json:"node_id"`
	Port        int      `json:"port"`
	Peers       []string `json:"peers"`
	MaxPeers    int      `json:"max_peers"`
	Timeout     int      `json:"timeout_seconds"`
} 