package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"go-blockchain/pkg/blockchain"
	"go-blockchain/pkg/wallet"
)

// getDefaultNodes returns appropriate node URLs based on environment
func getDefaultNodes() []string {
	// Check if running inside Docker container
	if isRunningInContainer() {
		// Use container network names with internal ports
		return []string{
			"http://node1:50051",   // Current container
			"http://node2:50051",   // Other containers
			"http://node3:50051",
		}
	}
	
	// Running on host - use localhost with mapped ports
	return []string{
		"http://localhost:50051",
		"http://localhost:50052", 
		"http://localhost:50053",
	}
}

// isRunningInContainer detects if CLI is running inside Docker
func isRunningInContainer() bool {
	// Method 1: Check if /.dockerenv exists
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	
	// Method 2: Try to resolve container hostnames
	if _, err := net.LookupHost("node2"); err == nil {
		return true
	}
	
	// Method 3: Check environment variables
	if os.Getenv("DOCKER_ENV") == "true" {
		return true
	}
	
	return false
}

// Update to use dynamic detection
var defaultNodes = getDefaultNodes()

// TransactionMessage represents a transaction message for P2P
type TransactionMessage struct {
	Transaction *blockchain.Transaction `json:"transaction"`
}

// BlockchainResponse represents blockchain data from nodes
type BlockchainResponse struct {
	Blocks  []*blockchain.Block `json:"blocks"`
	IsValid bool               `json:"is_valid"`
	Total   int                `json:"total"`
}

// AddressBook for friendly names
type AddressBook map[string]string

var addressBook = AddressBook{
	"alice": "",  // Will be populated
	"bob": "",    // Will be populated  
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "create-wallet":
		createWallet()
	case "create-transaction":
		createTransaction()
	case "send":  // ⭐ NEW SIMPLIFIED COMMAND
		sendTransaction()
	case "show-balance":
		showBalance()
	case "show-blockchain":
		showBlockchain()
	case "show-block":
		showBlock()
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Go-Blockchain CLI Usage:")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  create-wallet [name]          - Create a new wallet")
	fmt.Println("  send <from> <to> <amount>     - Send money between wallets")
	fmt.Println("  create-transaction [options]  - Create a transaction (advanced)")
	fmt.Println("    -from [wallet-name]         - Sender wallet name")
	fmt.Println("    -to [wallet-name|address]   - Receiver wallet name or address")
	fmt.Println("    -amount [amount]            - Amount to send")
	fmt.Println("  show-balance [wallet-name|address] - Show balance")
	fmt.Println("  show-blockchain               - Show the entire blockchain")
	fmt.Println("  show-block [index]            - Show a specific block")
	fmt.Println("  help                          - Show this help message")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  go run ./cmd/cli create-wallet alice")
	fmt.Println("  go run ./cmd/cli send alice bob 10.5")
	fmt.Println("  go run ./cmd/cli show-balance alice")
	fmt.Println("")
}

func createWallet() {
	var name string
	if len(os.Args) >= 3 {
		name = os.Args[2]
	} else {
		name = "wallet"
	}

	// Create new wallet
	w, err := wallet.NewWallet()
	if err != nil {
		log.Fatalf("Failed to create wallet: %v", err)
	}

	// Export wallet to file
	walletData, err := w.ExportWallet()
	if err != nil {
		log.Fatalf("Failed to export wallet: %v", err)
	}

	filename := fmt.Sprintf("%s.json", name)
	if err := os.WriteFile(filename, walletData, 0600); err != nil {
		log.Fatalf("Failed to save wallet file: %v", err)
	}

	fmt.Printf("Wallet created successfully!\n")
	fmt.Printf("Wallet file: %s\n", filename)
	fmt.Printf("Address: %x\n", w.GetAddress())
	fmt.Println("Keep your wallet file safe - it contains your private key!")
}

