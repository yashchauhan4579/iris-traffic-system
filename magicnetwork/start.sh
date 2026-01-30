#!/bin/bash
# Start MagicNetwork VPN Server
# Must be run as root

set -e

cd "$(dirname "$0")"

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "❌ Please run as root (sudo)"
    exit 1
fi

# Load environment if exists
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# Default values
API_PORT=${API_PORT:-8080}
WG_PORT=${WG_PORT:-51820}
WG_ADDRESS=${WG_ADDRESS:-"10.10.0.1/24"}
DATA_DIR=${DATA_DIR:-"./data"}

# Build if needed
if [ ! -f ./magicnetwork ]; then
    echo "📦 Building MagicNetwork..."
    go build -o magicnetwork ./cmd/magicnetwork
fi

# Run
echo "🚀 Starting MagicNetwork..."
./magicnetwork \
    --port "$API_PORT" \
    --wg-port "$WG_PORT" \
    --address "$WG_ADDRESS" \
    --data "$DATA_DIR" \
    ${MAGICNETWORK_API_KEY:+--api-key "$MAGICNETWORK_API_KEY"}

