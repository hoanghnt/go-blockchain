package blockchain

import (
	"crypto/sha256"
	"fmt"
)

// MerkleNode represents a node in the Merkle tree
type MerkleNode struct {
	Left  *MerkleNode
	Right *MerkleNode
	Hash  []byte
}

// MerkleTree represents a Merkle tree
type MerkleTree struct {
	Root *MerkleNode
}

// NewMerkleNode creates a new Merkle tree node
func NewMerkleNode(left, right *MerkleNode, data []byte) *MerkleNode {
	node := &MerkleNode{}
	
	if left == nil && right == nil {
		// Leaf node - hash the data
		hash := sha256.Sum256(data)
		node.Hash = hash[:]
	} else {
		// Internal node - hash the concatenation of left and right hashes
		var prevHashes []byte
		if left != nil {
			prevHashes = append(prevHashes, left.Hash...)
		}
		if right != nil {
			prevHashes = append(prevHashes, right.Hash...)
		}
		hash := sha256.Sum256(prevHashes)
		node.Hash = hash[:]
	}
	
	node.Left = left
	node.Right = right
	
	return node
}

// NewMerkleTree creates a new Merkle tree from transaction hashes
func NewMerkleTree(data [][]byte) *MerkleTree {
	if len(data) == 0 {
		return &MerkleTree{}
	}
	
	// Create leaf nodes
	var nodes []*MerkleNode
	for _, datum := range data {
		node := NewMerkleNode(nil, nil, datum)
		nodes = append(nodes, node)
	}
	
	// Build tree bottom-up
	for len(nodes) > 1 {
		var level []*MerkleNode
		
		// Pair nodes and create parent nodes
		for i := 0; i < len(nodes); i += 2 {
			left := nodes[i]
			var right *MerkleNode
			
			if i+1 < len(nodes) {
				right = nodes[i+1]
			} else {
				// Odd number of nodes - duplicate the last one
				right = nodes[i]
			}
			
			parent := NewMerkleNode(left, right, nil)
			level = append(level, parent)
		}
		
		nodes = level
	}
	
	return &MerkleTree{Root: nodes[0]}
}

// GetMerkleRoot returns the root hash of the Merkle tree
func (mt *MerkleTree) GetMerkleRoot() []byte {
	if mt.Root == nil {
		return nil
	}
	return mt.Root.Hash
}

// CalculateTransactionsMerkleRoot calculates Merkle root from transactions
func CalculateTransactionsMerkleRoot(transactions []*Transaction) []byte {
	if len(transactions) == 0 {
		// Empty block - return hash of empty string
		hash := sha256.Sum256([]byte(""))
		return hash[:]
	}
	
	// Get transaction hashes
	var txHashes [][]byte
	for _, tx := range transactions {
		txHash := tx.Hash()
		txHashes = append(txHashes, txHash)
	}
	
	// Build Merkle tree
	tree := NewMerkleTree(txHashes)
	return tree.GetMerkleRoot()
}

// VerifyMerkleRoot verifies if the given transactions produce the expected Merkle root
func VerifyMerkleRoot(transactions []*Transaction, expectedRoot []byte) bool {
	calculatedRoot := CalculateTransactionsMerkleRoot(transactions)
	
	if len(calculatedRoot) != len(expectedRoot) {
		return false
	}
	
	for i := range calculatedRoot {
		if calculatedRoot[i] != expectedRoot[i] {
			return false
		}
	}
	
	return true
}

// String returns string representation of Merkle root
func (mt *MerkleTree) String() string {
	if mt.Root == nil {
		return "Empty Merkle Tree"
	}
	return fmt.Sprintf("Merkle Root: %x", mt.Root.Hash)
} 