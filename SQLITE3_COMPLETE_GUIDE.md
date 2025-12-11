# SQLite3 Linux Build - Complete Troubleshooting Guide

## Quick Reference

| Problem | Solution |
|---------|----------|
| sqlite3 driver not found after build | Run `./rebuild-sqlite3.sh` |
| Don't know if environment is correct | Run `./diagnose-sqlite3.sh` |
| Need detailed build process | Run `./build-linux.sh` |
| Want to understand the issue | Read this document |

---

## Understanding the Issue

### Why does it work on macOS but not Linux?

**macOS:**
- SQLite3 is built-in (pre-installed)
- C compiler is included with Xcode
- Go's cgo works out of the box
- No additional setup needed

**Linux:**
- SQLite3 may or may not be installed
- C compiler (gcc) must be explicitly installed
- pkg-config must be installed to find SQLite3
- Go modules cache can hold stale builds

### What's Happening When You See the Error?

```
ERROR: Failed to create client store: sql: unknown driver "sqlite3" (forgotten import?)
```

**What this means:**
1. The Go code IS correct (the import is there)
2. The binary WAS compiled and linked
3. BUT at runtime, the sqlite3 driver fails to initialize
4. This happens because CGO isn't properly enabled during compilation

**Why?**
The `go-sqlite3` module needs to be compiled with CGO enabled. If it's built with CGO disabled, it's a "stub" that cannot actually use the SQLite3 library.

---

## Step-by-Step Solution

### Step 1: Check Your Environment (Optional but Recommended)

```bash
./diagnose-sqlite3.sh
```

This will show:
- ✓ or ✗ for each required component
- Which components need to be installed
- Version information
- Actual errors if any

### Step 2: Fix Dependencies

Based on your distribution:

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install -y build-essential libsqlite3-dev pkg-config
```

**CentOS/RHEL/Fedora:**
```bash
sudo yum groupinstall -y 'Development Tools'
sudo yum install -y sqlite-devel pkgconfig
```

**Alpine Linux:**
```bash
apk add build-base sqlite-dev pkgconfig
```

**Arch Linux:**
```bash
sudo pacman -S base-devel sqlite pkg-config
```

### Step 3: Clean Go Module Cache

This is the **critical step** that most people miss:

```bash
# Clean all Go caches
go clean -cache
go clean -modcache
go clean -testcache

# Remove sqlite3 specifically to force recompilation
rm -rf $(go env GOMODCACHE)/github.com/mattn/go-sqlite3*
```

### Step 4: Rebuild

```bash
CGO_ENABLED=1 go build -o bin/server cmd/server/main.go
```

### Step 5: Test

```bash
./bin/server -addr 127.0.0.1:8081
```

---

## Using the Automated Scripts

### The Nuclear Option (Recommended)

```bash
chmod +x rebuild-sqlite3.sh
./rebuild-sqlite3.sh
```

**What it does:**
1. Checks all dependencies
2. Installs missing dependencies automatically
3. Cleans Go cache completely
4. Rebuilds sqlite3 from scratch
5. Verifies the binary
6. Shows you the result

**Why use this:** One command that handles everything.

### The Diagnostic Route

```bash
chmod +x diagnose-sqlite3.sh
./diagnose-sqlite3.sh
```

**What it checks:**
1. SQLite3 CLI version
2. Development headers location
3. Library files location
4. pkg-config setup
5. Go environment variables
6. Builds and runs a test program

**When to use:** When you want to understand what's installed before rebuilding.

### The Detailed Route

```bash
chmod +x build-linux.sh
./build-linux.sh
```

**What it provides:**
1. Colored output with progress
2. Step-by-step information
3. Dependency installation instructions
4. Verbose build output
5. Verification of the result

**When to use:** For learning or when something goes wrong.

---

## Verification Steps

### Check 1: Binary Exists

```bash
ls -lh bin/server
```

Should show a ~34MB binary (the size includes Go runtime + all dependencies).

### Check 2: Binary Has Symbols

```bash
strings bin/server | grep sqlite3 | head -5
```

Should show sqlite3 symbols if properly linked.

### Check 3: Server Runs

```bash
./bin/server -addr 127.0.0.1:8081
```

Should show:
```
✅ SQLite database initialized successfully
Server starting on 127.0.0.1:8081
```

NOT:
```
ERROR: Failed to create client store: sql: unknown driver "sqlite3"
```

### Check 4: Binary Works (Optional)

```bash
./bin/server -addr 127.0.0.1:8081 &
sleep 2
curl http://127.0.0.1:8081/login 2>/dev/null | head -20
killall server 2>/dev/null
```

Should show HTML login page.

---

## Common Issues and Solutions

### Issue 1: "gcc: command not found"

**Diagnosis:**
```bash
which gcc
```

**Solution:** Install build tools

- Ubuntu/Debian: `sudo apt-get install build-essential`
- CentOS/RHEL: `sudo yum groupinstall 'Development Tools'`
- Alpine: `apk add build-base`
- Arch: `sudo pacman -S base-devel`

### Issue 2: "sqlite3.h: No such file"

**Diagnosis:**
```bash
find / -name sqlite3.h 2>/dev/null
```

**Solution:** Install SQLite3 development files

- Ubuntu/Debian: `sudo apt-get install libsqlite3-dev`
- CentOS/RHEL: `sudo yum install sqlite-devel`
- Alpine: `apk add sqlite-dev`
- Arch: `sudo pacman -S sqlite`

### Issue 3: "pkg-config not found"

**Diagnosis:**
```bash
which pkg-config
```

**Solution:** Install pkg-config

- Ubuntu/Debian: `sudo apt-get install pkg-config`
- CentOS/RHEL: `sudo yum install pkgconfig`
- Alpine: `apk add pkgconfig`
- Arch: `sudo pacman -S pkg-config`

### Issue 4: Error Still Persists After Installing

**Diagnosis:**
```bash
./diagnose-sqlite3.sh
```

**Solution - Complete Reset:**

```bash
# 1. Remove all go caches
go clean -cache
go clean -modcache
go clean -testcache

