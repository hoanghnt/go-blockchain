package p2p

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go-blockchain/pkg/blockchain"
)

// HTTPP2PService implements P2P communication using HTTP
type HTTPP2PService struct {
	config      *NetworkConfig
	peers       map[string]*PeerInfo
	peersMutex  sync.RWMutex
	handler     MessageHandler
	server      *http.Server
	httpClient  *http.Client
}

// NewHTTPP2PService creates a new HTTP-based P2P service
func NewHTTPP2PService(config *NetworkConfig, handler MessageHandler) *HTTPP2PService {
	return &HTTPP2PService{
		config:     config,
		peers:      make(map[string]*PeerInfo),
		handler:    handler,
		httpClient: &http.Client{Timeout: time.Duration(config.Timeout) * time.Second},
	}
}

// Start starts the HTTP server for P2P communication
func (s *HTTPP2PService) Start() error {
	mux := http.NewServeMux()
	
	// Register endpoints
	mux.HandleFunc("/transaction", s.handleTransactionEndpoint)
	mux.HandleFunc("/block/propose", s.handleBlockProposalEndpoint)
	mux.HandleFunc("/block/vote", s.handleVoteEndpoint)
	mux.HandleFunc("/block/commit", s.handleBlockCommitEndpoint)
	mux.HandleFunc("/block/get", s.handleGetBlockEndpoint)
	mux.HandleFunc("/block/latest", s.handleGetLatestBlockEndpoint)
	mux.HandleFunc("/blocks/sync", s.handleSyncEndpoint)
	mux.HandleFunc("/blockchain", s.handleGetBlockchainEndpoint)
	mux.HandleFunc("/node/status", s.handleNodeStatusEndpoint)
	
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.Port),
		Handler: mux,
	}
	
	log.Printf("Starting P2P HTTP server on port %d", s.config.Port)
	
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("P2P server error: %v", err)
		}
	}()
	
	return nil
}

// Stop stops the HTTP server
func (s *HTTPP2PService) Stop() error {
	if s.server != nil {
		return s.server.Shutdown(context.Background())
	}
	return nil
}

// ConnectToPeers connects to peer nodes
func (s *HTTPP2PService) ConnectToPeers(peers []string) error {
	s.peersMutex.Lock()
	defer s.peersMutex.Unlock()
	
	for _, peerAddr := range peers {
		if peerAddr == fmt.Sprintf("localhost:%d", s.config.Port) {
			continue // Skip self
		}
		
		peerInfo := &PeerInfo{
			NodeID:    fmt.Sprintf("node-%s", peerAddr),
			Address:   peerAddr,
			Connected: false,
			LastSeen:  time.Now().Unix(),
		}
		
		// Test connection
		if s.testConnection(peerAddr) {
			peerInfo.Connected = true
			log.Printf("Connected to peer: %s", peerAddr)
		} else {
			log.Printf("Failed to connect to peer: %s", peerAddr)
		}
		
		s.peers[peerAddr] = peerInfo
	}
	
	return nil
}

