.PHONY: all build clean test server client monitor certs install

# Binary names
SERVER_BIN=bin/server
CLIENT_BIN=bin/client
MONITOR_BIN=bin/client_monitor

# Build all components
all: build

# Build all binaries
build: server client monitor

# Build server
server:
	@echo "Building server..."
	@mkdir -p bin
	@go build -o $(SERVER_BIN) cmd/server/main.go

# Build client
client:
	@echo "Building client..."
	@mkdir -p bin
	@if [ "$$(uname)" = "Darwin" ] && [ "$$(sw_vers -productVersion | cut -d'.' -f1)" -ge 15 ]; then \
		echo "  Note: Screenshot functionality disabled on macOS 15+ (library incompatibility)"; \
		go build -tags noscreenshot -o $(CLIENT_BIN) cmd/client/main.go; \
	else \
		go build -o $(CLIENT_BIN) cmd/client/main.go; \
	fi

# Build monitor
monitor:
	@echo "Building client monitor..."
	@mkdir -p bin
	@go build -o $(MONITOR_BIN) ./client_monitor

# Build for multiple platforms
build-all: build-linux build-windows build-darwin

build-linux:
	@echo "Building for Linux..."
	@mkdir -p bin/linux
	@GOOS=linux GOARCH=amd64 go build -o bin/linux/server cmd/server/main.go
	@GOOS=linux GOARCH=amd64 go build -o bin/linux/client cmd/client/main.go
	@GOOS=linux GOARCH=amd64 go build -o bin/linux/client_monitor ./client_monitor

build-windows:
	@echo "Building for Windows..."
	@mkdir -p bin/windows
	@GOOS=windows GOARCH=amd64 go build -o bin/windows/server.exe cmd/server/main.go
	@GOOS=windows GOARCH=amd64 go build -o bin/windows/client.exe cmd/client/main.go
	@GOOS=windows GOARCH=amd64 go build -o bin/windows/client_monitor.exe ./client_monitor

build-darwin:
	@echo "Building for macOS..."
	@mkdir -p bin/darwin
	@GOOS=darwin GOARCH=amd64 go build -o bin/darwin/server cmd/server/main.go
	@GOOS=darwin GOARCH=amd64 go build -tags noscreenshot -o bin/darwin/client cmd/client/main.go
	@GOOS=darwin GOARCH=amd64 go build -o bin/darwin/client_monitor ./client_monitor
	@echo "  Note: macOS client built without screenshot support"

# Generate TLS certificates
certs:
	@echo "Generating TLS certificates..."
	@chmod +x scripts/generate-certs.sh
	@./scripts/generate-certs.sh

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -rf certs/

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run

# Run server
run-server: server certs
	@echo "Starting server..."
	@$(SERVER_BIN) -addr :8443 -cert certs/server.crt -key certs/server.key -token test-token

# Run client
run-client: client
	@echo "Starting client..."
	@$(CLIENT_BIN) -server wss://localhost:8443/ws -id test-client -token test-token -skip-tls

# Help
help:
	@echo "Available targets:"
	@echo "  all          - Build all components (default)"
	@echo "  build        - Build all binaries"
	@echo "  server       - Build server"
	@echo "  client       - Build client"
	@echo "  monitor      - Build client monitor"
	@echo "  build-all    - Build for all platforms"
	@echo "  build-linux  - Build for Linux"
	@echo "  build-windows - Build for Windows"
	@echo "  build-darwin - Build for macOS"
	@echo "  certs        - Generate TLS certificates"
	@echo "  test         - Run tests"
	@echo "  clean        - Remove build artifacts"
	@echo "  deps         - Install dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter"
	@echo "  run-server   - Build and run server"
	@echo "  run-client   - Build and run client"
	@echo "  help         - Show this help message"
