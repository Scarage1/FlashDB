#!/bin/bash
# =============================================================================
# FlashDB Deployment Script for DigitalOcean
# =============================================================================
# Run this script on a fresh DigitalOcean droplet to set up FlashDB
# Usage: curl -sSL https://raw.githubusercontent.com/YOUR_USERNAME/flashdb/main/scripts/deploy.sh | bash

set -e

echo "🚀 FlashDB Deployment Script"
echo "=============================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# -----------------------------------------------------------------------------
# Step 1: Update system
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[1/6] Updating system...${NC}"
apt-get update && apt-get upgrade -y

# -----------------------------------------------------------------------------
# Step 2: Install Docker
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[2/6] Installing Docker...${NC}"
if ! command -v docker &> /dev/null; then
    curl -fsSL https://get.docker.com -o get-docker.sh
    sh get-docker.sh
    rm get-docker.sh
    systemctl enable docker
    systemctl start docker
    echo -e "${GREEN}✓ Docker installed${NC}"
else
    echo -e "${GREEN}✓ Docker already installed${NC}"
fi

# -----------------------------------------------------------------------------
# Step 3: Install Docker Compose
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[3/6] Installing Docker Compose...${NC}"
if ! command -v docker-compose &> /dev/null; then
    curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
    chmod +x /usr/local/bin/docker-compose
    echo -e "${GREEN}✓ Docker Compose installed${NC}"
else
    echo -e "${GREEN}✓ Docker Compose already installed${NC}"
fi

# -----------------------------------------------------------------------------
# Step 4: Create FlashDB directory
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[4/6] Setting up FlashDB directory...${NC}"
mkdir -p /opt/flashdb
cd /opt/flashdb

# -----------------------------------------------------------------------------
# Step 5: Create docker-compose.yml
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[5/6] Creating docker-compose.yml...${NC}"
cat > docker-compose.yml << 'EOF'
version: '3.8'

services:
  flashdb:
    image: ${DOCKER_IMAGE:-flashdb/flashdb}:latest
    container_name: flashdb-server
    restart: unless-stopped
    ports:
      - "6379:6379"
      - "8080:8080"
    volumes:
      - flashdb-data:/data
    environment:
      - FLASHDB_PASSWORD=${FLASHDB_PASSWORD:-}
    command: >
      -addr :6379
      -data /data
      -web
      -webaddr :8080
      ${FLASHDB_PASSWORD:+-requirepass $FLASHDB_PASSWORD}
    healthcheck:
      test: ["CMD", "sh", "-c", "echo PING | nc -w 1 localhost 6379 | grep -q PONG"]
      interval: 30s
      timeout: 5s
      retries: 3

volumes:
  flashdb-data:
EOF

echo -e "${GREEN}✓ docker-compose.yml created${NC}"

# -----------------------------------------------------------------------------
# Step 6: Create .env file
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[6/6] Creating environment file...${NC}"
if [ ! -f .env ]; then
    cat > .env << EOF
# FlashDB Configuration
DOCKER_IMAGE=flashdb/flashdb
FLASHDB_PASSWORD=

# Uncomment and set a password for production:
# FLASHDB_PASSWORD=your-secure-password-here
EOF
    echo -e "${GREEN}✓ .env file created${NC}"
else
    echo -e "${GREEN}✓ .env file already exists${NC}"
fi

# -----------------------------------------------------------------------------
# Setup firewall
# -----------------------------------------------------------------------------
echo -e "${YELLOW}Setting up firewall...${NC}"
if command -v ufw &> /dev/null; then
    ufw allow 22/tcp    # SSH
    ufw allow 6379/tcp  # Redis
    ufw allow 8080/tcp  # HTTP API
    ufw allow 80/tcp    # HTTP (for SSL)
    ufw allow 443/tcp   # HTTPS
    ufw --force enable
    echo -e "${GREEN}✓ Firewall configured${NC}"
fi

# -----------------------------------------------------------------------------
# Done!
# -----------------------------------------------------------------------------
echo ""
echo -e "${GREEN}=============================="
echo "✅ FlashDB Setup Complete!"
echo "==============================${NC}"
echo ""
echo "Next steps:"
echo "1. Edit /opt/flashdb/.env to set your password"
echo "2. Run: cd /opt/flashdb && docker-compose up -d"
echo "3. Test: redis-cli -h localhost -p 6379 PING"
echo ""
echo "Ports:"
echo "  - 6379: Redis protocol (RESP)"
echo "  - 8080: HTTP API"
echo ""
echo -e "${YELLOW}⚠️  Remember to set FLASHDB_PASSWORD in .env for production!${NC}"