// testConnection tests if a peer is reachable
func (s *HTTPP2PService) testConnection(peerAddr string) bool {
	url := fmt.Sprintf("http://%s/node/status", peerAddr)
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// SendTransaction sends a transaction to a specific peer
func (s *HTTPP2PService) SendTransaction(ctx context.Context, tx *blockchain.Transaction) error {
	message := &TransactionMessage{Transaction: tx}
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	
	// Send to all connected peers
	return s.broadcastMessage("/transaction", data)
}

// BroadcastTransaction broadcasts a transaction to all peers
func (s *HTTPP2PService) BroadcastTransaction(tx *blockchain.Transaction) error {
	return s.SendTransaction(context.Background(), tx)
}

// ProposeBlock proposes a block to all peers and collects votes
func (s *HTTPP2PService) ProposeBlock(ctx context.Context, block *blockchain.Block, proposalID int64) (map[string]bool, error) {
	proposal := &BlockProposalMessage{
		Block:      block,
		ProposerID: s.config.NodeID,
		ProposalID: proposalID,
	}
	
	data, err := json.Marshal(proposal)
	if err != nil {
		return nil, err
	}
	
	votes := make(map[string]bool)
	s.peersMutex.RLock()
	defer s.peersMutex.RUnlock()
	
	for peerAddr, peer := range s.peers {
		if !peer.Connected {
			continue
		}
		
		vote, err := s.sendBlockProposal(peerAddr, data)
		if err != nil {
			log.Printf("Failed to get vote from %s: %v", peerAddr, err)
			votes[peerAddr] = false
		} else {
			votes[peerAddr] = vote
		}
	}
	
	return votes, nil
}

// sendBlockProposal sends a block proposal to a specific peer and gets vote
func (s *HTTPP2PService) sendBlockProposal(peerAddr string, data []byte) (bool, error) {
	url := fmt.Sprintf("http://%s/block/propose", peerAddr)
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("peer rejected proposal: %d", resp.StatusCode)
	}
	
	var response struct {
		Accept bool `json:"accept"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return false, err
	}
	
	return response.Accept, nil
}

// Vote sends a vote to peers
func (s *HTTPP2PService) Vote(ctx context.Context, vote *VoteMessage) error {
	data, err := json.Marshal(vote)
	if err != nil {
		return err
	}
	
	return s.broadcastMessage("/block/vote", data)
}

// CommitBlock commits a block to all peers
func (s *HTTPP2PService) CommitBlock(ctx context.Context, block *blockchain.Block, proposalID int64) error {
	commit := &BlockCommitMessage{
		Block:      block,
		ProposalID: proposalID,
	}
	
	data, err := json.Marshal(commit)
	if err != nil {
		return err
	}
	
	return s.broadcastMessage("/block/commit", data)
}

// broadcastMessage sends a message to all connected peers
func (s *HTTPP2PService) broadcastMessage(endpoint string, data []byte) error {
	s.peersMutex.RLock()
	defer s.peersMutex.RUnlock()
	
	for peerAddr, peer := range s.peers {
		if !peer.Connected {
			continue
		}
		
		go func(addr string) {
			url := fmt.Sprintf("http://%s%s", addr, endpoint)
			resp, err := http.Post(url, "application/json", bytes.NewReader(data))
			if err != nil {
				log.Printf("Failed to send message to %s: %v", addr, err)
				return
			}
			defer resp.Body.Close()
		}(peerAddr)
	}
	
	return nil
}

// GetBlock gets a block from a specific peer
func (s *HTTPP2PService) GetBlock(ctx context.Context, nodeID string, index int64) (*blockchain.Block, error) {
	// Find peer by nodeID
	var peerAddr string
	s.peersMutex.RLock()
	for addr, peer := range s.peers {
		if peer.NodeID == nodeID && peer.Connected {
			peerAddr = addr
			break
		}
	}
	s.peersMutex.RUnlock()
	
	if peerAddr == "" {
		return nil, fmt.Errorf("peer not found or not connected: %s", nodeID)
	}
	
	url := fmt.Sprintf("http://%s/block/get?index=%d", peerAddr, index)
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get block: %d", resp.StatusCode)
	}
	
	var block blockchain.Block
	if err := json.NewDecoder(resp.Body).Decode(&block); err != nil {
		return nil, err
	}
	
	return &block, nil
}

// GetLatestBlock gets the latest block from a peer
func (s *HTTPP2PService) GetLatestBlock(ctx context.Context, nodeID string) (*blockchain.Block, error) {
	// Similar implementation to GetBlock but for latest block
	var peerAddr string
	s.peersMutex.RLock()
	for addr, peer := range s.peers {
		if peer.NodeID == nodeID && peer.Connected {
			peerAddr = addr
			break
		}
	}
	s.peersMutex.RUnlock()
	
	if peerAddr == "" {
		return nil, fmt.Errorf("peer not found or not connected: %s", nodeID)
	}
	
	url := fmt.Sprintf("http://%s/block/latest", peerAddr)
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get latest block: %d", resp.StatusCode)
	}
	
	var block blockchain.Block
	if err := json.NewDecoder(resp.Body).Decode(&block); err != nil {
		return nil, err
	}
	
	return &block, nil
}

// GetBlocksFromIndex gets blocks from a specific index
func (s *HTTPP2PService) GetBlocksFromIndex(ctx context.Context, nodeID string, startIndex int64, limit int) ([]*blockchain.Block, error) {
	var peerAddr string
	s.peersMutex.RLock()
	for addr, peer := range s.peers {
		if peer.NodeID == nodeID && peer.Connected {
			peerAddr = addr
			break
		}
	}
	s.peersMutex.RUnlock()
	
	if peerAddr == "" {
		return nil, fmt.Errorf("peer not found or not connected: %s", nodeID)
	}
	
	url := fmt.Sprintf("http://%s/blocks/sync?start=%d&limit=%d", peerAddr, startIndex, limit)
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get blocks: %d", resp.StatusCode)
	}
	
	var response SyncResponseMessage
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	
	return response.Blocks, nil
}

// GetNodeStatus gets status from a specific node
func (s *HTTPP2PService) GetNodeStatus(ctx context.Context, nodeID string) (*NodeStatus, error) {
	var peerAddr string
	s.peersMutex.RLock()
	for addr, peer := range s.peers {
		if peer.NodeID == nodeID && peer.Connected {
			peerAddr = addr
			break
		}
	}
	s.peersMutex.RUnlock()
	
	if peerAddr == "" {
		return nil, fmt.Errorf("peer not found or not connected: %s", nodeID)
	}
	
	url := fmt.Sprintf("http://%s/node/status", peerAddr)
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get node status: %d", resp.StatusCode)
	}
	
	var status NodeStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	
	return &status, nil
}

// Disconnect disconnects from all peers
func (s *HTTPP2PService) Disconnect() error {
	s.peersMutex.Lock()
	defer s.peersMutex.Unlock()
	
	for _, peer := range s.peers {
		peer.Connected = false
	}
	
	return s.Stop()
}

// HTTP endpoint handlers

func (s *HTTPP2PService) handleTransactionEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var message TransactionMessage
	if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	if err := s.handler.HandleTransaction(message.Transaction); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
}

func (s *HTTPP2PService) handleBlockProposalEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var proposal BlockProposalMessage
	if err := json.NewDecoder(r.Body).Decode(&proposal); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	accept, err := s.handler.HandleBlockProposal(&proposal)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	response := map[string]bool{"accept": accept}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *HTTPP2PService) handleVoteEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var vote VoteMessage
	if err := json.NewDecoder(r.Body).Decode(&vote); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	if err := s.handler.HandleVote(&vote); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
}

func (s *HTTPP2PService) handleBlockCommitEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var commit BlockCommitMessage
	if err := json.NewDecoder(r.Body).Decode(&commit); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	if err := s.handler.HandleBlockCommit(&commit); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
}

func (s *HTTPP2PService) handleGetBlockEndpoint(w http.ResponseWriter, r *http.Request) {
	// Implementation depends on the handler having access to blockchain
	// For now, return a simple response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *HTTPP2PService) handleGetLatestBlockEndpoint(w http.ResponseWriter, r *http.Request) {
	// Implementation depends on the handler having access to blockchain
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *HTTPP2PService) handleSyncEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	startIndexStr := r.URL.Query().Get("start")
	limitStr := r.URL.Query().Get("limit")
	
	startIndex := int64(0)
	if startIndexStr != "" {
		if parsed, err := strconv.ParseInt(startIndexStr, 10, 64); err == nil {
			startIndex = parsed
		}
	}
	
	limit := 100 // Default limit
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	// Get blocks from handler
	if handler, ok := s.handler.(interface{ GetBlocksFromIndex(int64, int) []*blockchain.Block }); ok {
		blocks := handler.GetBlocksFromIndex(startIndex, limit)
		
		response := SyncResponseMessage{
			Blocks: blocks,
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	} else {
		http.Error(w, "Handler not available", http.StatusInternalServerError)
	}
}

func (s *HTTPP2PService) handleNodeStatusEndpoint(w http.ResponseWriter, r *http.Request) {
	// Get status from handler if available
	var status *NodeStatus
	
	if handler, ok := s.handler.(interface{ GetNodeStatus() *NodeStatus }); ok {
		status = handler.GetNodeStatus()
	} else {
		// Fallback status
		status = &NodeStatus{
			NodeID:   s.config.NodeID,
			IsLeader: false,
			Status:   "running",
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *HTTPP2PService) handleGetBlockchainEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get blockchain from handler
	if handler, ok := s.handler.(interface{ GetBlocksFromIndex(int64, int) []*blockchain.Block }); ok {
		blocks := handler.GetBlocksFromIndex(0, 1000) // Get up to 1000 blocks
		
		response := struct {
			Blocks    []*blockchain.Block `json:"blocks"`
			IsValid   bool                `json:"is_valid"`
			Total     int                 `json:"total"`
		}{
			Blocks:  blocks,
			IsValid: true, 
			Total:   len(blocks),
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	} else {
		http.Error(w, "Handler not available", http.StatusInternalServerError)
	}
}

// GetConnectedPeers returns map of connected peer addresses
func (s *HTTPP2PService) GetConnectedPeers() map[string]*PeerInfo {
	s.peersMutex.RLock()
	defer s.peersMutex.RUnlock()
	
	connected := make(map[string]*PeerInfo)
	for addr, peer := range s.peers {
		if peer.Connected {
			connected[addr] = peer
		}
	}
	
	return connected
} 