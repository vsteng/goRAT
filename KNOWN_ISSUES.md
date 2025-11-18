# Known Issues

## macOS 15+ Screenshot Library Incompatibility

**Issue**: The `github.com/kbinani/screenshot` library uses deprecated APIs on macOS 15+.

**Error**:
```
error: 'CGDisplayCreateImageForRect' is unavailable: obsoleted in macOS 15.0
```

**Workaround Options**:

1. **Build without screenshot support** (client will build but screenshot feature won't work):
   ```bash
   GOOS=darwin GOARCH=amd64 go build -o bin/client cmd/client/main.go
   ```

2. **Use an older macOS SDK** (if available):
   ```bash
   SDKROOT=/path/to/older/sdk go build -o bin/client cmd/client/main.go
   ```

3. **Cross-compile from Linux**:
   ```bash
   GOOS=darwin GOARCH=amd64 go build -o bin/client-darwin cmd/client/main.go
   ```

4. **Alternative screenshot library** (requires code changes):
   - Consider replacing with a library that uses ScreenCaptureKit
   - Or implement screenshot functionality using ScreenCaptureKit directly

**Status**: The server and client_monitor build successfully on macOS 15. The client builds but screenshot functionality may not work properly.

**Recommendation**: Deploy the client on the target platform (Linux/Windows servers) where the library works correctly.

## Other Platform-Specific Notes

### Windows
- Keylogger requires proper Windows hooks implementation for production use
- Auto-start uses registry, ensure proper permissions

### Linux  
- Keylogger requires reading from `/dev/input/eventX`, needs root or proper permissions
- Systemd service installation requires user privileges
- Screenshot requires X11 or Wayland session

## Security Considerations

- Always use TLS certificates from a trusted CA in production
- Change default authentication tokens
- Keylogger functionality should comply with local laws and regulations
- Run client with minimal required permissions
