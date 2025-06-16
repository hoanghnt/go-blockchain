# Makefile for go-blockchain project

.PHONY: proto build run clean test docker-build docker-up docker-down

# Generate protobuf and gRPC code
proto:
	@echo "Generating protobuf code..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		pkg/p2p/p2p.proto

# Build the project
build:
	@echo "Building the project..."
	go build -o bin/go-blockchain ./cmd/node
	go build -o bin/go-blockchain-cli ./cmd/cli

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f pkg/p2p/*.pb.go

# Build Docker images
docker-build:
	@echo "Building Docker images..."
	docker build -t go-blockchain .

# Run with Docker Compose
docker-up:
	@echo "Starting blockchain network..."
	docker-compose up -d

# Stop Docker Compose
docker-down:
	@echo "Stopping blockchain network..."
	docker-compose down

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

# Run single node for testing
run-node:
	@echo "Running single node..."
	go run ./cmd/node

# Run CLI
run-cli:
	@echo "Running CLI..."
	go run ./cmd/cli

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run

# Generate, build and test
all: proto build test

# Create necessary directories
init:
	@echo "Creating directories..."
	mkdir -p bin data logs 