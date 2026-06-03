.PHONY: build run clean test lint

# Variables
BINARY_NAME=go-file-sync
BUILD_DIR=build
GO_FLAGS=-ldflags="-s -w"

# Default target
all: build

build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(GO_FLAGS) .
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

run:
	@echo "Starting $(BINARY_NAME)..."
	go run . run

run-config:
	@echo "Starting $(BINARY_NAME) with custom config..."
	go run . run --config $(CONFIG)

check:
	@echo "Checking configuration..."
	go run . check --config $(CONFIG)

test:
	@echo "Running tests..."
	go test ./... -v -count=1

lint:
	@echo "Running linter..."
	go vet ./...
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Skipping."; \
	fi

clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	go clean

install:
	@echo "Installing $(BINARY_NAME)..."
	go install .

# Cross compilation
build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(GO_FLAGS) .

build-darwin:
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(GO_FLAGS) .

build-windows:
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(GO_FLAGS) .

build-all: build-linux build-darwin build-windows
	@echo "All builds complete"

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build the binary"
	@echo "  run            Run the application"
	@echo "  run-config     Run with custom CONFIG=path"
	@echo "  check          Validate configuration (CONFIG=path)"
	@echo "  test           Run all tests"
	@echo "  lint           Run linters"
	@echo "  clean          Remove build artifacts"
	@echo "  install        Install binary to GOPATH"
	@echo "  build-linux    Cross-compile for Linux"
	@echo "  build-darwin   Cross-compile for macOS"
	@echo "  build-windows  Cross-compile for Windows"
	@echo "  build-all      Cross-compile for all platforms"
