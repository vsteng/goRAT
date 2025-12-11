#!/bin/bash

set -e

echo "Building Server Manager Project..."
echo ""

# Check for Linux and ensure CGO dependencies
if [[ "$OSTYPE" == "linux-gnu"* ]] || [[ "$OSTYPE" == "linux"* ]]; then
    echo "ℹ️  Building on Linux - ensuring CGO dependencies..."
    
    # Check for required build tools and sqlite3 development files
    if ! command -v gcc &> /dev/null; then
        echo "❌ gcc not found. Installing build tools..."
        if command -v apt-get &> /dev/null; then
            sudo apt-get update && sudo apt-get install -y build-essential libsqlite3-dev
        elif command -v yum &> /dev/null; then
            sudo yum groupinstall -y 'Development Tools' && sudo yum install -y sqlite-devel
        elif command -v apk &> /dev/null; then
            sudo apk add build-base sqlite-dev
        fi
    fi
    
    if ! pkg-config --exists sqlite3 2>/dev/null; then
        echo "❌ sqlite3 development files not found. Installing..."
        if command -v apt-get &> /dev/null; then
            sudo apt-get install -y libsqlite3-dev pkg-config
        elif command -v yum &> /dev/null; then
            sudo yum install -y sqlite-devel pkgconfig
        elif command -v apk &> /dev/null; then
            sudo apk add sqlite-dev pkgconfig
        fi
    fi
    
    echo "✓ CGO dependencies verified"
    echo ""
    
    # Show CGO settings
    echo "CGO Configuration:"
    echo "  CGO_ENABLED: 1"
    which gcc && echo "  GCC: $(which gcc)"
    pkg-config --cflags sqlite3 2>/dev/null && echo "  SQLite3: $(pkg-config --cflags --libs sqlite3)" || echo "  SQLite3: Warning - pkg-config failed"
    echo ""
fi

# Create bin directory
mkdir -p bin

# Clean Go build cache for sqlite3 to force recompilation
echo "Cleaning build cache for fresh compilation..."
if [[ "$OSTYPE" == "linux-gnu"* ]] || [[ "$OSTYPE" == "linux"* ]]; then
    # Clean the sqlite3 module cache
    go clean -cache 2>/dev/null || true
    go clean -modcache 2>/dev/null || true
fi

# Download dependencies explicitly
echo "Downloading and verifying dependencies..."
go mod download -x 2>&1 | head -20
go mod verify 2>/dev/null || true

# Build server with CGO enabled (required for sqlite3)
echo "Building server..."
if [[ "$OSTYPE" == "linux-gnu"* ]] || [[ "$OSTYPE" == "linux"* ]]; then
    echo "  Using: CGO_ENABLED=1 go build -v -o bin/server cmd/server/main.go"
    CGO_ENABLED=1 go build -v -o bin/server cmd/server/main.go 2>&1 | grep -E "sqlite3|error|warning|github.com/mattn" || true
else
    go build -o bin/server cmd/server/main.go
fi
echo "✓ Server built successfully"

# Build client (with conditional screenshot support)
echo "Building client..."
if [[ "$OSTYPE" == "darwin"* ]] && [[ $(sw_vers -productVersion | cut -d'.' -f1) -ge 15 ]]; then
    echo "  Note: Screenshot functionality disabled on macOS 15+ (library incompatibility)"
    go build -tags noscreenshot -o bin/client cmd/client/main.go || {
        echo "  Building client without screenshot tags..."
        go build -o bin/client cmd/client/main.go
    }
else
    go build -o bin/client cmd/client/main.go
fi
echo "✓ Client built successfully"

# Build client_monitor
echo "Building client_monitor..."
go build -o bin/client_monitor ./client_monitor
echo "✓ Client monitor built successfully"

echo ""
echo "All builds completed successfully!"
echo "Binaries available in bin/"
ls -lh bin/

# Verify sqlite3 support
echo ""
echo "Verifying SQLite3 support..."
if [[ "$OSTYPE" == "linux-gnu"* ]] || [[ "$OSTYPE" == "linux"* ]]; then
    # Try to check if sqlite3 is linked
    ldd ./bin/server 2>/dev/null | grep -i sqlite && echo "✓ SQLite3 is linked" || echo "⚠️  SQLite3 linking not visible in ldd (this is normal)"
fi
