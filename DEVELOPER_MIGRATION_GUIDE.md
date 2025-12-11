# Developer Migration Guide

## Overview

Three major architectural improvements have been implemented. This guide helps you migrate existing code to use the new systems.

---

## 1. Migrating to Structured Logging

### Before (Old Way)

```go
import "log"

func handleClient(clientID string) {
    log.Printf("Client connected: %s\n", clientID)
    log.Fatal("Connection failed")
    log.Println("Client disconnected")
}
```

### After (New Way)

```go
import "gorat/pkg/logger"

func handleClient(clientID string) {
    log := logger.Get()
    
    log.InfoWith("client_connected", "client_id", clientID)
    log.ErrorWithErr("connection_failed", err)
    log.InfoWith("client_disconnected", "client_id", clientID)
}
```

### Migration Checklist

- [ ] Replace `log.Printf()` → `log.InfoWith(msg, key, value)`
- [ ] Replace `log.Fatal()` → `log.ErrorWithErr(msg, err); return`
- [ ] Replace `log.Println()` → `log.InfoWith(msg)`
- [ ] Replace `log.Fatalf()` → `log.ErrorWithErr(msg, err); return`
- [ ] Add contextual attributes (client_id, user_id, etc.)
- [ ] Test with `LOG_FORMAT=json` to verify output

### Example: client_manager.go

**Before:**
```go
func (m *ClientManager) Register(client *Client) {
    log.Printf("Client registered: %s from %s", client.ID, client.Metadata.IP)
    m.clients[client.ID] = client
}
```

**After:**
```go
func (m *ClientManager) Register(client *Client) {
    log := logger.Get()
    log.InfoWith("client_registered", 
        "client_id", client.ID, 
        "ip", client.Metadata.IP,
        "hostname", client.Metadata.Hostname)
    m.clients[client.ID] = client
}
```

### Logging Levels Guide

```go
// DEBUG - Development information, verbose
log.DebugWith("cache_lookup", "key", cacheKey, "hit", true)

// INFO - Normal operations
log.InfoWith("client_connected", "client_id", id, "ip", ip)

// WARN - Warning conditions
log.WarnWith("slow_operation", "duration_ms", 5000, "threshold_ms", 1000)

// ERROR - Error conditions
log.ErrorWithErr("database_error", err, "query", "SELECT * FROM clients")
```

---

## 2. Using Configuration Files

### Before (Hardcoded/Flags Only)

```go
// In main.go
addr := flag.String("addr", ":8080", "Server address")
webUser := flag.String("web-user", "admin", "Web UI username")
// ... many flags ...
```

### After (File-Based + Overrides)

```yaml
# config.yaml
address: ":8080"

webui:
  username: admin
  password: admin123

database:
  path: "./clients.db"
  max_connections: 25

logging:
  level: info
  format: json
```

```go
// In main.go
cfg, err := config.LoadConfig(*configPath)
if err != nil {
    log.ErrorWithErr("config load failed", err)
    return
}
```

### Configuration Precedence (lowest to highest)

1. **Defaults** (hardcoded in code)
2. **Config file** (config.yaml)
3. **Environment variables** (SERVER_ADDR=:9000)
4. **Command-line flags** (--addr :9000)

### Example: Production Setup

Create `config-prod.yaml`:
```yaml
address: ":443"

tls:
  enabled: true
  cert_file: "/etc/certs/server.crt"
  key_file: "/etc/certs/server.key"

database:
  path: "/data/clients.db"
  max_connections: 50

logging:
  level: warn
  format: json
```

Run:
```bash
./bin/server -config config-prod.yaml
```

### Environment Variables

Override any config value without code changes:

```bash
# Development
LOG_LEVEL=debug LOG_FORMAT=text ./bin/server

# Docker
docker run -e SERVER_ADDR=:8080 -e LOG_LEVEL=info myapp

# Kubernetes
env:
- name: SERVER_ADDR
  value: ":8080"
- name: LOG_LEVEL
  value: "info"
```

---

## 3. Using Services Container

### Before (No DI)

```go
// Components created throughout
func main() {
    manager := NewClientManager()
    store := NewClientStore("clients.db")
    sessionMgr := NewSessionManager(...)
    // ... hard to test, tightly coupled
}
```

### After (With DI)

```go
// Centralized initialization
func main() {
    cfg, _ := config.LoadConfig(*configPath)
    services, _ := NewServices(cfg)
    
    // Access any service
    services.Logger.InfoWith("server_starting")
    services.Storage.SaveClient(client)
}
```

### Accessing Services in Handlers

**Before:**
```go
func (s *Server) HandleMessage(client *Client, msg *Message) {
    // Needs to access global variables
    s.manager.GetClient(msg.ID)
    s.store.SaveClient(client)
}
```

**After:**
```go
func (h *Handler) HandleMessage(ctx context.Context, services *Services, msg *Message) {
    client, err := services.Storage.GetClient(msg.ID)
    services.Logger.InfoWith("message_handled", "msg_type", msg.Type)
}
```

### Service Interface Usage

```go
// Instead of accessing module globals
s.manager.SendMessage(id, msg)
s.store.SaveClient(client)

// Use services container
services.ClientMgr.SendMessage(id, msg)
services.Storage.SaveClient(client)
```

