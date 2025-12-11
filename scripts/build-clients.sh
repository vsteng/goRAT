#!/bin/bash
# Build script for creating both debug and release versions of the client

set -e

VERSION=${1:-"1.0.0"}
PLATFORMS=${2:-"linux windows darwin"}

echo "Building client version $VERSION for platforms: $PLATFORMS"
echo "=================================================="

for platform in $PLATFORMS; do
    echo ""
    echo "Building for $platform..."
    
    case $platform in
        linux)
            echo "  - Linux Debug..."
            GOOS=linux GOARCH=amd64 go build -tags debug -o bin/linux/client-debug cmd/client/main.go
            echo "  - Linux Release..."
            GOOS=linux GOARCH=amd64 go build -o bin/linux/client-release cmd/client/main.go
            ;;
        windows)
            echo "  - Windows Debug..."
            GOOS=windows GOARCH=amd64 go build -tags debug -o bin/windows/client-debug.exe cmd/client/main.go
            echo "  - Windows Release..."
            GOOS=windows GOARCH=amd64 go build -o bin/windows/client-release.exe cmd/client/main.go
            ;;
        darwin)
            echo "  - macOS Debug..."
            GOOS=darwin GOARCH=amd64 go build -tags "debug noscreenshot" -o bin/darwin/client-debug cmd/client/main.go
            echo "  - macOS Release..."
            GOOS=darwin GOARCH=amd64 go build -tags noscreenshot -o bin/darwin/client-release cmd/client/main.go
            ;;
        *)
            echo "  Unknown platform: $platform"
            ;;
    esac
done

echo ""
echo "Build complete!"
echo ""
echo "Debug versions:"
ls -lh bin/*/client-debug* 2>/dev/null || echo "  No debug builds found"
echo ""
echo "Release versions:"
ls -lh bin/*/client-release* 2>/dev/null || echo "  No release builds found"
