# Code Organization & Package Refactoring Plan

## Current vs. Proposed Structure

### Current Structure (Before)
```
server/
  handlers.go          (1068 lines - mixed concerns)
  client_manager.go    (290 lines)
  client_store.go      (removed; storage moved to pkg/storage)
  web_handlers.go      (business logic mixed with HTTP)
  proxy_handler.go     (tunneling logic)
  terminal_proxy.go    (terminal sessions)
  admin_setup.go       (admin endpoints)
  admin_models.go      (admin API)
  session.go           (session management)
  utils.go
  errors.go
  main.go
  logging_guide.go
  services.go
  instance.go

client/
  main.go              (1563 lines - monolithic)
  command.go
  file_browser*.go
  terminal.go
  keylogger*.go
  screenshot*.go
  system_stats*.go
  daemon*.go
  autostart*.go
  updater.go
  instance.go
  machine_id*.go
  errors.go
```

### Proposed Structure (After)
```
cmd/
  server/
    main.go            (entrypoint only)
  client/
    main.go            (entrypoint only)

pkg/
  api/                 (HTTP API handlers)
    handlers.go        (REST endpoints)
    middleware.go      (auth, logging, cors)
    errors.go          (API error responses)
    admin.go           (admin API)
    
  storage/             (Data persistence)
    store.go           (interface & SQLite impl)
    models.go          (data structures)
    migrations.go      (schema management)
    
  auth/                (Authentication & sessions)
    authenticator.go   (client auth)
    session.go         (session mgmt)
    middleware.go      (auth middleware)
    
  messaging/           (WebSocket & routing)
    client.go          (client connection)
    registry.go        (connection tracking)
    handlers.go        (message handlers)
    broadcaster.go     (message routing)
    
  proxy/               (Tunneling)
    manager.go         (proxy lifecycle)
    pool.go            (connection pooling)
    tunnel.go          (tunnel implementation)
    
  terminal/            (Remote terminals)
    session.go         (terminal sessions)
    pty.go             (pseudo-terminal)
    io.go              (input/output)
    
  logger/              (Logging) ✓ EXISTS
  config/              (Configuration) ✓ EXISTS
  interfaces/          (Abstractions) ✓ EXISTS

server/
  server.go            (core server logic)
  services.go          (DI container) ✓ EXISTS
  main.go              (legacy for now)

client/
  client.go            (core client logic)
  connection.go        (WebSocket connection)
  commands.go          (command handlers)
  main.go              (legacy for now)

common/
  protocol.go          (message definitions)
  utils.go             (shared utilities)
```

---

## Benefits of This Organization

### 1. **Clear Separation of Concerns**
| Concern | Package | Benefits |
|---------|---------|----------|
| HTTP Request Handling | `pkg/api/` | Changes to REST API don't affect business logic |
| Data Persistence | `pkg/storage/` | Easy to swap backends (SQLite → PostgreSQL) |
| Client Auth | `pkg/auth/` | Centralized auth logic, easier to add new auth methods |
| Message Routing | `pkg/messaging/` | WebSocket logic isolated from handlers |
| Proxy Tunneling | `pkg/proxy/` | Tunnel implementation details hidden |
| Terminal Sessions | `pkg/terminal/` | PTY handling isolated |

### 2. **Improved Testability**
```go
// Before: Hard to test handlers without full server
func TestHandler(t *testing.T) {
    // Need to set up entire Server struct with all dependencies
    server := NewServer(config)
    // Can't test without actual database, WebSocket, etc.
}

// After: Easy to mock with interfaces
func TestAPIHandler(t *testing.T) {
    mockStore := &MockStorage{}
    mockRegistry := &MockRegistry{}
    handler := api.NewHandler(mockStore, mockRegistry)
    // Test just the handler logic
}
```

### 3. **Reduced File Sizes**
- `handlers.go` (1068 lines) → Split across multiple packages
- `client/main.go` (1563 lines) → Split into logical modules

### 4. **Better Code Discovery**
- All API handlers in `pkg/api/`
- All storage logic in `pkg/storage/`
- All authentication in `pkg/auth/`

### 5. **Easier to Add Features**
```go
// Adding a new message type handler
// Old: Add to giant switch statement in handlers.go
// New: Add to pkg/messaging/handlers.go with single responsibility

// Adding a new API endpoint
// Old: Find space in web_handlers.go (800+ lines)
// New: Add to pkg/api/handlers.go with clear location
```

---

## Migration Path

