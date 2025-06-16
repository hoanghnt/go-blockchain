# Golang Blockchain Implementation

A complete blockchain implementation in Go featuring distributed consensus, ECDSA digital signatures, and automatic node recovery capabilities.

## 🎯 Project Overview

This project implements a production-ready blockchain system with the following key features:

- **ECDSA Digital Signatures**: Secure transaction signing using P-256 elliptic curve
- **Distributed Consensus**: 3-node network with leader-follower consensus and majority voting
- **Persistent Storage**: LevelDB with Merkle tree verification for data integrity
- **Node Recovery**: Automatic synchronization when nodes rejoin the network
- **Docker Deployment**: Multi-container setup with persistent volumes
- **CLI Interface**: User-friendly command-line tools for wallet and transaction management
- **REST API**: Complete API endpoints for external integration

## 🏗️ Architecture

### Core Components

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│      Node 1     │    │      Node 2     │    │      Node 3     │
│   (Leader)      │    │   (Follower)    │    │   (Follower)    │
├─────────────────┤    ├─────────────────┤    ├─────────────────┤
│   Consensus     │◄──►│   Consensus     │◄──►│   Consensus     │
│   Engine        │    │   Engine        │    │   Engine        │
├─────────────────┤    ├─────────────────┤    ├─────────────────┤
│   LevelDB       │    │   LevelDB       │    │   LevelDB       │
│   Storage       │    │   Storage       │    │   Storage       │
├─────────────────┤    ├─────────────────┤    ├─────────────────┤
│   gRPC/HTTP     │    │   gRPC/HTTP     │    │   gRPC/HTTP     │
│   API Server    │    │   API Server    │    │   API Server    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Package Structure

- `pkg/blockchain`: Block and transaction structures, Merkle tree implementation
- `pkg/wallet`: ECDSA key management and transaction signing
- `pkg/consensus`: Leader-follower consensus mechanism with voting
- `pkg/storage`: LevelDB integration and data persistence
- `pkg/p2p`: Node-to-node communication via gRPC
- `pkg/sync`: Node recovery and blockchain synchronization
- `cmd/node`: Blockchain node main application
- `cmd/cli`: Command-line interface tools

## 🚀 Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.21+ (for development)

### 1. Launch the Blockchain Network

```bash
# Start 3-node blockchain network
docker-compose up -d

# Check node status
docker-compose ps
```

### 2. Create User Wallets

```bash
# Enter node container
docker-compose exec node1 sh

# Create wallet for Alice
./go-blockchain-cli create-wallet Alice

# Create wallet for Bob  
./go-blockchain-cli create-wallet Bob

# Exit container
exit
```

### 3. Send Your First Transaction

```bash
# Enter node container
docker-compose exec node1 sh

# Send 100 coins from Alice to Bob
./go-blockchain-cli send Alice Bob 100

# Check blockchain status
./go-blockchain-cli show-blockchain

# Exit container
exit
```

## 📋 CLI Commands Reference

**Note**: All CLI commands must be run inside the container. Use `docker-compose exec node1 sh` to enter the container first.

### Available Commands

```bash
# Create a new wallet
./go-blockchain-cli create-wallet <name>

# Send transaction (simplified)
./go-blockchain-cli send <from> <to> <amount>

# Create transaction (advanced)
./go-blockchain-cli create-transaction -from <wallet> -to <address> -amount <amount>

# Show entire blockchain
./go-blockchain-cli show-blockchain

# Show specific block
./go-blockchain-cli show-block <index>

# Show help
./go-blockchain-cli help
```

### Alternative: Using CLI Container

```bash
# Start CLI container
docker-compose --profile tools run --rm cli create-wallet Alice

# Or run specific commands
docker-compose --profile tools run --rm cli send Alice Bob 10.5
docker-compose --profile tools run --rm cli show-blockchain
```

## 🛠️ Development Setup

### Local Development

```bash
# Clone repository
git clone <repository-url>
cd golang-blockchain

# Install dependencies
go mod download

# Run tests
go test ./...

# Build binaries
go build -o go-blockchain-node ./cmd/node
go build -o go-blockchain-cli ./cmd/cli
```

### Running Without Docker

```bash
go run ./cmd/cli create-wallet Alice
go run ./cmd/cli send Alice Bob 100
go run ./cmd/cli show-blockchain
```

