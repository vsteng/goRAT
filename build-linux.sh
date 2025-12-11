#!/bin/bash
# Linux-specific build script with comprehensive SQLite3 debugging

set -e

echo "🐧 Linux-Specific Build Script"
echo "=============================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Step 1: Check and install dependencies
echo -e "${BLUE}Step 1: Checking dependencies...${NC}"
echo ""

MISSING_DEPS=0

if ! command -v gcc &> /dev/null; then
    echo -e "${RED}✗ gcc not found${NC}"
    MISSING_DEPS=1
else
    echo -e "${GREEN}✓ gcc found: $(gcc --version | head -1)${NC}"
fi

if ! command -v pkg-config &> /dev/null; then
    echo -e "${RED}✗ pkg-config not found${NC}"
    MISSING_DEPS=1
else
    echo -e "${GREEN}✓ pkg-config found${NC}"
fi

if ! pkg-config --exists sqlite3 2>/dev/null; then
    echo -e "${RED}✗ SQLite3 dev files not found${NC}"
    MISSING_DEPS=1
else
    SQLITE_VERSION=$(pkg-config --modversion sqlite3)
    echo -e "${GREEN}✓ SQLite3 found: $SQLITE_VERSION${NC}"
fi

echo ""

# Install missing dependencies if needed
if [ $MISSING_DEPS -eq 1 ]; then
    echo -e "${YELLOW}Installing missing dependencies...${NC}"
    
    if command -v apt-get &> /dev/null; then
        echo "Detected Debian/Ubuntu system"
        sudo apt-get update
        sudo apt-get install -y build-essential libsqlite3-dev pkg-config
    elif command -v yum &> /dev/null; then
        echo "Detected CentOS/RHEL system"
        sudo yum groupinstall -y 'Development Tools'
        sudo yum install -y sqlite-devel pkgconfig
    elif command -v apk &> /dev/null; then
        echo "Detected Alpine system"
        sudo apk add build-base sqlite-dev pkgconfig
    elif command -v pacman &> /dev/null; then
        echo "Detected Arch system"
        sudo pacman -S --noconfirm base-devel sqlite
    else
        echo -e "${RED}Unknown Linux distribution. Please install:${NC}"
        echo "  - build-essential or equivalent"
        echo "  - libsqlite3-dev or sqlite-devel"
        echo "  - pkg-config"
        exit 1
    fi
    echo -e "${GREEN}✓ Dependencies installed${NC}"
fi

echo ""
echo -e "${BLUE}Step 2: Checking CGO environment...${NC}"
echo ""

echo "Environment variables:"
echo "  CGO_ENABLED: ${CGO_ENABLED:-not set (will be set to 1)}"
echo "  GOOS: ${GOOS:-$(go env GOOS)}"
echo "  GOARCH: ${GOARCH:-$(go env GOARCH)}"
echo ""

# Check SQLite3 configuration
echo "SQLite3 configuration:"
SQLite_CFLAGS=$(pkg-config --cflags sqlite3)
SQLite_LIBS=$(pkg-config --libs sqlite3)
echo "  CFLAGS: $SQLite_CFLAGS"
echo "  LIBS: $SQLite_LIBS"
echo ""

# Step 3: Clean and prepare
echo -e "${BLUE}Step 3: Cleaning build cache...${NC}"
go clean -cache
go clean -modcache
echo -e "${GREEN}✓ Cache cleaned${NC}"
echo ""

# Step 4: Download dependencies
echo -e "${BLUE}Step 4: Downloading dependencies...${NC}"
go mod download
go mod verify
echo -e "${GREEN}✓ Dependencies verified${NC}"
echo ""

# Step 5: Build
echo -e "${BLUE}Step 5: Building with CGO enabled...${NC}"
echo ""

mkdir -p bin

# Export CGO settings explicitly
export CGO_ENABLED=1
export CGO_CFLAGS=$(pkg-config --cflags sqlite3)
export CGO_LDFLAGS=$(pkg-config --libs sqlite3)

echo "Build command:"
echo "  CGO_ENABLED=1 CGO_CFLAGS='$CGO_CFLAGS' CGO_LDFLAGS='$CGO_LDFLAGS' go build -v -o bin/server cmd/server/main.go"
echo ""

# Build with verbose output
if CGO_ENABLED=1 go build -v -o bin/server cmd/server/main.go 2>&1 | tee /tmp/build.log; then
    echo ""
    echo -e "${GREEN}✓ Server built successfully${NC}"
    
    # Check if sqlite3 was actually compiled
    if grep -q "github.com/mattn/go-sqlite3" /tmp/build.log; then
        echo -e "${GREEN}✓ SQLite3 module was compiled${NC}"
    else
        echo -e "${YELLOW}⚠ SQLite3 module not mentioned in build log (may be cached)${NC}"
    fi
else
    echo ""
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
fi

echo ""

# Build client
echo -e "${BLUE}Step 6: Building client...${NC}"
go build -o bin/client cmd/client/main.go
echo -e "${GREEN}✓ Client built${NC}"

# Build monitor
echo -e "${BLUE}Step 7: Building client_monitor...${NC}"
go build -o bin/client_monitor ./client_monitor
echo -e "${GREEN}✓ Client monitor built${NC}"

echo ""
echo -e "${BLUE}Build Artifacts:${NC}"
ls -lh bin/

echo ""
echo -e "${BLUE}Verifying SQLite3 linking...${NC}"
echo ""

# Try to check dynamic libraries
if command -v ldd &> /dev/null; then
    echo "Linked libraries:"
    ldd ./bin/server | grep -E "sqlite|libc" || echo "  (sqlite may be statically linked)"
fi

# Try to check the binary
if file ./bin/server | grep -q "x86-64"; then
    echo -e "${GREEN}✓ Binary is properly compiled for x86-64${NC}"
fi

echo ""
echo -e "${GREEN}✅ Build completed successfully!${NC}"
echo ""
echo "To run the server:"
echo "  ./bin/server -addr 127.0.0.1:8081"
echo ""