### Phase 1: Create Package Structure (NOW)
- ✅ Create all pkg/* directories
- ✅ Create doc.go files explaining each package
- Create package refactoring plan (this file)

### Phase 2: Extract Interfaces (Next)
- Move storage logic to `pkg/storage/`
- Move auth logic to `pkg/auth/`
- Move messaging logic to `pkg/messaging/`
- Move proxy logic to `pkg/proxy/`
- Move terminal logic to `pkg/terminal/`
- Move API logic to `pkg/api/`

### Phase 3: Update Imports (After Phase 2)
- Update `server/handlers.go` to import from new packages
- Update `server/main.go` to wire up new packages
- Update tests to use new imports

### Phase 4: Cleanup (Final)
- Archive old monolithic files (handlers.go, web_handlers.go, etc.)
- Clean up imports
- Update documentation

---

## File Migration Guide

### Storage Package Migration
```
OLD: server/client_store.go (787 lines) — removed, replaced by pkg/storage/sqlite.go
NEW: pkg/storage/store.go (logic only)
NEW: pkg/storage/models.go (data structures)
NEW: pkg/storage/migrations.go (schema)

OLD: imports: server.NewClientStore()
NEW: imports: storage.NewStore()
```

### API Package Migration
```
OLD: server/web_handlers.go (HTTP handlers mixed with logic)
NEW: pkg/api/handlers.go (pure HTTP handlers)
NEW: pkg/api/middleware.go (auth, logging, CORS)

OLD: Server.HandleRequest()
NEW: api.Handler.ServeHTTP()
```

### Authentication Migration
```
OLD: server/session.go + server/utils.go
NEW: pkg/auth/session.go (user sessions)
NEW: pkg/auth/authenticator.go (client auth)
NEW: pkg/auth/middleware.go (auth middleware)

OLD: imports: server.NewSessionManager()
NEW: imports: auth.NewSessionManager()
```

### Messaging Migration
```
OLD: server/client_manager.go (290 lines, mixed concerns)
NEW: pkg/messaging/registry.go (client tracking)
NEW: pkg/messaging/client.go (connection wrapper)
NEW: pkg/messaging/broadcaster.go (routing)
NEW: pkg/messaging/handlers.go (message handlers)

OLD: server.ClientManager.Register()
NEW: messaging.Registry.Register()
```

### Proxy Migration
```
OLD: server/proxy_handler.go (1000+ lines)
NEW: pkg/proxy/manager.go (lifecycle)
NEW: pkg/proxy/pool.go (connection pooling)
NEW: pkg/proxy/tunnel.go (tunnel implementation)

OLD: server.ProxyManager.CreateProxy()
NEW: proxy.Manager.CreateProxy()
```

### Terminal Migration
```
OLD: server/terminal_proxy.go
NEW: pkg/terminal/session.go (session management)
NEW: pkg/terminal/pty.go (pseudo-terminal)
NEW: pkg/terminal/io.go (input/output)

OLD: server.TerminalProxy.StartSession()
NEW: terminal.Proxy.StartSession()
```

---

## Example: Refactoring client_manager.go

### Before (Monolithic)
```go
// server/client_manager.go (290 lines)
package server

type ClientManager struct {
    clients map[string]*Client
    register chan *Client
    // ... many responsibilities
}

func (m *ClientManager) Register(client *Client) { }
func (m *ClientManager) Unregister(clientID string) { }
func (m *ClientManager) Broadcast(msg *Message) { }
func (m *ClientManager) SendToClient(id string, msg *Message) { }
// ... many more methods
```

### After (Separated)
```go
// pkg/messaging/registry.go
package messaging

type Registry struct {
    clients map[string]Connection
    register chan Connection
}

func (r *Registry) Register(conn Connection) error { }
func (r *Registry) Unregister(id string) error { }
func (r *Registry) Get(id string) (Connection, error) { }
func (r *Registry) GetAll() []Connection { }

// pkg/messaging/broadcaster.go
package messaging

type Broadcaster struct {
    registry *Registry
}

func (b *Broadcaster) Broadcast(msg *Message) error { }
func (b *Broadcaster) SendToClient(id string, msg *Message) error { }

// pkg/messaging/handlers.go
package messaging

type MessageHandler struct {
    registry *Registry
    storage Storage
}

func (h *MessageHandler) HandleMessage(ctx context.Context, msg *Message) error { }
```

---

## Integration Points

### Services Container (Already Exists)
The `Services` container will wire up all new packages:

```go
// server/services.go (updated)
type Services struct {
    Config      *config.ServerConfig
    Logger      *logger.Logger
    Storage     storage.Store              // NEW
    Auth        auth.Authenticator         // NEW
    Registry    messaging.Registry         // NEW
    Broadcaster messaging.Broadcaster      // NEW
    ProxyMgr    proxy.Manager              // NEW
    TermProxy   terminal.Proxy             // NEW
}

func NewServices(cfg *config.ServerConfig) (*Services, error) {
    storage := storage.NewStore(cfg.Database.Path)
    auth := auth.NewAuthenticator()
    registry := messaging.NewRegistry()
    broadcaster := messaging.NewBroadcaster(registry)
    proxyMgr := proxy.NewManager(registry)
    termProxy := terminal.NewProxy(registry)
    
    return &Services{
        Storage: storage,
        Auth: auth,
        Registry: registry,
        Broadcaster: broadcaster,
        ProxyMgr: proxyMgr,
        TermProxy: termProxy,
    }, nil
}
```

---

## Implementation Checklist

### Phase 1: Structure (NOW)
- [x] Create pkg/api/ directory
- [x] Create pkg/storage/ directory
- [x] Create pkg/auth/ directory
- [x] Create pkg/messaging/ directory
- [x] Create pkg/proxy/ directory
- [x] Create pkg/terminal/ directory
- [ ] Create PACKAGE_STRUCTURE.md (this file)

### Phase 2: Storage Package
- [ ] Create `pkg/storage/store.go` with interface
- [ ] Create `pkg/storage/models.go`
- [ ] Create `pkg/storage/migrations.go`
- [x] Move logic from `server/client_store.go`
- [ ] Add tests

### Phase 3: Auth Package
- [ ] Create `pkg/auth/authenticator.go`
- [ ] Create `pkg/auth/session.go`
- [ ] Create `pkg/auth/middleware.go`
- [ ] Move logic from `server/session.go` and `server/utils.go`
- [ ] Add tests

### Phase 4: Messaging Package
- [ ] Create `pkg/messaging/registry.go`
- [ ] Create `pkg/messaging/client.go`
- [ ] Create `pkg/messaging/handlers.go`
- [ ] Create `pkg/messaging/broadcaster.go`
- [ ] Move logic from `server/client_manager.go`
- [ ] Add tests

### Phase 5: Proxy Package
- [ ] Create `pkg/proxy/manager.go`
- [ ] Create `pkg/proxy/pool.go`
- [ ] Create `pkg/proxy/tunnel.go`
- [ ] Move logic from `server/proxy_handler.go`
- [ ] Add tests

### Phase 6: Terminal Package
- [ ] Create `pkg/terminal/session.go`
- [ ] Create `pkg/terminal/pty.go`
- [ ] Create `pkg/terminal/io.go`
- [ ] Move logic from `server/terminal_proxy.go`
- [ ] Add tests

### Phase 7: API Package
- [ ] Create `pkg/api/handlers.go`
- [ ] Create `pkg/api/middleware.go`
- [ ] Create `pkg/api/errors.go`
- [ ] Create `pkg/api/admin.go`
- [ ] Move logic from `server/web_handlers.go` and `server/admin_*.go`
- [ ] Add tests

### Phase 8: Update Services
- [ ] Update `server/services.go` to wire new packages
- [ ] Update `server/main.go` to use new Services
- [ ] Update imports throughout

### Phase 9: Testing
- [ ] Unit tests for each package
- [ ] Integration tests
- [ ] Build verification

### Phase 10: Cleanup
- [ ] Archive old files (with git history preserved)
- [ ] Update documentation
- [ ] Final build and test

---

## Example Package File Structure

### pkg/storage/
```
store.go           - Storage interface and SQLite implementation (350 lines)
models.go          - Client, Proxy, User, Settings data structures (200 lines)
migrations.go      - Database schema and migrations (150 lines)
errors.go          - Storage-specific errors (50 lines)
```

### pkg/api/
```
handlers.go        - REST endpoint handlers (300 lines)
middleware.go      - Auth, logging, CORS middleware (150 lines)
errors.go          - API error response formatting (100 lines)
admin.go           - Admin API endpoints (200 lines)
types.go           - Request/response types (150 lines)
```

### pkg/messaging/
```
registry.go        - Client connection registry (150 lines)
client.go          - Client connection wrapper (100 lines)
handlers.go        - Message type handlers (400 lines)
broadcaster.go     - Message routing and broadcasting (100 lines)
types.go           - Message-related types (100 lines)
```

---

## Benefits Summary

| Benefit | Impact | Timeline |
|---------|--------|----------|
| Reduced monolithic files | High | Immediate |
| Easier testing | High | After phase 2 |
| Better code discovery | Medium | Immediate |
| Loose coupling | High | After phase 8 |
| Easier onboarding | Medium | After documentation |
| Scalability | High | After all phases |

---

## Next Steps

1. Review this structure document
2. Start with Phase 2: Storage Package (least disruptive)
3. Incrementally move through other packages
4. Update tests after each phase
5. Document changes in README