func createTransaction() {
	var (
		fromWallet = flag.String("from", "", "Wallet file of sender")
		toAddress  = flag.String("to", "", "Receiver address (hex)")
		amount     = flag.Float64("amount", 0, "Amount to send")
	)

	// Parse flags starting from index 2 (skip "cli" and "create-transaction")
	flag.CommandLine.Parse(os.Args[2:])

	if *fromWallet == "" || *toAddress == "" || *amount <= 0 {
		fmt.Println("Error: Missing required parameters")
		fmt.Println("Usage: create-transaction -from <wallet-file> -to <address> -amount <amount>")
		os.Exit(1)
	}

	// Load sender wallet from file
	senderWallet, err := loadWalletFromFile(*fromWallet)
	if err != nil {
		log.Fatalf("Failed to load sender wallet: %v", err)
	}

	// Parse receiver address
	receiverAddr, err := hex.DecodeString(*toAddress)
	if err != nil {
		log.Fatalf("Invalid receiver address: %v", err)
	}

	// Create transaction
	tx := blockchain.NewTransaction(
		senderWallet.GetAddress(),
		receiverAddr,
		*amount,
	)

	// Sign transaction
	if err := tx.SignTransaction(senderWallet.PrivateKey); err != nil {
		log.Fatalf("Failed to sign transaction: %v", err)
	}

	// Print transaction details
	fmt.Println("Transaction created successfully!")
	fmt.Printf("Transaction Hash: %x\n", tx.TxHash)
	fmt.Printf("From: %x\n", tx.Sender)
	fmt.Printf("To: %x\n", tx.Receiver)
	fmt.Printf("Amount: %.2f\n", tx.Amount)
	fmt.Printf("Timestamp: %d\n", tx.Timestamp)
	fmt.Printf("Signature: %x\n", tx.Signature)

	// Send transaction to blockchain nodes
	fmt.Println("\nSending transaction to blockchain network...")
	if err := sendTransactionToNodes(tx); err != nil {
		log.Printf("Warning: Failed to send transaction to some nodes: %v", err)
		fmt.Println("Transaction created locally but may not be in the network.")
	} else {
		fmt.Println("✅ Transaction sent to blockchain network successfully!")
		fmt.Println("It will be included in the next block.")
	}
}

// loadWalletFromFile loads a wallet from a JSON file
func loadWalletFromFile(filename string) (*wallet.Wallet, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read wallet file: %v", err)
	}

	w := &wallet.Wallet{}
	if err := w.ImportWallet(data); err != nil {
		return nil, fmt.Errorf("failed to import wallet: %v", err)
	}

	return w, nil
}

// sendTransactionToNodes sends transaction to all available blockchain nodes
func sendTransactionToNodes(tx *blockchain.Transaction) error {
	message := &TransactionMessage{Transaction: tx}
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %v", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	successCount := 0
	
	for _, nodeURL := range defaultNodes {
		if err := sendTransactionToNode(client, nodeURL, data); err != nil {
			log.Printf("Failed to send to %s: %v", nodeURL, err)
		} else {
			fmt.Printf("✅ Sent to node: %s\n", nodeURL)
			successCount++
		}
	}

	if successCount == 0 {
		return fmt.Errorf("failed to send transaction to any node")
	}

	fmt.Printf("Transaction sent to %d/%d nodes\n", successCount, len(defaultNodes))
	return nil
}

// sendTransactionToNode sends transaction to a specific node
func sendTransactionToNode(client *http.Client, nodeURL string, data []byte) error {
	url := fmt.Sprintf("%s/transaction", nodeURL)
	
	resp, err := client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("node returned status: %d", resp.StatusCode)
	}

	return nil
}

// ⭐ NEW SIMPLIFIED SEND COMMAND
func sendTransaction() {
	if len(os.Args) < 5 {
		fmt.Println("Usage: send <from> <to> <amount>")
		fmt.Println("Example: send alice bob 10.5")
		os.Exit(1)
	}

	fromName := os.Args[2]
	toName := os.Args[3]
	amountStr := os.Args[4]

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		fmt.Printf("Invalid amount: %s\n", amountStr)
		os.Exit(1)
	}

	// Load sender wallet
	senderWallet, err := loadWalletByName(fromName)
	if err != nil {
		log.Fatalf("Failed to load sender wallet '%s': %v", fromName, err)
	}

	// Resolve receiver address
	receiverAddr, err := resolveAddress(toName)
	if err != nil {
		log.Fatalf("Failed to resolve receiver '%s': %v", toName, err)
	}

	// Create and send transaction
	if err := createAndSendTransaction(senderWallet, receiverAddr, amount); err != nil {
		log.Fatalf("Failed to send transaction: %v", err)
	}

	fmt.Printf("✅ Sent %.2f from %s to %s\n", amount, fromName, toName)
}

