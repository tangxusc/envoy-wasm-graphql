# Envoy WASM GraphQL Federation Extension Makefile

# Project variables
PROJECT_NAME := envoy-wasm-graphql-federation
WASM_FILE := $(PROJECT_NAME).wasm
MAIN_FILE := cmd/wasm/main.go

# TinyGo variables
TINYGO := tinygo
TINYGO_VERSION := 0.30.0
WASM_TARGET := wasi
WASM_OPT := -opt=2 -no-debug

# Docker variables
DOCKER_IMAGE := tinygo/tinygo:$(TINYGO_VERSION)
DOCKER_WORKDIR := /workspace

# Build directories
BUILD_DIR := build
DIST_DIR := dist

# Go variables
GO_VERSION := 1.21
GO := go

.PHONY: all build build-docker clean test lint format deps check-deps check-tinygo help

# Default target
all: build

# Build the WASM file using TinyGo
build: check-tinygo
	@echo "Building WASM file..."
	@mkdir -p $(BUILD_DIR)
	$(TINYGO) build $(WASM_OPT) -target $(WASM_TARGET) -o $(BUILD_DIR)/$(WASM_FILE) $(MAIN_FILE)
	@echo "WASM file built: $(BUILD_DIR)/$(WASM_FILE)"
	@ls -lh $(BUILD_DIR)/$(WASM_FILE)

# Build using Docker (for consistent environment)
build-docker:
	@echo "Building WASM file using Docker..."
	@mkdir -p $(BUILD_DIR)
	docker run --rm \
		-v $(PWD):$(DOCKER_WORKDIR) \
		-w $(DOCKER_WORKDIR) \
		$(DOCKER_IMAGE) \
		tinygo build $(WASM_OPT) -target $(WASM_TARGET) -o $(BUILD_DIR)/$(WASM_FILE) $(MAIN_FILE)
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

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Lint the code
lint: check-deps
	@echo "Running linter..."
	golangci-lint run ./...

# Format the code
format:
	@echo "Formatting code..."
	$(GO) fmt ./...
	goimports -w .

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GO) mod download
	$(GO) mod tidy

# Check if required tools are installed
check-deps:
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Install it from https://golangci-lint.run/usage/install/" && exit 1)
	@which goimports > /dev/null || (echo "goimports not found. Install with: go install golang.org/x/tools/cmd/goimports@latest" && exit 1)

# Check if TinyGo is installed
check-tinygo:
	@which $(TINYGO) > /dev/null || (echo "TinyGo not found. Install it from https://tinygo.org/getting-started/install/" && exit 1)
	@echo "TinyGo version: $$($(TINYGO) version)"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)
	rm -f coverage.out coverage.html
	@echo "Clean completed"

# Development helpers
dev-setup: deps check-deps check-tinygo
	@echo "Development environment setup completed"

# Quick build and test cycle
quick: format lint test build
	@echo "Quick build and test cycle completed"

# Install TinyGo using Docker
install-tinygo-docker:
	@echo "You can use Docker to build without installing TinyGo locally:"
	@echo "Run: make build-docker"

# Validate WASM file
validate-wasm: build
	@echo "Validating WASM file..."
	@which wasm-validate > /dev/null && wasm-validate $(BUILD_DIR)/$(WASM_FILE) || echo "wasm-validate not found, skipping validation"
	@file $(BUILD_DIR)/$(WASM_FILE)

# Show file size
size: build
	@echo "WASM file size analysis:"
	@ls -lh $(BUILD_DIR)/$(WASM_FILE)
	@echo "Size: $$(du -h $(BUILD_DIR)/$(WASM_FILE) | cut -f1)"

# Generate documentation
docs:
	@echo "Generating documentation..."
	$(GO) doc -all ./pkg/... > docs/api.md 2>/dev/null || true
	@echo "Documentation generated in docs/"

# Development watch mode (requires entr)
watch:
	@which entr > /dev/null || (echo "entr not found. Install with: apt-get install entr or brew install entr" && exit 1)
	@echo "Watching for changes... Press Ctrl+C to stop"
	find . -name "*.go" | entr -r make quick

# Show available targets
help:
	@echo "Available targets:"
	@echo "  build          - Build WASM file using local TinyGo"
	@echo "  build-docker   - Build WASM file using Docker"
	@echo "  dist           - Create distribution package"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  lint           - Run linter"
	@echo "  format         - Format code"
	@echo "  deps           - Install dependencies"
	@echo "  check-deps     - Check required tools"
	@echo "  check-tinygo   - Check TinyGo installation"
	@echo "  clean          - Clean build artifacts"
	@echo "  dev-setup      - Setup development environment"
	@echo "  quick          - Quick build and test cycle"
	@echo "  validate-wasm  - Validate WASM file"
	@echo "  size           - Show WASM file size"
	@echo "  docs           - Generate documentation"
	@echo "  watch          - Watch for changes and rebuild"
	@echo "  help           - Show this help message"