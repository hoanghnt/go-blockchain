package blockchain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// Block represents a block in the blockchain
type Block struct {
	Index            int64          `json:"index"`              // Block number
	Timestamp        int64          `json:"timestamp"`          // Block creation time
	Transactions     []*Transaction `json:"transactions"`       // List of transactions
	MerkleRoot       []byte         `json:"merkle_root"`        // Merkle root of transactions
	PreviousBlockHash []byte        `json:"previous_block_hash"` // Hash of previous block
	CurrentBlockHash  []byte        `json:"current_block_hash"`  // Hash of current block
	Nonce            int64          `json:"nonce"`              // For future proof-of-work if needed
}

// NewBlock creates a new block
func NewBlock(index int64, transactions []*Transaction, previousHash []byte) *Block {
	block := &Block{
		Index:             index,
		Timestamp:         time.Now().Unix(),
		Transactions:      transactions,
		MerkleRoot:        CalculateTransactionsMerkleRoot(transactions),
		PreviousBlockHash: previousHash,
		Nonce:             0,
	}
	
	// Calculate and set the current block hash
	block.CurrentBlockHash = block.CalculateHash()
	
	return block
}

// NewGenesisBlock creates the first block in the blockchain
func NewGenesisBlock() *Block {
	// Genesis block has no transactions and no previous hash
	return NewBlock(0, []*Transaction{}, []byte{})
}

// CalculateHash calculates the hash of the block
func (b *Block) CalculateHash() []byte {
	// Create a copy without CurrentBlockHash for hashing
	blockData := struct {
		Index             int64          `json:"index"`
		Timestamp         int64          `json:"timestamp"`
		Transactions      []*Transaction `json:"transactions"`
		MerkleRoot        []byte         `json:"merkle_root"`
		PreviousBlockHash []byte         `json:"previous_block_hash"`
		Nonce             int64          `json:"nonce"`
	}{
		Index:             b.Index,
		Timestamp:         b.Timestamp,
		Transactions:      b.Transactions,
		MerkleRoot:        b.MerkleRoot,
		PreviousBlockHash: b.PreviousBlockHash,
		Nonce:             b.Nonce,
	}
	
	data, _ := json.Marshal(blockData)
	hash := sha256.Sum256(data)
	return hash[:]
}

// ValidateBlock validates the block structure and content
func (b *Block) ValidateBlock(previousBlock *Block) error {
	// Check if it's genesis block
	if b.Index == 0 && previousBlock == nil {
		return nil // Genesis block is always valid
	}
	
	if previousBlock == nil {
		return fmt.Errorf("previous block is required for non-genesis block")
	}
	
	// Check index continuity
	if b.Index != previousBlock.Index+1 {
		return fmt.Errorf("invalid block index: expected %d, got %d", 
			previousBlock.Index+1, b.Index)
	}
	
	// Check previous hash reference
	if !bytesEqual(b.PreviousBlockHash, previousBlock.CurrentBlockHash) {
		return fmt.Errorf("invalid previous block hash")
	}
	
	// Check timestamp (should be after previous block)
	if b.Timestamp <= previousBlock.Timestamp {
		return fmt.Errorf("invalid timestamp: block timestamp should be after previous block")
	}
	
	// Validate Merkle root
	expectedMerkleRoot := CalculateTransactionsMerkleRoot(b.Transactions)
	if !bytesEqual(b.MerkleRoot, expectedMerkleRoot) {
		return fmt.Errorf("invalid Merkle root")
	}
	
	// Validate current block hash
	expectedHash := b.CalculateHash()
	if !bytesEqual(b.CurrentBlockHash, expectedHash) {
		return fmt.Errorf("invalid block hash")
	}
	
	// Validate all transactions in the block
	for i, tx := range b.Transactions {
		if !tx.IsValid() {
			return fmt.Errorf("invalid transaction at index %d", i)
		}
	}
	
	return nil
}

// AddTransaction adds a transaction to the block (used during block construction)
func (b *Block) AddTransaction(tx *Transaction) {
	b.Transactions = append(b.Transactions, tx)
	// Recalculate Merkle root
	b.MerkleRoot = CalculateTransactionsMerkleRoot(b.Transactions)
	// Recalculate block hash
	b.CurrentBlockHash = b.CalculateHash()
}

// GetTransactionCount returns the number of transactions in the block
func (b *Block) GetTransactionCount() int {
	return len(b.Transactions)
}

// HasTransaction checks if a transaction exists in the block
func (b *Block) HasTransaction(txHash []byte) bool {
	for _, tx := range b.Transactions {
		if bytesEqual(tx.TxHash, txHash) {
			return true
		}
	}
	return false
}

// String returns string representation of the block
func (b *Block) String() string {
	return fmt.Sprintf("Block{Index: %d, Hash: %x, PrevHash: %x, Transactions: %d, Time: %d}",
		b.Index, b.CurrentBlockHash, b.PreviousBlockHash, len(b.Transactions), b.Timestamp)
}

// ToJSON converts block to JSON
func (b *Block) ToJSON() ([]byte, error) {
	return json.MarshalIndent(b, "", "  ")
}

// bytesEqual compares two byte slices
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
} 