// loadWalletByName loads wallet by name (with or without .json extension)
func loadWalletByName(name string) (*wallet.Wallet, error) {
	// Try with .json extension first
	filename := name
	if !strings.HasSuffix(name, ".json") {
		filename = name + ".json"
	}

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("wallet file '%s' not found", filename)
	}

	return wallet.LoadWalletFromFile(filename)
}

// resolveAddress resolves wallet name or hex address to bytes
func resolveAddress(nameOrAddress string) ([]byte, error) {
	// Try to load as wallet name first
	if wallet, err := loadWalletByName(nameOrAddress); err == nil {
		return wallet.GetAddress(), nil
	}

	// Try to parse as hex address
	if len(nameOrAddress) > 10 { // Assume it's an address if long enough
		return hex.DecodeString(nameOrAddress)
	}

	return nil, fmt.Errorf("could not resolve '%s' as wallet name or address", nameOrAddress)
}

// createAndSendTransaction creates and sends a transaction
func createAndSendTransaction(senderWallet *wallet.Wallet, receiverAddr []byte, amount float64) error {
	// Create transaction
	tx := blockchain.NewTransaction(
		senderWallet.GetAddress(),
		receiverAddr,
		amount,
	)

	// Sign transaction
	if err := tx.SignTransaction(senderWallet.PrivateKey); err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Print transaction details
	fmt.Printf("📝 Transaction Details:\n")
	fmt.Printf("   Hash: %s\n", shortHash(tx.TxHash))
	fmt.Printf("   From: %s\n", shortAddress(tx.Sender))
	fmt.Printf("   To: %s\n", shortAddress(tx.Receiver))
	fmt.Printf("   Amount: %.2f\n", tx.Amount)

	// Send to network
	fmt.Println("\n📡 Sending to blockchain network...")
	return sendTransactionToNodes(tx)
}

// shortHash returns shortened hash for display
func shortHash(hash []byte) string {
	hashStr := fmt.Sprintf("%x", hash)
	if len(hashStr) > 16 {
		return hashStr[:8] + "..." + hashStr[len(hashStr)-8:]
	}
	return hashStr
}

// shortAddress returns shortened address for display
func shortAddress(addr []byte) string {
	addrStr := fmt.Sprintf("%x", addr)
	if len(addrStr) > 16 {
		return addrStr[:8] + "..." + addrStr[len(addrStr)-8:]
	}
	return addrStr
}

// Enhanced show-balance to get balance from network nodes
func showBalance() {
	if len(os.Args) < 3 {
		fmt.Println("Error: Missing address/wallet name parameter")
		fmt.Println("Usage: show-balance <wallet-name|address>")
		os.Exit(1)
	}

	nameOrAddress := os.Args[2]
	
	// Resolve to address
	address, err := resolveAddress(nameOrAddress)
	if err != nil {
		log.Fatalf("Failed to resolve '%s': %v", nameOrAddress, err)
	}

	// Get blockchain from network nodes
	client := &http.Client{Timeout: 5 * time.Second}
	var balance float64 = 0.0
	
	for _, nodeURL := range defaultNodes {
		if response := getBlockchainFromNode(client, nodeURL); response != nil {
			fmt.Printf("📡 Connected to node: %s\n", nodeURL)
			balance = calculateBalance(response.Blocks, address)
			break
		}
	}

	fmt.Printf("👤 %s\n", nameOrAddress)
	fmt.Printf("📍 Address: %s\n", shortAddress(address))
	fmt.Printf("💰 Balance: %.2f\n", balance)
}