---

## 4. Practical Migration Steps

### Step 1: Update Imports

```go
// Add to imports
import (
    "gorat/pkg/logger"
    "gorat/pkg/config"
)
```

### Step 2: Initialize in main()

```go
// In main.go
logger.Init(logger.LogLevel(*logLevel), *logFormat)
cfg, err := config.LoadConfig(*configPath)
services, err := NewServices(cfg)
```

### Step 3: Replace log.* Calls

```bash
# Find all old log usage
grep -r "log\.Printf" server/
grep -r "log\.Fatal" server/
grep -r "log\.Println" server/

# Replace systematically
```

### Step 4: Add Context

When logging, include relevant context:

```go
// Weak
log.InfoWith("message received")

// Better
log.InfoWith("message_received", "client_id", clientID, "type", msg.Type)

// Best
log.InfoWith("message_received", 
    "client_id", clientID, 
    "msg_type", msg.Type,
    "size_bytes", len(msg.Payload),
    "duration_ms", elapsed)
```

### Step 5: Create Environment Configs

```bash
# Development
cp config.example.yaml config-dev.yaml
# Edit for dev settings

# Production
cp config.example.yaml config-prod.yaml
# Edit for production settings
```

### Step 6: Test Logging Output

```bash
# Text format (human readable)
LOG_FORMAT=text LOG_LEVEL=debug ./bin/server

# JSON format (for parsing)
LOG_FORMAT=json LOG_LEVEL=info ./bin/server 2>&1 | jq '.'

# Production-like
LOG_FORMAT=json LOG_LEVEL=warn ./bin/server
```

---

## 5. Common Migration Patterns

### Pattern 1: Error Handling

**Before:**
```go
if err != nil {
    log.Fatal(err)
}
```

**After:**
```go
if err != nil {
    log := logger.Get()
    log.ErrorWithErr("operation_failed", err)
    return err
}
```

### Pattern 2: Operation Logging

**Before:**
```go
fmt.Printf("Processing %d clients\n", len(clients))
```

**After:**
```go
logger.Get().InfoWith("processing_clients", "count", len(clients))
```

### Pattern 3: Debug Tracing

**Before:**
```go
if debug {
    log.Printf("Cache hit for %s\n", key)
}
```

**After:**
```go
logger.Get().DebugWith("cache_lookup", "key", key, "hit", true)
```

### Pattern 4: Warnings

**Before:**
```go
if elapsed > timeout {
    log.Printf("WARNING: Operation took %dms\n", elapsed)
}
```

**After:**
```go
if elapsed > timeout {
    logger.Get().WarnWith("slow_operation", 
        "duration_ms", elapsed, 
        "timeout_ms", timeout)
}
```

---

## 6. Testing Your Changes

### Unit Test Example

```go
func TestClientManager(t *testing.T) {
    // Initialize logger for test
    logger.Init(logger.DebugLevel, "text")
    
    // Your test code
    manager := NewClientManager()
    // ...
}
```

### Integration Test Example

```go
func TestWithServices(t *testing.T) {
    cfg := &config.ServerConfig{
        Database: config.DatabaseConfig{Path: ":memory:"},
        Logging:  config.LoggingConfig{Level: "debug", Format: "text"},
    }
    
    services, err := NewServices(cfg)
    require.NoError(t, err)
    
    // Test with actual services
    err = services.Storage.SaveClient(testClient)
    require.NoError(t, err)
}
```

---

## 7. Troubleshooting

### Issue: "undefined: logger"

**Solution:** Add to imports:
```go
import "gorat/pkg/logger"
```

### Issue: "undefined: config"

**Solution:** Add to imports:
```go
import "gorat/pkg/config"
```

### Issue: "invalid log level"

**Solution:** Use valid levels:
```go
logger.Init(logger.DebugLevel, "text")   // ✓
logger.Init(logger.VERBOSE, "text")      // ✗
```

### Issue: JSON logs aren't pretty

**Solution:** Use jq for formatting:
```bash
./bin/server 2>&1 | jq '.'
```

---

## 8. Best Practices

### DO ✓

- Add context to every log entry
- Use appropriate log levels
- Test with JSON format
- Create environment-specific configs
- Use services container for new code

### DON'T ✗

- Mix old `log` package with new logger
- Hardcode configuration values
- Log sensitive information (passwords, tokens)
- Ignore configuration validation errors
- Use printf-style logging with logger

---

## 9. Code Review Checklist

When reviewing code changes:

- [ ] Uses `logger.Get()` instead of `log`
- [ ] Log messages include context (client_id, etc.)
- [ ] No hardcoded configuration values
- [ ] Errors logged with `ErrorWithErr()`
- [ ] Appropriate log level used
- [ ] JSON format tested

---

## 10. References

- **Logger:** `pkg/logger/logger.go`
- **Config:** `pkg/config/config.go`
- **Services:** `server/services.go`
- **Examples:** `server/logging_guide.go`
- **Full Guide:** `ARCHITECTURE_IMPROVEMENTS.md`
- **Quick Reference:** `ARCHITECTURE_QUICK_START.md`