## 🔧 Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `NODE_ID` | Unique node identifier | `node1` |
| `IS_LEADER` | Whether node is consensus leader | `false` |
| `HTTP_PORT` | HTTP API server port | `8080` |
| `GRPC_PORT` | gRPC communication port | `50051` |
| `PEERS` | Comma-separated peer addresses | `""` |
| `DATA_DIR` | Data storage directory | `./data` |
| `LOG_LEVEL` | Logging level (debug/info/warn/error) | `info` |

### Docker Compose Configuration

The `docker-compose.yml` file defines:
- **3 validator nodes** with persistent storage
- **Network isolation** for secure communication
- **Port mapping** for external access
- **Volume mounts** for data persistence

## 🔍 Troubleshooting

### Common Issues

**1. Connection Refused Errors**
```bash
# Check if all nodes are running
docker-compose ps

# View node logs
docker-compose logs node1
docker-compose logs node2  
docker-compose logs node3
```

**2. Transaction Takes Long Time**
- Normal behavior: Transactions require consensus (15-20 seconds)
- Check blockchain status: `docker-compose exec node1 sh` then `./go-blockchain-cli show-blockchain`

**3. Node Recovery Issues**
```bash
# Restart specific node
docker-compose restart node2

# Check blockchain status
docker-compose exec node2 sh
./go-blockchain-cli show-blockchain
exit
```

**4. Storage Issues**
```bash
# Reset blockchain data (⚠️ This deletes all data)
docker-compose down -v
docker-compose up -d
```

### Debugging Commands

```bash
# View detailed logs
docker-compose logs -f node1

# Execute commands inside container
docker-compose exec node1 sh

# Check container networking
docker network ls
docker network inspect <network_name>

# Monitor resource usage
docker-compose exec node1 top
```

## 🧪 Testing Node Recovery

To test the automatic node recovery mechanism:

```bash
# 1. Start network and create initial state
docker-compose up -d

# Enter container and create wallets
docker-compose exec node1 sh
./go-blockchain-cli create-wallet Alice
./go-blockchain-cli create-wallet Bob
./go-blockchain-cli send Alice Bob 50
exit

# 2. Stop a node deliberately
docker-compose stop node2

# 3. Continue making transactions (will work with 2/3 nodes)
docker-compose exec node1 sh
./go-blockchain-cli send Bob Alice 25
exit

# 4. Restart the stopped node
docker-compose start node2

# 5. Verify automatic recovery
docker-compose exec node2 sh
./go-blockchain-cli show-blockchain
exit
```

The stopped node will automatically:
- Detect missing blocks upon restart
- Synchronize with active peers
- Resume participating in consensus

## 🔐 Security Features

### ECDSA Implementation

- **Curve**: NIST P-256 (secp256r1)
- **Key Generation**: Cryptographically secure random number generator
- **Signature Format**: ASN.1 DER encoding
- **Hash Function**: SHA-256 for transaction hashing

### Consensus Security

- **Byzantine Fault Tolerance**: Handles up to 1 failed node in 3-node setup
- **Majority Voting**: Requires 2/3 agreement for block acceptance
- **Block Validation**: Complete verification of transactions and signatures
- **Chain Integrity**: Cryptographic linking via previous block hashes

### Data Integrity

- **Merkle Trees**: Efficient verification of transaction sets
- **Block Hashing**: Tamper-evident block structure
- **Signature Verification**: All transactions cryptographically verified
- **Persistent Storage**: Reliable data storage with LevelDB

## 📊 Performance Characteristics

- **Block Time**: ~15-20 seconds (configurable)
- **Transaction Throughput**: Limited by consensus mechanism
- **Storage**: Efficient key-value storage with LevelDB
- **Network**: gRPC for high-performance inter-node communication
- **Recovery Time**: Automatic, typically under 30 seconds

## 🤝 Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🎓 Educational Purpose

This blockchain implementation demonstrates:

- **Distributed Systems**: Consensus mechanisms and fault tolerance
- **Cryptography**: Digital signatures and hash functions
- **Networking**: Peer-to-peer communication protocols
- **Database**: Persistent storage and data integrity
- **Containerization**: Docker deployment and orchestration

Perfect for learning blockchain fundamentals and distributed system concepts!

---

**Built with ❤️ in Go** | **Powered by Docker** | **Secured by ECDSA**
