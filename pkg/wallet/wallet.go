package wallet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
)

// Wallet represents a user's wallet with ECDSA key pair
type Wallet struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
	Address    []byte
}

// GenerateKeyPair creates a new ECDSA key pair
func GenerateKeyPair() (*ecdsa.PrivateKey, error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}
	return privKey, nil
}

// NewWallet creates a new wallet with generated ECDSA key pair
func NewWallet() (*Wallet, error) {
	privKey, err := GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	wallet := &Wallet{
		PrivateKey: privKey,
		PublicKey:  &privKey.PublicKey,
		Address:    PublicKeyToAddress(&privKey.PublicKey),
	}

	return wallet, nil
}

// PublicKeyToAddress converts a public key to an address (hash of public key)
func PublicKeyToAddress(pubKey *ecdsa.PublicKey) []byte {
	// Concatenate X and Y coordinates of the public key
	pubKeyBytes := append(pubKey.X.Bytes(), pubKey.Y.Bytes()...)
	
	// Hash the public key to create address
	hash := sha256.Sum256(pubKeyBytes)
	return hash[:20] // Use first 20 bytes as address (similar to Ethereum)
}

// SignData signs arbitrary data with the wallet's private key
func (w *Wallet) SignData(data []byte) ([]byte, error) {
	hash := sha256.Sum256(data)
	r, s, err := ecdsa.Sign(rand.Reader, w.PrivateKey, hash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %w", err)
	}
	
	// Concatenate R and S as signature
	signature := append(r.Bytes(), s.Bytes()...)
	return signature, nil
}

// VerifySignature verifies a signature against data and public key
func VerifySignature(data []byte, signature []byte, pubKey *ecdsa.PublicKey) bool {
	hash := sha256.Sum256(data)
	
	// Split signature back to R and S
	sigLen := len(signature)
	if sigLen%2 != 0 {
		return false
	}
	
	r := new(big.Int).SetBytes(signature[:sigLen/2])
	s := new(big.Int).SetBytes(signature[sigLen/2:])
	
	return ecdsa.Verify(pubKey, hash[:], r, s)
}

// GetAddress returns the wallet's address
func (w *Wallet) GetAddress() []byte {
	return w.Address
}

// ExportWallet exports wallet to JSON format (for CLI usage)
func (w *Wallet) ExportWallet() ([]byte, error) {
	walletData := map[string]interface{}{
		"private_key_x": w.PrivateKey.X.String(),
		"private_key_y": w.PrivateKey.Y.String(),
		"private_key_d": w.PrivateKey.D.String(),
		"public_key_x":  w.PublicKey.X.String(),
		"public_key_y":  w.PublicKey.Y.String(),
		"address":       fmt.Sprintf("%x", w.Address),
	}
	
	return json.MarshalIndent(walletData, "", "  ")
}

// ImportWallet imports wallet from JSON format
func (w *Wallet) ImportWallet(data []byte) error {
	var walletData map[string]interface{}
	if err := json.Unmarshal(data, &walletData); err != nil {
		return fmt.Errorf("failed to unmarshal wallet data: %w", err)
	}

	// Parse private key
	privKeyD, ok := walletData["private_key_d"].(string)
	if !ok {
		return fmt.Errorf("invalid private key format")
	}

	d := new(big.Int)
	d.SetString(privKeyD, 10)

	privKey := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     new(big.Int),
			Y:     new(big.Int),
		},
		D: d,
	}

	// Parse public key coordinates
	pubKeyX, ok := walletData["public_key_x"].(string)
	if !ok {
		return fmt.Errorf("invalid public key X format")
	}
	pubKeyY, ok := walletData["public_key_y"].(string)
	if !ok {
		return fmt.Errorf("invalid public key Y format")
	}

	privKey.PublicKey.X.SetString(pubKeyX, 10)
	privKey.PublicKey.Y.SetString(pubKeyY, 10)

	// Set wallet fields
	w.PrivateKey = privKey
	w.PublicKey = &privKey.PublicKey
	w.Address = PublicKeyToAddress(&privKey.PublicKey)

	return nil
}

// LoadWalletFromFile loads wallet from JSON file
func LoadWalletFromFile(filename string) (*Wallet, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read wallet file: %w", err)
	}

	w := &Wallet{}
	if err := w.ImportWallet(data); err != nil {
		return nil, fmt.Errorf("failed to import wallet: %w", err)
	}

	return w, nil
} 