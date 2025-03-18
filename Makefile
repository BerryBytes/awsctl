# Variables
GO_BUILD_FLAGS=-gcflags "all=-N -l" --ldflags "-s -w"
BIN_DIR=bin
BINARY_NAME=awsctl

# Platform and architecture for cross-compilation
GO_LINUX=linux
GO_AMD64=amd64
GO_ARM64=arm64
GO_WINDOWS=windows
GO_DARWIN=darwin

# Default
run:
	@echo "Running the project locally..."
	go run ./cmd/main.go sso setup

# Build binaries for Linux
.PHONY: build-linux
build-linux:
	@echo "Building for Linux (AMD64 and ARM64)..."
	go build $(GO_BUILD_FLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/main.go
	GOOS=$(GO_LINUX) GOARCH=$(GO_ARM64) go build $(GO_BUILD_FLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/main.go

# Build binaries for Windows
.PHONY: build-windows
build-windows:
	@echo "Building for Windows (AMD64)..."
	GOOS=$(GO_WINDOWS) GOARCH=$(GO_AMD64) go build $(GO_BUILD_FLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/main.go

# Build binaries for macOS
.PHONY: build-darwin
build-darwin:
	@echo "Building for macOS (AMD64 and ARM64)..."
	GOOS=$(GO_DARWIN) GOARCH=$(GO_AMD64) go build $(GO_BUILD_FLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/main.go
	GOOS=$(GO_DARWIN) GOARCH=$(GO_ARM64) go build $(GO_BUILD_FLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/main.go

# Target to build the binaries for all platforms
.PHONY: all
all: build-linux build-windows build-darwin

run-ssosetup:
	@echo "Running the project locally..."
	go run ./cmd/main.go sso setup

run-ssoinit:
	@echo "Running the project locally..."
	go run ./cmd/main.go sso init

.PHONY: fmt
fmt:
	@echo "Formatting Go code..."
	go fmt ./...

.PHONY: gmt
gmt:
	@echo "Installing Go dependencies..."
	go mod tidy

.PHONY: clean
clean:
	@echo "Cleaning up the bin directory..."
	rm -rf $(BIN_DIR)