// calculateBalance calculates balance from blockchain blocks
func calculateBalance(blocks []*blockchain.Block, address []byte) float64 {
	balance := 0.0
	
	for _, block := range blocks {
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

func showBlockchain() {
	// Try to get blockchain from running nodes
	client := &http.Client{Timeout: 5 * time.Second}
	
	for _, nodeURL := range defaultNodes {
		if response := getBlockchainFromNode(client, nodeURL); response != nil {
			fmt.Printf("📡 Connected to node: %s\n", nodeURL)
			fmt.Printf("📊 Total blocks in network: %d\n", response.Total)
			displayBlocks(response.Blocks, response.IsValid)
			return
		}
	}

	// Fallback to local blockchain if no nodes available
	fmt.Println("⚠️  No blockchain nodes available, showing local blockchain...")
	bc := blockchain.NewBlockchain()
	blocks := bc.GetAllBlocks()
	displayBlocks(blocks, bc.IsValidChain())
}

// getBlockchainFromNode attempts to get blockchain data from a node
func getBlockchainFromNode(client *http.Client, nodeURL string) *BlockchainResponse {
	url := fmt.Sprintf("%s/blockchain", nodeURL)
	resp, err := client.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var response BlockchainResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil
	}

	return &response
}

// displayBlocks displays blockchain information
func displayBlocks(blocks []*blockchain.Block, isValid bool) {
	fmt.Println("Blockchain Information:")
	fmt.Printf("Total Blocks: %d\n", len(blocks))
	fmt.Printf("Is Valid: %t\n", isValid)
	fmt.Println("----------------------------------------")

	for _, block := range blocks {
		fmt.Printf("Block %d:\n", block.Index)
		fmt.Printf("  Hash: %x\n", block.CurrentBlockHash)
		fmt.Printf("  Previous Hash: %x\n", block.PreviousBlockHash)
		fmt.Printf("  Merkle Root: %x\n", block.MerkleRoot)
		fmt.Printf("  Timestamp: %d\n", block.Timestamp)
		fmt.Printf("  Transactions: %d\n", len(block.Transactions))
		
		for i, tx := range block.Transactions {
			fmt.Printf("    Tx %d: %x -> %x (%.2f)\n", 
				i, tx.Sender, tx.Receiver, tx.Amount)
		}
		fmt.Println("----------------------------------------")
	}
}

func showBlock() {
	if len(os.Args) < 3 {
		fmt.Println("Error: Missing block index parameter")
		fmt.Println("Usage: show-block <index>")
		os.Exit(1)
	}

	indexStr := os.Args[2]
	index, err := strconv.ParseInt(indexStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid block index: %v", err)
	}

	bc := blockchain.NewBlockchain()
	block := bc.GetBlock(index)

	if block == nil {
		fmt.Printf("Block %d not found\n", index)
		os.Exit(1)
	}

	// Convert block to JSON for pretty printing
	blockJSON, err := block.ToJSON()
	if err != nil {
		log.Fatalf("Failed to convert block to JSON: %v", err)
	}

	fmt.Printf("Block %d:\n", index)
	fmt.Println(string(blockJSON))
}

// Demo function to create sample transactions
func createSampleTransactions() {
	fmt.Println("Creating sample wallets and transactions...")

	// Create Alice's wallet
	alice, err := wallet.NewWallet()
	if err != nil {
		log.Fatalf("Failed to create Alice's wallet: %v", err)
	}

	// Create Bob's wallet
	bob, err := wallet.NewWallet()
	if err != nil {
		log.Fatalf("Failed to create Bob's wallet: %v", err)
	}

	fmt.Printf("Alice's address: %x\n", alice.GetAddress())
	fmt.Printf("Bob's address: %x\n", bob.GetAddress())

	// Create transaction from Alice to Bob
	tx := blockchain.NewTransaction(
		alice.GetAddress(),
		bob.GetAddress(),
		50.0,
	)

	// Sign transaction
	if err := tx.SignTransaction(alice.PrivateKey); err != nil {
		log.Fatalf("Failed to sign transaction: %v", err)
	}

	fmt.Printf("Transaction created: %s\n", tx.String())
	fmt.Printf("Transaction hash: %x\n", tx.TxHash)
	fmt.Printf("Signature valid: %t\n", tx.VerifyTransaction(alice.PublicKey))
} 