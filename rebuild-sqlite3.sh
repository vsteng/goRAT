#!/bin/bash
# Force rebuild of sqlite3 on Linux

set -e

echo "Force Rebuild SQLite3 Script"
echo "============================"
echo ""

# Ensure we're on Linux
if [[ ! "$OSTYPE" =~ ^linux ]]; then
    echo "This script is for Linux only"
    exit 1
fi

# Step 1: Check dependencies
echo "Checking dependencies..."
if ! command -v gcc &> /dev/null; then
    echo "Installing gcc and build tools..."
    if command -v apt-get &> /dev/null; then
        sudo apt-get update && sudo apt-get install -y build-essential libsqlite3-dev pkg-config
    elif command -v yum &> /dev/null; then
        sudo yum groupinstall -y 'Development Tools' && sudo yum install -y sqlite-devel pkgconfig
    fi
fi

if ! pkg-config --exists sqlite3 2>/dev/null; then
    echo "Installing sqlite3 dev files..."
    if command -v apt-get &> /dev/null; then
        sudo apt-get install -y libsqlite3-dev pkg-config
    elif command -v yum &> /dev/null; then
        sudo yum install -y sqlite-devel pkgconfig
    fi
fi

echo "✓ Dependencies OK"
echo ""

# Step 2: Clean everything
echo "Cleaning build cache..."
rm -rf bin/server
go clean -cache
go clean -modcache
go clean -testcache

# Step 3: Remove go-sqlite3 from mod cache to force recompilation
echo "Removing sqlite3 from module cache..."
go mod tidy
rm -rf $(go env GOMODCACHE)/github.com/mattn/go-sqlite3*

echo "✓ Cache cleaned"
echo ""

# Step 4: Rebuild with force
echo "Rebuilding server with sqlite3..."
echo ""

mkdir -p bin

export CGO_ENABLED=1
export PKG_CONFIG_PATH="${PKG_CONFIG_PATH}:/usr/lib/pkgconfig:/usr/local/lib/pkgconfig:/usr/lib/x86_64-linux-gnu/pkgconfig"

# Get sqlite3 info
SQLITE_CFLAGS=$(pkg-config --cflags sqlite3)
SQLITE_LIBS=$(pkg-config --libs sqlite3)

echo "SQLite3 Configuration:"
echo "  CFLAGS: $SQLITE_CFLAGS"
echo "  LIBS: $SQLITE_LIBS"
echo ""

# Build with explicit flags
echo "Building..."
CGO_ENABLED=1 \
CGO_CFLAGS="$SQLITE_CFLAGS" \
CGO_LDFLAGS="$SQLITE_LIBS" \
go build -v -x -o bin/server cmd/server/main.go 2>&1 | tee /tmp/build-output.log

echo ""
if [ -f bin/server ]; then
    echo "✓ Binary created: $(ls -lh bin/server | awk '{print $5, $9}')"
    
    # Verify it has sqlite3
    if ldd bin/server 2>/dev/null | grep -q sqlite; then
        echo "✓ SQLite3 is dynamically linked"
        ldd bin/server | grep sqlite
    elif strings bin/server 2>/dev/null | grep -q "sqlite3"; then
        echo "✓ SQLite3 symbols found in binary"
    else
        echo "⚠ Warning: SQLite3 may not be properly linked"
    fi
    
    echo ""
    echo "Ready to test!"
    echo "Run: ./bin/server -addr 127.0.0.1:8081"
else
    echo "✗ Build failed"
    exit 1
fi