# 2. Remove the binary
rm -f bin/server

# 3. Re-download all dependencies
go mod download
go mod verify

# 4. Rebuild with explicit environment
export CGO_ENABLED=1
export CGO_CFLAGS=$(pkg-config --cflags sqlite3)
export CGO_LDFLAGS=$(pkg-config --libs sqlite3)
go build -v -o bin/server cmd/server/main.go
```

### Issue 5: "pkg-config not finding sqlite3"

**Diagnosis:**
```bash
pkg-config --list-all | grep -i sqlite
pkg-config --modversion sqlite3 2>&1
```

**Solution:** Set PKG_CONFIG_PATH

```bash
# Find where sqlite3.pc is located
find /usr -name "sqlite3.pc" 2>/dev/null

# Add to path (example, adjust based on find output)
export PKG_CONFIG_PATH="/usr/lib/x86_64-linux-gnu/pkgconfig:$PKG_CONFIG_PATH"

# Try again
pkg-config --modversion sqlite3
```

---

## Understanding the Build Flags

When you see this command:

```bash
CGO_ENABLED=1 CGO_CFLAGS="-I/usr/include" CGO_LDFLAGS="-L/usr/lib -lsqlite3" go build -v -o bin/server cmd/server/main.go
```

Here's what each part does:

| Flag | Meaning |
|------|---------|
| `CGO_ENABLED=1` | Enable C compiler integration |
| `CGO_CFLAGS=...` | Tell compiler where to find headers (sqlite3.h) |
| `CGO_LDFLAGS=...` | Tell linker where to find libraries (libsqlite3.so) |
| `-v` | Verbose output showing what's being compiled |
| `-o bin/server` | Output file location |
| `cmd/server/main.go` | Input file to compile |

---

## Docker / Container Builds

### Alpine-based (Minimal, ~5MB base image)

```dockerfile
FROM golang:1.25-alpine

RUN apk add --no-cache build-base sqlite-dev pkgconfig

WORKDIR /app
COPY . .

RUN CGO_ENABLED=1 go build -o server cmd/server/main.go

EXPOSE 8080
CMD ["./server", "-addr", ":8080"]
```

### Debian-based (Larger, more compatible)

```dockerfile
FROM golang:1.25

RUN apt-get update && \
    apt-get install -y build-essential libsqlite3-dev pkg-config && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY . .

RUN CGO_ENABLED=1 go build -o server cmd/server/main.go

EXPOSE 8080
CMD ["./server", "-addr", ":8080"]
```

### Multi-stage build (Optimized)

```dockerfile
# Build stage
FROM golang:1.25-alpine as builder

RUN apk add --no-cache build-base sqlite-dev pkgconfig

WORKDIR /app
COPY . .

RUN CGO_ENABLED=1 go build -o server cmd/server/main.go

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache libsqlite3

COPY --from=builder /app/server /app/server
COPY --from=builder /app/web /app/web

EXPOSE 8080
CMD ["/app/server", "-addr", ":8080"]
```

---

## Advanced Topics

### Cross-Compilation (Linux from macOS)

This is **not recommended** for most users. Instead, build on the target Linux system.

If you must cross-compile:

```bash
# Requires x86_64-linux-gnu-gcc to be installed
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
  CC=x86_64-linux-gnu-gcc \
  go build -o bin/server cmd/server/main.go
```

### Static Linking

To create a standalone binary without runtime dependencies:

```bash
CGO_ENABLED=1 CFLAGS="-static" LDFLAGS="-static -s" \
  go build -o bin/server cmd/server/main.go
```

### Build Optimization

For smaller binary size:

```bash
CGO_ENABLED=1 go build -ldflags="-s -w" -o bin/server cmd/server/main.go
```

This removes debug symbols (-s) and DWARF symbol tables (-w), reducing size by ~30%.

---

## Getting Help

If you're still stuck:

1. **Run diagnostics:** `./diagnose-sqlite3.sh`
2. **Share the output** with a description of your system
3. **Check:** Do you have sudo access? Some systems don't allow sudo.
4. **Try:** Building in a Docker container if system install fails

---

## Summary

**The Problem:** sqlite3 Go module cache without CGO support

**The Solution:** Clean cache + rebuild with CGO enabled

**The Fastest Fix:** `./rebuild-sqlite3.sh`

**The Safe Route:** `./diagnose-sqlite3.sh` → install deps → rebuild

**Good Luck!** 🚀
