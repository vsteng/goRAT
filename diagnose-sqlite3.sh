#!/bin/bash
# Diagnostic script to test SQLite3 CGO setup

echo "SQLite3 CGO Diagnostic Script"
echo "============================="
echo ""

# Test 1: Check if sqlite3 CLI exists
echo "Test 1: SQLite3 CLI"
if command -v sqlite3 &> /dev/null; then
    echo "✓ sqlite3 CLI found: $(sqlite3 --version)"
else
    echo "✗ sqlite3 CLI not found"
fi
echo ""

# Test 2: Check development files
echo "Test 2: SQLite3 Development Files"
if [ -f /usr/include/sqlite3.h ]; then
    echo "✓ /usr/include/sqlite3.h exists"
    ls -lh /usr/include/sqlite3.h
else
    echo "✗ /usr/include/sqlite3.h not found"
fi

if [ -f /usr/lib/x86_64-linux-gnu/libsqlite3.so ]; then
    echo "✓ /usr/lib/x86_64-linux-gnu/libsqlite3.so exists"
    ls -lh /usr/lib/x86_64-linux-gnu/libsqlite3.so
elif [ -f /usr/lib64/libsqlite3.so ]; then
    echo "✓ /usr/lib64/libsqlite3.so exists"
    ls -lh /usr/lib64/libsqlite3.so
elif [ -f /usr/lib/libsqlite3.so ]; then
    echo "✓ /usr/lib/libsqlite3.so exists"
    ls -lh /usr/lib/libsqlite3.so
else
    echo "✗ libsqlite3.so not found in standard locations"
    find /usr -name "libsqlite3.so*" 2>/dev/null | head -10
fi
echo ""

# Test 3: pkg-config
echo "Test 3: pkg-config"
if command -v pkg-config &> /dev/null; then
    echo "✓ pkg-config found"
    if pkg-config --exists sqlite3; then
        echo "✓ sqlite3 registered with pkg-config"
        echo "  Version: $(pkg-config --modversion sqlite3)"
        echo "  CFLAGS: $(pkg-config --cflags sqlite3)"
        echo "  LIBS: $(pkg-config --libs sqlite3)"
    else
        echo "✗ sqlite3 not registered with pkg-config"
        echo "  Available modules: $(pkg-config --list-all | grep -i sqlite || echo 'none')"
    fi
else
    echo "✗ pkg-config not found"
fi
echo ""

# Test 4: Go environment
echo "Test 4: Go Environment"
echo "  Go version: $(go version)"
echo "  GOPATH: $(go env GOPATH)"
echo "  GOOS: $(go env GOOS)"
echo "  GOARCH: $(go env GOARCH)"
echo "  CGO_ENABLED: $(go env CGO_ENABLED)"
echo "  CC: $(go env CC)"
echo "  CXX: $(go env CXX)"
echo ""

# Test 5: Try simple Go build test
echo "Test 5: Test Go CGO Build"
TESTDIR=$(mktemp -d)
cat > $TESTDIR/test.go << 'EOF'
package main

import (
	"fmt"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer db.Close()
	
	fmt.Println("✓ SQLite3 driver loaded successfully!")
	fmt.Printf("Database: %v\n", db)
}
EOF

echo "Building test program in $TESTDIR..."
cd $TESTDIR
if CGO_ENABLED=1 go build -o test_sqlite test.go 2>&1; then
    echo "✓ Test program built successfully"
    if ./test_sqlite; then
        echo "✓ Test program executed successfully"
    else
        echo "✗ Test program failed to execute"
    fi
else
    echo "✗ Test program failed to build"
fi
cd - > /dev/null
rm -rf $TESTDIR

echo ""
echo "Diagnostic complete!"
