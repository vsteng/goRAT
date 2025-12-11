# Cross-Platform Build Guide

## Overview
GoRAT provides multiple build scripts for different platforms. Choose the one that best fits your development environment.

## Windows Build Scripts

### Option 1: Batch File (.bat)
**Best for:** Traditional Windows Command Prompt users

```cmd
cd C:\path\to\gorat
build-clients.bat
```

**With flags:**
```cmd
build-clients.bat clean      # Clean build cache first
build-clients.bat release    # Optimized release build
```

### Option 2: PowerShell Script (.ps1)
**Best for:** Modern Windows PowerShell users with better error handling

```powershell
cd C:\path\to\gorat
.\build-clients.ps1
```

**With parameters:**
```powershell
.\build-clients.ps1 -Clean          # Clean build cache
.\build-clients.ps1 -Release        # Optimized release build
.\build-clients.ps1 -Verbose        # Detailed output
.\build-clients.ps1 -Clean -Release # Both flags combined
```

### Option 3: Go Command
**Best for:** Direct control with Go

```cmd
REM Windows 64-bit
set GOOS=windows
set GOARCH=amd64
go build -o bin\client-windows-amd64.exe .\cmd\client\main.go

REM Windows 32-bit
set GOOS=windows
set GOARCH=386
go build -o bin\client-windows-386.exe .\cmd\client\main.go

REM Linux 64-bit (cross-compile from Windows)
set GOOS=linux
set GOARCH=amd64
go build -o bin\client-linux-amd64 .\cmd\client\main.go
```

## Output Binaries

The build scripts generate the following client binaries:

### Windows
- `bin/client-windows-amd64.exe` - Windows 64-bit (Intel/AMD processors)
- `bin/client-windows-386.exe` - Windows 32-bit

### Linux
- `bin/client-linux-amd64` - Linux 64-bit (Intel/AMD processors)
- `bin/client-linux-386` - Linux 32-bit
- `bin/client-linux-arm` - Linux ARM (Raspberry Pi, etc.)
- `bin/client-linux-arm64` - Linux ARM 64-bit

### macOS
- `bin/client-darwin-amd64` - macOS Intel (x86-64)
- `bin/client-darwin-arm64` - macOS Apple Silicon (M1, M2, M3)

## Build Flags

### Release Build (Optimized, Smaller)
Removes debug symbols and uses optimizations for smaller binary size.

**Batch file:**
```cmd
build-clients.bat release
```

**PowerShell:**
```powershell
.\build-clients.ps1 -Release
```

**Manual:**
```cmd
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o bin\client-windows-amd64.exe .\cmd\client\main.go
```

### Debug Build (Default)
Includes debug symbols for debugging support.

```cmd
build-clients.bat
```

### Clean Build
Clears Go's build cache before building.

```cmd
build-clients.bat clean
```

## Platform-Specific Details

### Windows Build Requirements
- Go 1.21 or later
- No additional dependencies required for cross-compilation
- Can build for other platforms from Windows

### Linux Build Requirements
- Go 1.21 or later
- `gcc` and `make` for CGO (SQLite3)
- `sqlite3-dev` or similar development package

**Installation (Ubuntu/Debian):**
```bash
sudo apt-get install build-essential libsqlite3-dev pkg-config
```

**Installation (CentOS/RHEL):**
```bash
sudo yum groupinstall "Development Tools"
sudo yum install sqlite-devel
```

### macOS Build Requirements
- Go 1.21 or later
- Xcode Command Line Tools: `xcode-select --install`
- Note: macOS 15+ has library incompatibilities with older screenshot library

## Running Built Clients

### Windows
```cmd
REM Navigate to client directory or provide full path
.\bin\client-windows-amd64.exe -server wss://your-server:8080/ws

REM Run with options
.\bin\client-windows-amd64.exe `
  -server wss://your-server:8080/ws `
  -autostart `
  -daemon
```

### Linux/macOS
```bash
# Make executable
chmod +x ./bin/client-linux-amd64

# Run client
./bin/client-linux-amd64 -server wss://your-server:8080/ws

# Run as daemon
./bin/client-linux-amd64 -daemon -server wss://your-server:8080/ws
```

## Client Options

```
-server string
    Server WebSocket URL (must include /ws path)
    Default: wss://localhost:8080/ws
    
-autostart
    Enable auto-start on system boot
    Default: false for release, true for debug
    
-daemon
    Run as background daemon/service
    Default: false
    
-help
    Show help message
```

## Architecture Support Matrix

| Platform | x86 | x64 | ARM | ARM64 |
|----------|-----|-----|-----|-------|
| Windows  | ✓   | ✓   | -   | -     |
| Linux    | ✓   | ✓   | ✓   | ✓     |
| macOS    | -   | ✓   | -   | ✓     |

**Legend:**
- ✓ Supported
- - Not available
- Note: Some ARM variants may have limited feature support

## Troubleshooting

### Build fails with "go: finding module" error
**Solution:** Ensure go.mod exists and module path is correct:
```cmd
type go.mod
REM Should show: module gorat
```

### GOOS/GOARCH not recognized
**Solution:** Use proper case (lowercase):
```cmd
REM Correct:
set GOOS=linux

REM Incorrect:
set GOOS=Linux
```

### "command not found: go"
**Solution:** Go is not in your PATH:
1. Install Go from https://golang.org/dl/
2. Restart terminal/command prompt
3. Verify: `go version`

### Build succeeds but binary won't run
**Solution:** Verify correct platform and architecture:
```bash
# Check binary file info
file ./bin/client-linux-amd64

# List all binaries
ls -la ./bin/
```

### PowerShell: "cannot be loaded because running scripts is disabled"
**Solution:** Allow script execution:
```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

### Binary is too large
**Solution:** Use release build flag:
```cmd
build-clients.bat release
```

This can reduce binary size by 30-40%.

## Continuous Integration

### GitHub Actions Example
```yaml
name: Build GoRAT Clients

on: [push]

jobs:
  build:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: 1.21
      - name: Build clients
        run: .\build-clients.ps1 -Release
      - uses: actions/upload-artifact@v2
        with:
          name: clients
          path: bin/
```

## Performance Benchmarks

Build times (approximate, on modern hardware):

| Target | Time |
|--------|------|
| windows-amd64 | 2-3s |
| windows-386 | 2-3s |
| linux-amd64 | 2-3s |
| linux-386 | 2-3s |
| linux-arm | 3-5s |
| linux-arm64 | 3-5s |
| darwin-amd64 | 2-3s |
| darwin-arm64 | 2-3s |

**Total time for all targets:** ~20-30 seconds

## Binary Sizes

Approximate binary sizes (release build without debug symbols):

| Target | Size |
|--------|------|
| windows-amd64 | 8-10 MB |
| windows-386 | 7-9 MB |
| linux-amd64 | 6-8 MB |
| linux-386 | 5-7 MB |
| linux-arm | 5-7 MB |
| linux-arm64 | 6-8 MB |
| darwin-amd64 | 7-9 MB |
| darwin-arm64 | 6-8 MB |

Sizes can vary based on Go version and build flags.

## Next Steps

1. **Build clients:** Run `build-clients.bat` or `.\build-clients.ps1`
2. **Deploy binaries:** Distribute to target systems
3. **Configure settings:** Set server URL for each client
4. **Start server:** Run server binary or container
5. **Monitor dashboard:** View connected clients at http://localhost:8080

## See Also
- `README.md` - Project overview
- `WINDOWS_BUILD_GUIDE.md` - Detailed Windows build instructions
- `START_HERE.md` - Quick start guide
