.PHONY: build build-web clean test lint verify

# Variables
BINARY_NAME=go-file-sync
BUILD_DIR=dist
GO_FLAGS=-ldflags="-s -w"

# Default target
all: build

# Compile check for the whole module (catches packages that don't build).
verify:
	@echo "Verifying build of all packages..."
	go build ./...

build-web:
	@echo "Building Vue3 frontend..."
	@if [ -d web ]; then \
		cd web && pnpm install && pnpm run build; \
	else \
		echo "web/ directory not found, skipping frontend build"; \
	fi

build: build-web verify
	@echo "Building $(BINARY_NAME)..."
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(GO_FLAGS) .
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe"

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
build-linux: build-web
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(GO_FLAGS) .

build-darwin: build-web
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(GO_FLAGS) .

build-windows: build-web
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(GO_FLAGS) .

build-all: build-web
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(GO_FLAGS) .
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(GO_FLAGS) .
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(GO_FLAGS) .
	@echo "All builds complete"

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build the binary (includes frontend)"
	@echo "  build-web      Build only the Vue3 frontend"
	@echo "  test           Run all tests"
	@echo "  lint           Run linters"
	@echo "  clean          Remove build artifacts"
	@echo "  install        Install binary to GOPATH"
	@echo "  build-linux    Cross-compile for Linux"
	@echo "  build-darwin   Cross-compile for macOS"
	@echo "  build-windows  Cross-compile for Windows"
	@echo "  build-all      Cross-compile for all platforms"
