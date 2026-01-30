#!/bin/bash
set -e

# MagicBox Node Installation Script
# For NVIDIA Jetson and Linux systems

INSTALL_DIR="/opt/magicbox"
CONFIG_DIR="/etc/magicbox"
DATA_DIR="/var/lib/magicbox"
USER="magicbox"
GROUP="magicbox"

echo "🚀 Installing MagicBox Node..."

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root (sudo)"
    exit 1
fi

# Create user and group if they don't exist
if ! id "$USER" &>/dev/null; then
    echo "Creating user $USER..."
    useradd -r -s /bin/false -d "$INSTALL_DIR" "$USER"
fi

# Create directories
echo "Creating directories..."
mkdir -p "$INSTALL_DIR/bin"
mkdir -p "$CONFIG_DIR"
mkdir -p "$DATA_DIR/events/pending"
mkdir -p "$DATA_DIR/events/sent"
mkdir -p "$DATA_DIR/events/failed"
mkdir -p "$DATA_DIR/images"
mkdir -p "$DATA_DIR/logs"

# Copy binary
if [ -f "./magicbox" ]; then
    echo "Installing binary..."
    cp ./magicbox "$INSTALL_DIR/bin/"
    chmod +x "$INSTALL_DIR/bin/magicbox"
elif [ -f "./build/magicbox-arm64" ]; then
    echo "Installing ARM64 binary..."
    cp ./build/magicbox-arm64 "$INSTALL_DIR/bin/magicbox"
    chmod +x "$INSTALL_DIR/bin/magicbox"
else
    echo "Error: Binary not found. Run 'make build' or 'make build-jetson' first."
    exit 1
fi

# Create default config if it doesn't exist
if [ ! -f "$CONFIG_DIR/config.json" ]; then
    echo "Creating default config..."
    cat > "$CONFIG_DIR/config.json" << EOF
{
  "nodeName": "$(hostname)",
  "nodeModel": "$(cat /proc/device-tree/model 2>/dev/null || echo 'Linux')",
  "mac": "$(ip link show | grep ether | head -1 | awk '{print $2}')",
  "state": "unconfigured",
  "platform": {},
  "cameras": [],
  "configVersion": 0
}
EOF
fi

# Set permissions
echo "Setting permissions..."
chown -R "$USER:$GROUP" "$INSTALL_DIR"
chown -R "$USER:$GROUP" "$CONFIG_DIR"
chown -R "$USER:$GROUP" "$DATA_DIR"

# Install systemd service
echo "Installing systemd service..."
cp ./scripts/magicbox.service /etc/systemd/system/
systemctl daemon-reload

echo ""
echo "✅ MagicBox Node installed successfully!"
echo ""
echo "Next steps:"
echo "  1. Enable the service:  sudo systemctl enable magicbox"
echo "  2. Start the service:   sudo systemctl start magicbox"
echo "  3. Check status:        sudo systemctl status magicbox"
echo "  4. Access Web UI:       http://$(hostname -I | awk '{print $1}'):8080"
echo ""
echo "Configuration file: $CONFIG_DIR/config.json"
echo "Data directory:     $DATA_DIR"
echo "Logs:               journalctl -u magicbox -f"

