#!/bin/bash
# Run YOLOv8 Object Detection Worker

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Default values
CAMERAS="${CAMERAS:-cam_05432c86af8537a6}"
NATS_URL="${NATS_URL:-nats://localhost:4222}"
PLATFORM_URL="${PLATFORM_URL:-http://localhost:3001}"
CONFIDENCE="${CONFIDENCE:-0.5}"

echo "🔍 YOLOv8 Object Detection Worker"
echo "================================"
echo "Cameras: $CAMERAS"
echo "NATS: $NATS_URL"
echo "Platform: $PLATFORM_URL"
echo "Confidence: $CONFIDENCE"
echo ""

# Check if virtual environment exists
if [ ! -d "$SCRIPT_DIR/venv" ]; then
    echo "Creating virtual environment..."
    python3 -m venv "$SCRIPT_DIR/venv"
    source "$SCRIPT_DIR/venv/bin/activate"
    echo "Installing dependencies..."
    pip install -r "$SCRIPT_DIR/requirements.txt"
else
    source "$SCRIPT_DIR/venv/bin/activate"
fi

# Run the worker
python3 "$SCRIPT_DIR/main.py" \
    --cameras "$CAMERAS" \
    --nats-url "$NATS_URL" \
    --platform-url "$PLATFORM_URL" \
    --confidence "$CONFIDENCE"

