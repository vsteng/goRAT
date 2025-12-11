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

On macOS, these dependencies are typically pre-installed, which is why the build works fine there. On Linux, you need to explicitly install them.

## Solution

### Step 1: Install Required Dependencies

Choose the command for your Linux distribution:

#### Ubuntu/Debian
```bash
sudo apt-get update
sudo apt-get install -y build-essential libsqlite3-dev
```

#### CentOS/RHEL/Fedora
```bash
sudo yum groupinstall -y 'Development Tools'
sudo yum install -y sqlite-devel
```

#### Alpine Linux
```bash
apk add build-base sqlite-dev
```

#### Arch Linux
```bash
sudo pacman -S base-devel sqlite
```

### Step 2: Build with CGO Enabled

The updated build scripts automatically enable CGO on Linux. You can build using:

#### Using build.sh (Recommended)
```bash
chmod +x build.sh
./build.sh
```

The script will automatically:
- Detect if you're on Linux
- Check for required dependencies
- Install them if missing (with sudo)
- Build with CGO_ENABLED=1

#### Using Makefile
```bash
make server
```

Or for cross-platform builds:
```bash
make build-linux
```

#### Manual build
```bash
CGO_ENABLED=1 go build -o bin/server cmd/server/main.go
```

## Verification

After building, verify the server works:

```bash
./bin/server -addr 127.0.0.1:8081
```

You should see output similar to:
```
2025/12/11 10:00:00 ✅ SQLite database initialized
2025/12/11 10:00:00 Server starting on 127.0.0.1:8081
```

If you see the "ERROR: Failed to create client store" message, the dependencies aren't properly installed.

## Docker Builds

If building in Docker, ensure your Dockerfile includes the necessary dependencies:

```dockerfile
FROM golang:1.21-alpine

# Install build dependencies for CGO and SQLite3
RUN apk add --no-cache build-base sqlite-dev

WORKDIR /app

COPY . .

# Build with CGO enabled
RUN CGO_ENABLED=1 go build -o server cmd/server/main.go

CMD ["./server", "-addr", ":8080"]
```

## Cross-Compilation Notes

When cross-compiling for Linux from macOS:

```bash
# This requires a Linux cross-compiler setup
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC=x86_64-linux-gnu-gcc go build -o bin/server cmd/server/main.go
```

**Note:** Cross-compilation with CGO is complex and requires the appropriate C compiler for the target platform. For most use cases, it's easier to build directly on the target Linux system.

## Troubleshooting

### "sqlite3.h: No such file or directory"
The SQLite3 development files are not installed. Install them using the commands above for your distribution.

### "gcc: command not found"
Build tools (GCC) are not installed. Install the development tools package for your distribution.

### "pkg-config --cflags sqlite3" fails
SQLite3 pkg-config files are missing. Reinstall libsqlite3-dev or sqlite-devel.

## Why This Works

1. **CGO_ENABLED=1** - Enables C code compilation, required for sqlite3's C bindings
2. **Build dependencies** - Provides the C compiler (gcc) and SQLite3 headers/libraries
3. **Automatic dependency detection** - The updated build scripts check and warn about missing dependencies

## Performance Note

Building with CGO is slightly slower but necessary for SQLite3. Once built, there's no performance impact at runtime - SQLite3 will work normally for persistent storage.
