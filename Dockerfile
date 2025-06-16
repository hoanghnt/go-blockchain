# Build stage
FROM golang:1.22-alpine AS builder

# Set working directory
WORKDIR /app

# Install make for build process
RUN apk add --no-cache make

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o /go-blockchain ./cmd/node
RUN go build -o /go-blockchain-cli ./cmd/cli

# Run stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create app directory
WORKDIR /app

# Copy binaries from builder stage
COPY --from=builder /go-blockchain .
COPY --from=builder /go-blockchain-cli .

# Create data directory for LevelDB
RUN mkdir -p /app/data

# Create non-root user for security
RUN addgroup -g 1001 -S blockchain && \
    adduser -u 1001 -S blockchain -G blockchain

# Change ownership of app directory
RUN chown -R blockchain:blockchain /app

# Switch to non-root user
USER blockchain

# Expose port for gRPC communication
EXPOSE 50051

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD test -f /app/data/blockchain.db || exit 1

# Default command
CMD ["./go-blockchain"] 