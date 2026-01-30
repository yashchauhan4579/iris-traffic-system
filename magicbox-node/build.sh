#!/bin/bash
# Build script for MagicBox Node

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
VERSION="${VERSION:-1.0.0}"
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-arm64}"
OUTPUT_DIR="${OUTPUT_DIR:-./build}"
BINARY_NAME="magicbox"
NGINX_WWW_DIR="${NGINX_WWW_DIR:-/var/www/html}"
COPY_TO_NGINX="${COPY_TO_NGINX:-true}"

# Parse command line arguments
CLEAN=false
RELEASE=false
ARCH=""
NO_COPY=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --clean)
            CLEAN=true
            shift
            ;;
        --release)
            RELEASE=true
            shift
            ;;
        --arch)
            ARCH="$2"
            shift 2
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        --no-copy)
            NO_COPY=true
            shift
            ;;
        --nginx-dir)
            NGINX_WWW_DIR="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --clean       Clean build directory before building"
            echo "  --release     Build release binaries for multiple architectures"
            echo "  --arch ARCH   Build for specific architecture (amd64, arm64)"
            echo "  --version VER Set version string (default: 1.0.0)"
            echo "  --no-copy     Don't copy binary to nginx www directory"
            echo "  --nginx-dir   Nginx www directory (default: /var/www/html)"
            echo "  -h, --help    Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  VERSION       Version string"
            echo "  GOOS          Target OS (default: linux)"
            echo "  GOARCH        Target architecture (default: arm64)"
            echo "  OUTPUT_DIR    Output directory (default: ./build)"
            echo "  GO_BIN        Path to Go binary (default: /usr/local/go/bin/go)"
            echo "  NGINX_WWW_DIR Nginx www directory (default: /var/www/html)"
            echo "  COPY_TO_NGINX Copy to nginx after build (default: true)"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Clean build directory if requested
if [ "$CLEAN" = true ]; then
    echo -e "${YELLOW}Cleaning build directory...${NC}"
    rm -rf "$OUTPUT_DIR"
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Function to build for a specific architecture
build_arch() {
    local os=$1
    local arch=$2
    local output_name="${BINARY_NAME}"
    
    if [ "$os" != "linux" ] || [ "$arch" != "arm64" ]; then
        output_name="${BINARY_NAME}-${os}-${arch}"
    fi
    
    local output_path="${OUTPUT_DIR}/${output_name}"
    
    echo -e "${GREEN}Building for ${os}/${arch}...${NC}"
    
    GOOS="$os" GOARCH="$arch" "$GO_BIN" build \
        -ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -s -w" \
        -trimpath \
        -o "$output_path" \
        ./cmd/magicbox
    
    # Make it executable
    chmod +x "$output_path"
    
    # Show binary info
    if command -v file > /dev/null; then
        echo -e "  ${GREEN}✓${NC} Built: $output_path"
        file "$output_path"
    else
        echo -e "  ${GREEN}✓${NC} Built: $output_path"
    fi
    
    # Show binary size
    if [ -f "$output_path" ]; then
        local size=$(du -h "$output_path" | cut -f1)
        echo -e "  ${GREEN}✓${NC} Size: $size"
    fi
    
    # Copy to nginx www directory if requested (only for default linux/arm64 build)
    if [ "$NO_COPY" = false ] && [ "$COPY_TO_NGINX" = "true" ] && [ "$os" = "linux" ] && [ "$arch" = "arm64" ]; then
        if [ -d "$NGINX_WWW_DIR" ]; then
            echo -e "  ${GREEN}Copying to nginx www directory...${NC}"
            if sudo cp "$output_path" "$NGINX_WWW_DIR/${BINARY_NAME}" 2>/dev/null; then
                sudo chmod +x "$NGINX_WWW_DIR/${BINARY_NAME}"
                echo -e "  ${GREEN}✓${NC} Copied to: $NGINX_WWW_DIR/${BINARY_NAME}"
            else
                echo -e "  ${YELLOW}⚠${NC} Failed to copy to nginx directory (may need sudo)"
            fi
        else
            echo -e "  ${YELLOW}⚠${NC} Nginx www directory not found: $NGINX_WWW_DIR"
        fi
    fi
}

# Use full path to Go binary
GO_BIN="${GO_BIN:-/usr/local/go/bin/go}"

# Check if Go is installed
if [ ! -f "$GO_BIN" ]; then
    echo -e "${RED}Error: Go not found at $GO_BIN${NC}"
    echo -e "${YELLOW}You can set GO_BIN environment variable to specify Go path${NC}"
    exit 1
fi

# Show Go version
echo -e "${GREEN}Go version:${NC}"
"$GO_BIN" version

# Check if we're in the right directory
if [ ! -f "go.mod" ]; then
    echo -e "${RED}Error: go.mod not found. Please run this script from the magicbox-node directory.${NC}"
    exit 1
fi

# Download dependencies
echo -e "${GREEN}Downloading dependencies...${NC}"
"$GO_BIN" mod download
"$GO_BIN" mod tidy

# Build based on options
if [ "$RELEASE" = true ]; then
    echo -e "${GREEN}Building release binaries for multiple architectures...${NC}"
    echo ""
    
    # Build for multiple architectures
    build_arch "linux" "amd64"
    build_arch "linux" "arm64"
    
    echo ""
    echo -e "${GREEN}✓ Release build complete!${NC}"
    echo -e "Binaries are in: ${OUTPUT_DIR}/"
    ls -lh "$OUTPUT_DIR" | grep "$BINARY_NAME"
    
elif [ -n "$ARCH" ]; then
    echo -e "${GREEN}Building for linux/${ARCH}...${NC}"
    build_arch "linux" "$ARCH"
    
else
    # Default: build for current platform
    echo -e "${GREEN}Building for current platform (${GOOS}/${GOARCH})...${NC}"
    build_arch "$GOOS" "$GOARCH"
    
    echo ""
    echo -e "${GREEN}✓ Build complete!${NC}"
    echo -e "Binary: ${OUTPUT_DIR}/${BINARY_NAME}"
    echo ""
    echo -e "To run:"
    echo -e "  ${OUTPUT_DIR}/${BINARY_NAME} -config ./data/config.json -data ./data -port 8080"
fi

