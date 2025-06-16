package blockchain

import (
	"fmt"
	"sync"
)

// Blockchain represents the blockchain
type Blockchain struct {
	blocks            []*Block
	pendingTxs        []*Transaction
	mutex             sync.RWMutex
	difficulty        int // For future proof-of-work implementation
	miningReward      float64
	transactionPool   map[string]*Transaction // Hash -> Transaction
}

// NewBlockchain creates a new blockchain with deterministic genesis
func NewBlockchain() *Blockchain {
	bc := &Blockchain{
		blocks:          make([]*Block, 0),
		pendingTxs:      make([]*Transaction, 0),
		difficulty:      1,
		miningReward:    10.0,
		transactionPool: make(map[string]*Transaction),
	}
	
	// Create deterministic genesis block (same for all nodes)
	genesisBlock := createDeterministicGenesis()
	bc.blocks = append(bc.blocks, genesisBlock)
	
	return bc
}

// createDeterministicGenesis creates identical genesis for all nodes
func createDeterministicGenesis() *Block {
	// Fixed values for deterministic genesis
	genesis := &Block{
		Index:             0,
		Transactions:      []*Transaction{},
		Timestamp:         1704067200, // Fixed: 2024-01-01 00:00:00 UTC
		PreviousBlockHash: []byte{},
		Nonce:             0,
	}
	
	// Calculate hashes using correct method names
	genesis.MerkleRoot = CalculateTransactionsMerkleRoot(genesis.Transactions)
	genesis.CurrentBlockHash = genesis.CalculateHash()
	
	return genesis
}

// GetLatestBlock returns the latest block in the chain
func (bc *Blockchain) GetLatestBlock() *Block {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	
	if len(bc.blocks) == 0 {
		return nil
	}
	return bc.blocks[len(bc.blocks)-1]
}

// GetBlock returns a block by index
func (bc *Blockchain) GetBlock(index int64) *Block {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	
	if index < 0 || index >= int64(len(bc.blocks)) {
		return nil
	}
	return bc.blocks[index]
}

// GetBlockByHash returns a block by its hash
func (bc *Blockchain) GetBlockByHash(hash []byte) *Block {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	
	for _, block := range bc.blocks {
		if bytesEqual(block.CurrentBlockHash, hash) {
			return block
		}
	}
	return nil
}

// GetBlockCount returns the number of blocks in the chain
func (bc *Blockchain) GetBlockCount() int64 {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	
	return int64(len(bc.blocks))
}

// AddBlock adds a new block to the blockchain
func (bc *Blockchain) AddBlock(block *Block) error {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	
	// Validate the block
	var previousBlock *Block
	if len(bc.blocks) > 0 {
		previousBlock = bc.blocks[len(bc.blocks)-1]
	}
	
	if err := block.ValidateBlock(previousBlock); err != nil {
		return fmt.Errorf("invalid block: %w", err)
	}
	
	// Add the block
	bc.blocks = append(bc.blocks, block)
	
	// Remove processed transactions from pending pool AND pending slice
	for _, tx := range block.Transactions {
		txHashStr := fmt.Sprintf("%x", tx.TxHash)
		delete(bc.transactionPool, txHashStr)
		
		// REMOVE FROM PENDING SLICE
		for i := len(bc.pendingTxs) - 1; i >= 0; i-- {
			if fmt.Sprintf("%x", bc.pendingTxs[i].TxHash) == txHashStr {
				// Remove by replacing with last element and truncating
				bc.pendingTxs[i] = bc.pendingTxs[len(bc.pendingTxs)-1]
				bc.pendingTxs = bc.pendingTxs[:len(bc.pendingTxs)-1]
				break
			}
		}
	}
	
	return nil
}

// CreateBlock creates a new block with pending transactions
func (bc *Blockchain) CreateBlock(maxTransactions int) (*Block, error) {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	
	// Get latest block for reference
	latestBlock := bc.blocks[len(bc.blocks)-1]
	
	// Select transactions to include (up to maxTransactions)
	var selectedTxs []*Transaction
	count := 0
	for _, tx := range bc.pendingTxs {
		if count >= maxTransactions {
			break
		}
		selectedTxs = append(selectedTxs, tx)
		count++
	}
	
	// Create new block
	newBlock := NewBlock(
		latestBlock.Index+1,
		selectedTxs,
		latestBlock.CurrentBlockHash,
	)
	
	return newBlock, nil
}

// AddTransaction adds a transaction to the pending pool
func (bc *Blockchain) AddTransaction(tx *Transaction) error {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	
	// Basic validation
	if !tx.IsValid() {
		return fmt.Errorf("invalid transaction")
	}
	
	// Check if transaction already exists
	txHashStr := fmt.Sprintf("%x", tx.TxHash)
	if _, exists := bc.transactionPool[txHashStr]; exists {
		return fmt.Errorf("transaction already exists in pool")
	}
	
	// Add to transaction pool and pending list
	bc.transactionPool[txHashStr] = tx
	bc.pendingTxs = append(bc.pendingTxs, tx)
	
	return nil
}

// GetPendingTransactions returns all pending transactions
func (bc *Blockchain) GetPendingTransactions() []*Transaction {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	
	// Return a copy to avoid race conditions
	txs := make([]*Transaction, len(bc.pendingTxs))
	copy(txs, bc.pendingTxs)
	return txs
}

// GetTransactionCount returns the total number of transactions in the blockchain
func (bc *Blockchain) GetTransactionCount() int {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	
	count := 0
	for _, block := range bc.blocks {
		count += len(block.Transactions)
	}
	return count
}

// GetBalance calculates the balance for a given address
func (bc *Blockchain) GetBalance(address []byte) float64 {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	
	balance := 0.0
	
	// Iterate through all blocks and transactions
	for _, block := range bc.blocks {
		for _, tx := range block.Transactions {
			// If address is sender, subtract amount
			if bytesEqual(tx.Sender, address) {
				balance -= tx.Amount
			}
			// If address is receiver, add amount
			if bytesEqual(tx.Receiver, address) {
				balance += tx.Amount
			}
		}
	}
	
	return balance
}

// IsValidChain validates the entire blockchain
func (bc *Blockchain) IsValidChain() bool {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	
	// Check if we have at least genesis block
	if len(bc.blocks) == 0 {
		return false
	}
	
	// Validate each block
	for i := 1; i < len(bc.blocks); i++ {
		currentBlock := bc.blocks[i]
		previousBlock := bc.blocks[i-1]
		
		if err := currentBlock.ValidateBlock(previousBlock); err != nil {
			return false
		}
	}
	
	return true
}

// GetAllBlocks returns all blocks (for debugging/API)
func (bc *Blockchain) GetAllBlocks() []*Block {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	
	// Return a copy to avoid race conditions
	blocks := make([]*Block, len(bc.blocks))
	copy(blocks, bc.blocks)
	return blocks
}

// String returns string representation of the blockchain
func (bc *Blockchain) String() string {
	return fmt.Sprintf("Blockchain{Blocks: %d, PendingTxs: %d, Valid: %t}",
		len(bc.blocks), len(bc.pendingTxs), bc.IsValidChain())
} 