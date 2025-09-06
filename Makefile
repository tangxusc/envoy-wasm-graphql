# Envoy WASM GraphQL Federation Extension Makefile

# Project variables
PROJECT_NAME := envoy-wasm-graphql-federation
WASM_FILE := $(PROJECT_NAME).wasm
MAIN_FILE := cmd/wasm/main.go

# Go variables
GO_VERSION := 1.24
GO := go

# Build directories
BUILD_DIR := build
DIST_DIR := dist

.PHONY: all build clean test format deps help

# Default target
all: build

# Build the WASM file using Go wasip1 mode
build:
	@echo "Building WASM file with Go wasip1 mode..."
	@mkdir -p $(BUILD_DIR)
	env GOOS=wasip1 GOARCH=wasm $(GO) build -buildmode=c-shared -o $(BUILD_DIR)/$(WASM_FILE) $(MAIN_FILE)
	@echo "WASM file built: $(BUILD_DIR)/$(WASM_FILE)"
	@ls -lh $(BUILD_DIR)/$(WASM_FILE)

# Create distribution package
dist: build
	@echo "Creating distribution package..."
	@mkdir -p $(DIST_DIR)
	@cp $(BUILD_DIR)/$(WASM_FILE) $(DIST_DIR)/
	@cp examples/envoy.yaml $(DIST_DIR)/ 2>/dev/null || true
	@cp examples/config.json $(DIST_DIR)/ 2>/dev/null || true
	@cp README.md $(DIST_DIR)/ 2>/dev/null || true
	@echo "Distribution package created in $(DIST_DIR)/"

# Run tests
test:
	@echo "Running tests..."
	$(GO) test -v ./...

# Format the code
format:
	@echo "Formatting code..."
	$(GO) fmt ./...

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GO) mod download
	$(GO) mod tidy

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)
	@echo "Clean completed"

# Quick build and test cycle
quick: format test build
	@echo "Quick build and test cycle completed"

# Show file size
size: build
	@echo "WASM file size analysis:"
	@ls -lh $(BUILD_DIR)/$(WASM_FILE)
	@echo "Size: $$(du -h $(BUILD_DIR)/$(WASM_FILE) | cut -f1)"

# Show available targets
help:
	@echo "Available targets:"
	@echo "  build          - Build WASM file using Go wasip1 mode"
	@echo "  dist           - Create distribution package"
	@echo "  test           - Run tests"
	@echo "  format         - Format code"
	@echo "  deps           - Install dependencies"
	@echo "  clean          - Clean build artifacts"
	@echo "  quick          - Quick build and test cycle"
	@echo "  size           - Show WASM file size"
	@echo "  help           - Show this help message"