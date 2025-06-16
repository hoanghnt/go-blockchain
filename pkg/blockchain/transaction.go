package blockchain

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
	
	"go-blockchain/pkg/wallet"
)

// Transaction represents a blockchain transaction
type Transaction struct {
	Sender    []byte  `json:"sender"`    // Address of sender
	Receiver  []byte  `json:"receiver"`  // Address of receiver
	Amount    float64 `json:"amount"`    // Amount to transfer
	Timestamp int64   `json:"timestamp"` // Unix timestamp
	Signature []byte  `json:"signature"` // ECDSA signature
	TxHash    []byte  `json:"tx_hash"`   // Hash of the transaction
}

// NewTransaction creates a new transaction
func NewTransaction(sender, receiver []byte, amount float64) *Transaction {
	return &Transaction{
		Sender:    sender,
		Receiver:  receiver,
		Amount:    amount,
		Timestamp: time.Now().Unix(),
	}
}

// Hash calculates the hash of the transaction (excluding signature)
func (tx *Transaction) Hash() []byte {
	// Create a copy without signature for hashing
	txCopy := Transaction{
		Sender:    tx.Sender,
		Receiver:  tx.Receiver,
		Amount:    tx.Amount,
		Timestamp: tx.Timestamp,
	}
	
	data, _ := json.Marshal(txCopy)
	hash := sha256.Sum256(data)
	return hash[:]
}

// SignTransaction signs the transaction with the given private key
func (tx *Transaction) SignTransaction(privKey *ecdsa.PrivateKey) error {
	// Hash the transaction data
	txHash := tx.Hash()
	tx.TxHash = txHash
	
	// Sign the hash
	signature, err := (&wallet.Wallet{PrivateKey: privKey}).SignData(txHash)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %w", err)
	}
	
	tx.Signature = signature
	return nil
}

// VerifyTransaction verifies the transaction signature
func (tx *Transaction) VerifyTransaction(pubKey *ecdsa.PublicKey) bool {
	if tx.Signature == nil || len(tx.Signature) == 0 {
		return false
	}
	
	// Hash the transaction data
	txHash := tx.Hash()
	
	// Verify signature
	return wallet.VerifySignature(txHash, tx.Signature, pubKey)
}

// IsValid checks if the transaction is valid
func (tx *Transaction) IsValid() bool {
	// Check basic transaction validity
	if tx.Amount <= 0 {
		return false
	}
	
	if len(tx.Sender) == 0 || len(tx.Receiver) == 0 {
		return false
	}
	
	if tx.Timestamp <= 0 {
		return false
	}
	
	// Check if signature exists
	if len(tx.Signature) == 0 {
		return false
	}
	
	return true
}

// String returns string representation of transaction
func (tx *Transaction) String() string {
	return fmt.Sprintf("Transaction{From: %x, To: %x, Amount: %.2f, Time: %d}",
		tx.Sender, tx.Receiver, tx.Amount, tx.Timestamp)
} 