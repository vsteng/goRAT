# Linux Build Guide - SQLite3 CGO Issue

## Problem

When building the server on Linux, you may encounter this error:

```
ERROR: Failed to create client store: sql: unknown driver "sqlite3" (forgotten import?)
Server will continue without persistent storage
```

This happens because the server uses SQLite3 for persistent storage, which requires:
1. **CGO to be enabled** during compilation
2. **SQLite3 development files** to be installed on the system
3. **Proper module cache** - sometimes the go mod cache needs to be cleared

On macOS, these dependencies are typically pre-installed, which is why the build works fine there. On Linux, you need to explicitly install them.

## Quick Fix (Recommended)

### Option 1: Use the Automated Linux Build Script

The easiest way is to use the dedicated Linux build script which handles everything:

```bash
chmod +x rebuild-sqlite3.sh
./rebuild-sqlite3.sh
```

This script:
- Checks and installs all dependencies
- Cleans the Go module cache (important!)
- Rebuilds sqlite3 from source
- Verifies the SQLite3 linking
- Shows detailed build information

### Option 2: Use the Linux-Specific Build Script

For a comprehensive build with detailed diagnostics:

```bash
chmod +x build-linux.sh
./build-linux.sh
```

## Manual Steps

If the automated scripts don't work, follow these steps:

### Step 1: Install Required Dependencies

Choose the command for your Linux distribution:

#### Ubuntu/Debian (Recommended for most users)
```bash
sudo apt-get update
sudo apt-get install -y build-essential libsqlite3-dev pkg-config
```

#### CentOS/RHEL/Fedora
```bash
sudo yum groupinstall -y 'Development Tools'
sudo yum install -y sqlite-devel pkgconfig
```

#### Alpine Linux
```bash
apk add build-base sqlite-dev pkgconfig
```

#### Arch Linux
```bash
sudo pacman -S base-devel sqlite pkg-config
```

### Step 2: Clean the Go Module Cache

This is **critical** - old cached modules must be removed:

```bash
go clean -cache
go clean -modcache
go clean -testcache
rm -rf $(go env GOMODCACHE)/github.com/mattn/go-sqlite3*
```

### Step 3: Rebuild

```bash
CGO_ENABLED=1 go build -o bin/server cmd/server/main.go
```

Or if that doesn't work, use explicit flags:

```bash
export CGO_ENABLED=1
export CGO_CFLAGS=$(pkg-config --cflags sqlite3)
export CGO_LDFLAGS=$(pkg-config --libs sqlite3)
go build -o bin/server cmd/server/main.go
```

## Verification & Diagnostics

### Run Diagnostic Script

To check if everything is properly configured:

```bash
chmod +x diagnose-sqlite3.sh
./diagnose-sqlite3.sh
```

This will:
- Check sqlite3 CLI installation
- Verify development files
- Test pkg-config setup
- Show Go environment
- Build and test a simple sqlite3 program

### Manual Verification

After building, verify the server works:

```bash
./bin/server -addr 127.0.0.1:8081
```

You should see output similar to:
```
2025/12/11 10:00:00 ✅ SQLite database initialized successfully
2025/12/11 10:00:00 Server starting on 127.0.0.1:8081
```

If you still see the "ERROR: Failed to create client store" message, check:
1. CGO dependencies are installed (run `gcc --version`)
2. SQLite3 dev files exist (`pkg-config --modversion sqlite3`)
3. The module cache was cleaned (`go clean -modcache`)

### Check Binary Linking

To verify SQLite3 is properly linked in the binary:

```bash
# Check if sqlite3 is dynamically linked
ldd ./bin/server | grep sqlite

# Or check for sqlite3 symbols in the binary
strings ./bin/server | grep sqlite3
```

## Available Scripts

| Script | Purpose |
|--------|---------|
| `build.sh` | General build script (auto-detects OS) |
| `build-linux.sh` | Linux-specific with detailed output |
| `rebuild-sqlite3.sh` | Force rebuild sqlite3 (cleans cache) |
| `diagnose-sqlite3.sh` | Check environment and test sqlite3 |

## Docker Builds

If building in Docker, ensure your Dockerfile includes the necessary dependencies:

```dockerfile
FROM golang:1.25-alpine

# Install build dependencies for CGO and SQLite3
RUN apk add --no-cache build-base sqlite-dev pkg-config

WORKDIR /app

COPY . .

# Build with CGO enabled
RUN CGO_ENABLED=1 go build -o server cmd/server/main.go

CMD ["./server", "-addr", ":8080"]
```

Or for Debian-based:

```dockerfile
FROM golang:1.25

# Install build dependencies
RUN apt-get update && \
    apt-get install -y build-essential libsqlite3-dev pkg-config && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY . .

# Build with CGO enabled
RUN CGO_ENABLED=1 go build -o server cmd/server/main.go

CMD ["./server", "-addr", ":8080"]
```

## Troubleshooting

### "sqlite3.h: No such file or directory"
**Solution:** Install SQLite3 development files:
```bash
# Ubuntu/Debian
sudo apt-get install -y libsqlite3-dev

# CentOS/RHEL
sudo yum install -y sqlite-devel

# Alpine
apk add sqlite-dev
```

### "gcc: command not found"
**Solution:** Install GCC and build tools:
```bash
# Ubuntu/Debian
sudo apt-get install -y build-essential

# CentOS/RHEL
sudo yum groupinstall -y 'Development Tools'

# Alpine
apk add build-base
```

### "pkg-config: command not found"
**Solution:** Install pkg-config:
```bash
# Ubuntu/Debian
sudo apt-get install -y pkg-config

# CentOS/RHEL
sudo yum install -y pkgconfig

# Alpine
apk add pkgconfig
```

### ERROR persists after installing dependencies

1. **Clean the module cache completely:**
   ```bash
   go clean -cache
   go clean -modcache
   rm -rf $(go env GOMODCACHE)/github.com/mattn/*
   ```

2. **Re-download dependencies:**
   ```bash
   go mod download
   go mod verify
   ```

3. **Force rebuild:**
   ```bash
   rm -f bin/server
   CGO_ENABLED=1 go build -v -o bin/server cmd/server/main.go
   ```

4. **If still failing, use the script:**
   ```bash
   ./rebuild-sqlite3.sh
   ```

### "pkg-config not in path for sqlite3"

Set the PKG_CONFIG_PATH:

```bash
export PKG_CONFIG_PATH="/usr/lib/pkgconfig:/usr/local/lib/pkgconfig:/usr/lib/x86_64-linux-gnu/pkgconfig"
CGO_ENABLED=1 go build -o bin/server cmd/server/main.go
```

## Why This Works

1. **CGO_ENABLED=1** - Enables C code compilation, required for sqlite3's C bindings
2. **Build dependencies** - Provides the C compiler (gcc) and SQLite3 headers/libraries
3. **Module cache cleaning** - Ensures cached modules are recompiled with proper flags
4. **pkg-config** - Communicates proper compilation flags between Go and SQLite3

## Performance Note

Building with CGO is slightly slower than pure Go builds but necessary for SQLite3. Once built, there's no performance impact at runtime - SQLite3 will work normally for persistent storage.

## Cross-Compilation

When cross-compiling for Linux from macOS (advanced):

```bash
# First, set up a Linux cross-compiler toolchain
# This is complex and beyond the scope here

# Then build with:
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC=x86_64-linux-gnu-gcc go build -o bin/server cmd/server/main.go
```

**Recommendation:** For most use cases, build directly on the target Linux system using `./rebuild-sqlite3.sh`.

