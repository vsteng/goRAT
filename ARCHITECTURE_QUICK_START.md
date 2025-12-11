# Quick Start: New Architecture Features

## Overview

Three major architectural improvements have been implemented:

1. **Structured Logging** - Slog-based contextual logging
2. **Configuration Management** - YAML config + environment variables
3. **Dependency Injection** - Services container for loose coupling

---

## Building the Server

```bash
cd /Users/tengbozhang/chrom
go build -o ./bin/server ./cmd/server/main.go
```

---

## Configuration

### Option 1: Command-Line Flags (Backward Compatible)

```bash
./bin/server \
  -addr :8080 \
  -web-user admin \
  -web-pass admin123 \
  -log-level info \
  -log-format text
```

### Option 2: Configuration File + Overrides

```bash
# Create config.yaml
cp config.example.yaml config.yaml
edit config.yaml  # Customize as needed

# Run with config file
./bin/server -config config.yaml
```

### Option 3: Environment Variables

```bash
export SERVER_ADDR=:9000
export WEB_USERNAME=operator
export WEB_PASSWORD=secure123
export LOG_LEVEL=debug
export LOG_FORMAT=json
./bin/server
```

### Option 4: Mix All Three (Priority: Flags > Env Vars > File > Defaults)

```bash
LOG_LEVEL=warn ./bin/server \
  -config config.yaml \
  -addr :8080 \
  -log-format json
```

---

## Logging

### Using Structured Logging in Code

```go
import "gorat/pkg/logger"

func main() {
    // Initialize logger (done in main.go)
    logger.Init(logger.InfoLevel, "text")
    log := logger.Get()
    
    // Log with context
    log.InfoWith("user_login", "user_id", 123, "ip", "192.168.1.1")
    log.ErrorWithErr("database_error", err, "table", "users")
    log.WarnWith("slow_query", "duration_ms", 5000)
    log.DebugWith("cache_hit", "key", "user:123")
}
```

### Log Formats

**Text (default):**
```
2025-12-11T16:10:23.456Z	INFO	user_login	user_id=123	ip=192.168.1.1
```

**JSON (for log aggregation systems):**
```json
{"time":"2025-12-11T16:10:23.456Z","level":"INFO","msg":"user_login","user_id":123,"ip":"192.168.1.1"}
```

### Log Levels

- `debug` - Development/detailed debugging
- `info` - Normal operation
- `warn` - Warning conditions
- `error` - Error conditions

---

## Services Container

### Accessing Services

```go
// In handlers or business logic
services, err := NewServices(cfg)
if err != nil {
    log.ErrorWithErr("services_init_failed", err)
    return
}

// Access any service
services.Storage.SaveClient(client)
services.Logger.InfoWith("client_saved", "id", client.ID)
```

### Available Services

| Service | Purpose |
|---------|---------|
| `Config` | Server configuration |
| `Logger` | Structured logging |
| `Storage` | Database/persistence layer |
| `ClientMgr` | Client connection management |
| `ProxyMgr` | Proxy tunneling |
| `SessionMgr` | User session management |
| `TermProxy` | Remote terminal sessions |
| `Auth` | Authentication logic |

---

## Configuration File Reference

See `config.example.yaml` for all options:

```yaml
address: ":8080"

tls:
  enabled: false
  cert_file: ""
  key_file: ""

webui:
  username: admin
  password: admin
  port: 8080

database:
  path: "./clients.db"
  max_connections: 25

logging:
  level: info        # debug, info, warn, error
  format: text       # text or json
```

---

## Migration Checklist

### For Server Code

- [ ] Replace `log.Printf()` with `log.InfoWith(msg, key, value)`
- [ ] Replace `log.Fatal()` with `log.ErrorWithErr(msg, err); return`
- [ ] Replace `log.Println()` with `log.InfoWith(msg)`
- [ ] Add contextual attributes (client_id, user_id, request_id, etc.)
- [ ] Test with `LOG_FORMAT=json` to verify JSON output

### For Client Code

- [ ] Similar logging replacements
- [ ] Use same logger.Get() pattern
- [ ] Add operation context to logs

---

## Examples

### Example 1: Run with Debug Logging

```bash
LOG_LEVEL=debug LOG_FORMAT=text ./bin/server
```

Output:
```
DEBUG	client_manager_started
DEBUG	websocket_listener	port=8080
INFO	client_connected	client_id=abc123	ip=192.168.1.100
```

### Example 2: Production Setup

```bash
# config-prod.yaml
address: ":443"
tls:
  enabled: true
  cert_file: "/etc/certs/server.crt"
  key_file: "/etc/certs/server.key"
logging:
  level: warn      # Only warnings and errors
  format: json     # For log aggregation

# Run
./bin/server -config config-prod.yaml
```

### Example 3: Docker Environment

```bash
docker run \
  -e SERVER_ADDR=:8080 \
  -e LOG_LEVEL=info \
  -e LOG_FORMAT=json \
  -e DB_PATH=/data/clients.db \
  -v /data:/data \
  gorat-server
```

---

## Testing

### Test Logging Output

```bash
# Text format
LOG_LEVEL=debug LOG_FORMAT=text ./bin/server 2>&1 | head -20

# JSON format (for parsing)
LOG_LEVEL=info LOG_FORMAT=json ./bin/server 2>&1 | jq '.'
```

### Test Configuration Loading

```bash
# Should print loaded config
./bin/server -config config.example.yaml -h | head -5
```

---

## Troubleshooting

### "Config file not found"
```bash
cp config.example.yaml config.yaml
./bin/server -config config.yaml
```

### "Invalid log level"
Valid values: `debug`, `info`, `warn`, `error`
```bash
LOG_LEVEL=info ./bin/server
```

### "TLS cert not found"
If enabling TLS, provide valid cert/key files:
```bash
./bin/server -config config.yaml -cert /path/to/cert.crt -key /path/to/key.key
```

---

## Next Steps

1. Review `ARCHITECTURE_IMPROVEMENTS.md` for full details
2. Read `server/logging_guide.go` for integration patterns
3. Update existing log calls (see Migration Checklist)
4. Test with different log levels and formats
5. Deploy with appropriate configuration per environment

