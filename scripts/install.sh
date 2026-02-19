#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}🦞 ClawMem One-Click Installer${NC}"
echo "=================================="

# 1. Check Root
if [ "$EUID" -ne 0 ]; then 
  echo -e "${RED}Please run as root (sudo ./scripts/install.sh)${NC}"
  exit 1
fi

# 2. Interactive Configuration
echo -e "\n${YELLOW}[Configuration] (Press Enter to use defaults)${NC}"

# Port
read -p "Service Port [8090]: " PORT
PORT=${PORT:-8090}

# Paths
DEFAULT_DB_PATH="/var/lib/clawmem/clawmem.db"
read -p "SQLite DB Path [$DEFAULT_DB_PATH]: " DB_PATH
DB_PATH=${DB_PATH:-$DEFAULT_DB_PATH}

DEFAULT_VECTOR_PATH="/var/lib/clawmem/vectors"
read -p "Vector DB Path [$DEFAULT_VECTOR_PATH]: " VECTOR_DB_PATH
VECTOR_DB_PATH=${VECTOR_DB_PATH:-$DEFAULT_VECTOR_PATH}

# Cloudflare
echo -e "\n${BLUE}--- Embedding Provider (Cloudflare Recommended) ---${NC}"
read -p "Embedding Strategy (cloud_first/local_only) [cloud_first]: " STRATEGY
STRATEGY=${STRATEGY:-cloud_first}

if [ "$STRATEGY" = "cloud_first" ]; then
    echo -e "${YELLOW}Tip: Get these from your Cloudflare Dashboard -> Workers AI${NC}"
    read -p "Cloudflare Account ID: " CF_ACCOUNT_ID
    read -p "Cloudflare API Token: " CF_API_TOKEN
fi

# 3. Build/Install Binary
echo -e "\n${YELLOW}[Building Binary]${NC}"
if command -v go &> /dev/null; then
    echo "Go detected. Building from source..."
    go build -o clawmem ./cmd/server
    mv clawmem /usr/local/bin/clawmem
    echo -e "${GREEN}Binary installed to /usr/local/bin/clawmem${NC}"
else
    echo -e "${RED}Go not found. Please install Go 1.23+ or download the binary release first.${NC}"
    exit 1
fi

# 4. Setup Directories & Config
echo -e "\n${YELLOW}[Setting up Environment]${NC}"
# Ensure directories exist based on user input
mkdir -p "$(dirname "$DB_PATH")"
mkdir -p "$VECTOR_DB_PATH"
mkdir -p /etc/clawmem

CONFIG_FILE="/etc/clawmem/config.env"
cat > "$CONFIG_FILE" <<EOF
PORT=$PORT
DB_PATH=$DB_PATH
VECTOR_DB_PATH=$VECTOR_DB_PATH
EMBEDDING_STRATEGY=$STRATEGY

# Cloudflare Configuration
CF_ACCOUNT_ID=$CF_ACCOUNT_ID
CF_API_TOKEN=$CF_API_TOKEN

# LLM Configuration (Defaults)
DISABLE_LLM_SUMMARY=true
EOF
echo -e "${GREEN}Config saved to $CONFIG_FILE${NC}"

# 5. Setup Systemd
echo -e "\n${YELLOW}[Configuring Systemd]${NC}"
SERVICE_FILE="/etc/systemd/system/clawmem.service"
# Extract working directory from DB path for the service context
WORKING_DIR=$(dirname "$DB_PATH")

cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=ClawMem Memory Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$WORKING_DIR
ExecStart=/usr/local/bin/clawmem
Restart=always
RestartSec=5
EnvironmentFile=/etc/clawmem/config.env

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable clawmem
systemctl restart clawmem

# 6. Final Status
echo -e "\n${GREEN}==================================${NC}"
echo -e "${GREEN}🦞 ClawMem Installed & Started!${NC}"
echo -e "=================================="
echo -e "Status:  $(systemctl is-active clawmem)"
echo -e "Port:    $PORT"
echo -e "DB Path: $DB_PATH"
echo -e "\n${BLUE}Useful Commands:${NC}"
echo -e "  View Logs:    ${YELLOW}journalctl -u clawmem -f${NC}"
echo -e "  Restart:      ${YELLOW}systemctl restart clawmem${NC}"
echo -e "  Edit Config:  ${YELLOW}nano /etc/clawmem/config.env && systemctl restart clawmem${NC}"
echo -e "\nTest it now:"
echo -e "  curl http://localhost:$PORT/health"
