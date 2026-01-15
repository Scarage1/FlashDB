# =============================================================================
# FlashDB Dockerfile - Multi-stage build
# =============================================================================

# Stage 1: Build the Go binary
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=1.0.0" \
    -o /flashdb \
    ./cmd/flashdb

# =============================================================================
# Stage 2: Production image
# =============================================================================
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user for security
RUN addgroup -g 1000 flashdb && \
    adduser -u 1000 -G flashdb -s /bin/sh -D flashdb

# Create data directory
RUN mkdir -p /data && chown -R flashdb:flashdb /data

# Copy binary from builder
COPY --from=builder /flashdb /usr/local/bin/flashdb

# Switch to non-root user
USER flashdb

# Set working directory
WORKDIR /data

# Expose ports
# 6379 - Redis protocol (RESP)
# 8080 - HTTP API
EXPOSE 6379 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD echo "PING" | nc -w 1 localhost 6379 | grep -q "PONG" || exit 1

# Default environment variables
ENV FLASHDB_ADDR=":6379" \
    FLASHDB_DATA="/data" \
    FLASHDB_MAXCLIENTS="10000"

# Entrypoint with environment variable support
ENTRYPOINT ["flashdb"]
CMD ["-addr", ":6379", "-data", "/data", "-web", "-webaddr", ":8080"]
