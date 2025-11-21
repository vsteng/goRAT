# Client Build Versions

The client has two build configurations: **Debug** and **Release**.

## Debug Version

**Build command:**
```bash
make client-debug
# OR
go build -tags debug -o bin/client-debug cmd/client/main.go
```

**Default settings:**
- `daemon`: **false** (runs in foreground by default)
- `autostart`: **false** (no auto-start on boot)
- `logging`: **enabled** (logs to stderr or file if daemon)

**Behavior:**
- Logs are always enabled and verbose
- Logs to `stderr` by default
- When running as daemon, logs to `client_debug.log`
- Shows detailed debug information
- Ideal for development and troubleshooting

**Usage examples:**
```bash
# Run in foreground with logging
./bin/client-debug -server wss://yourserver.com/ws

# Run as daemon (logs to client_debug.log)
./bin/client-debug -server wss://yourserver.com/ws -daemon

# Override auto-start
./bin/client-debug -server wss://yourserver.com/ws -autostart
```

## Release Version

**Build command:**
```bash
make client-release
# OR
make client  # (default)
# OR
go build -o bin/client-release cmd/client/main.go
```

**Default settings:**
- `daemon`: **true** (runs in background by default)
- `autostart`: **false** (no auto-start on boot)
- `logging`: **disabled** (silent operation)

**Behavior:**
- Logs are disabled by default (outputs to `/dev/null`)
- Runs as background daemon by default
- Can enable logging with environment variable: `CLIENT_ENABLE_LOG=1`
- When logging is enabled and running as daemon, logs to `client.log`
- Silent operation for production deployments

**Usage examples:**
```bash
# Run as daemon (default, no logging)
./bin/client-release -server wss://yourserver.com/ws

# Run in foreground
./bin/client-release -server wss://yourserver.com/ws -daemon=false

# Enable logging via environment variable
CLIENT_ENABLE_LOG=1 ./bin/client-release -server wss://yourserver.com/ws

# Run with auto-start enabled
./bin/client-release -server wss://yourserver.com/ws -autostart
```

## Command-line Flags

Both versions support the same flags:

| Flag | Debug Default | Release Default | Description |
|------|--------------|-----------------|-------------|
| `-server` | `wss://localhost/ws` | `wss://localhost/ws` | Server WebSocket URL |
| `-daemon` | `false` | `true` | Run as background daemon/service |
| `-autostart` | `false` | `false` | Enable auto-start on boot |

**Note:** Command-line flags always override build defaults.

## Build All Platforms

To build both debug and release versions for all platforms:

```bash
make build-all
```

This creates:
- `bin/linux/client-debug` and `bin/linux/client-release`
- `bin/windows/client-debug.exe` and `bin/windows/client-release.exe`
- `bin/darwin/client-debug` and `bin/darwin/client-release`

## Checking Build Version

Run the client to see which build mode it is:

```bash
./bin/client-debug -server wss://test.com/ws
# Output: Client build mode: debug

./bin/client-release -server wss://test.com/ws
# Output: Client build mode: release
```

## Recommendations

- Use **debug version** for:
  - Development and testing
  - Troubleshooting connection issues
  - Understanding client behavior
  - Running interactively

- Use **release version** for:
  - Production deployments
  - Silent background operation
  - Minimal resource usage
  - Stealth operation
