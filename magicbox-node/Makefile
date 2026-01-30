.PHONY: build run clean dev install

# Variables
APP_NAME := magicbox
VERSION := 1.0.0
BUILD_DIR := ./build
MAIN_PATH := ./cmd/magicbox

# Build flags
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(shell date -u '+%Y-%m-%d_%H:%M:%S')"

# Build for current platform
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)
	@echo "Built: $(BUILD_DIR)/$(APP_NAME)"

# Build for Jetson (ARM64)
build-jetson:
	@echo "Building $(APP_NAME) for Jetson (ARM64)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-arm64 $(MAIN_PATH)
	@echo "Built: $(BUILD_DIR)/$(APP_NAME)-arm64"

# Build for all platforms
build-all: build build-jetson

# Run locally
run: build
	$(BUILD_DIR)/$(APP_NAME) -config ./data/config.json -data ./data -port 8080

# Development mode with auto-reload (requires air)
dev:
	@which air > /dev/null || (echo "Installing air..." && go install github.com/air-verse/air@latest)
	air

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -rf ./data/events/pending/*
	rm -rf ./data/events/sent/*
	rm -rf ./data/events/failed/*

# Install dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	@which golangci-lint > /dev/null || (echo "Install golangci-lint first" && exit 1)
	golangci-lint run

# Create systemd service file
install-service:
	@echo "Creating systemd service..."
	@sudo cp scripts/magicbox.service /etc/systemd/system/
	@sudo systemctl daemon-reload
	@echo "Service installed. Enable with: sudo systemctl enable magicbox"

# Package for deployment
package: build-jetson
	@echo "Packaging for deployment..."
	@mkdir -p $(BUILD_DIR)/package
	@cp $(BUILD_DIR)/$(APP_NAME)-arm64 $(BUILD_DIR)/package/magicbox
	@cp -r scripts $(BUILD_DIR)/package/
	@tar -czvf $(BUILD_DIR)/magicbox-$(VERSION)-arm64.tar.gz -C $(BUILD_DIR)/package .
	@echo "Package created: $(BUILD_DIR)/magicbox-$(VERSION)-arm64.tar.gz"

