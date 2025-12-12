# Code Organization and Package Structure Guide

## Executive Summary

This document outlines a phased approach to reorganizing the goRAT codebase from monolithic packages into well-defined, modular packages. This improves testability, maintainability, and scalability.

---

## Current State Analysis

### Server Package Issues

**File Size Problems:**
- `handlers.go` - 1,068 lines (mixed concerns)
- `client_store.go` (removed) - storage logic now lives in `pkg/storage`
- `proxy_handler.go` - 1,000+ lines (tunneling)
- `web_handlers.go` - 800+ lines (REST + business logic)

**Mixed Responsibilities:**
- HTTP handlers with database logic
- Business logic with infrastructure code
- Authentication mixed with session management
- Proxy tunneling with connection pooling

### Client Package Issues
- `main.go` - 1,563 lines (monolithic)
- Mixed concerns (daemon, commands, terminal, file ops)
- Difficult to test individual components

---

## Proposed Architecture

### Layer 1: HTTP/API Layer (`pkg/api/`)
Handles REST API endpoints and HTTP middleware.

**Files:**
- `handlers.go` - REST endpoints
- `middleware.go` - Auth, logging, CORS
- `errors.go` - Error formatting
- `admin.go` - Admin endpoints

**Responsibilities:**
- Parse HTTP requests
- Format responses
- Apply auth middleware
- Handle CORS

**NOT responsible for:**
- Business logic
- Database queries
- Message routing

### Layer 2: Business Logic Layer

#### Authentication (`pkg/auth/`)
Handles client authentication and session management.

**Files:**
- `authenticator.go` - Client auth
- `session.go` - Session lifecycle
- `middleware.go` - Auth middleware

#### Messaging (`pkg/messaging/`)
Handles WebSocket communication and message routing.

**Files:**
- `registry.go` - Connection tracking
- `client.go` - Connection wrapper
- `handlers.go` - Message handlers
- `broadcaster.go` - Routing

#### Proxy Management (`pkg/proxy/`)
Handles proxy tunnel creation and management.

**Files:**
- `manager.go` - Lifecycle
- `pool.go` - Connection pooling
- `tunnel.go` - Tunnel implementation

#### Terminal Sessions (`pkg/terminal/`)
Handles remote terminal operations.

**Files:**
- `session.go` - Session management
- `pty.go` - Pseudo-terminal
- `io.go` - I/O handling

### Layer 3: Data Layer (`pkg/storage/`)
Handles all data persistence.

**Files:**
- `store.go` - Storage interface & implementation
- `models.go` - Data structures
- `migrations.go` - Schema management

### Layer 4: Common (`pkg/`)
Shared infrastructure.

**Existing packages:**
- `logger/` - Structured logging ✓
- `config/` - Configuration ✓
- `interfaces/` - Abstractions ✓

---

## Migration Phases

### Phase 1: Preparation (CURRENT)
- Create directory structure
- Document migration plan
- Set up placeholder files

**Estimated effort:** 2-4 hours

### Phase 2: Storage Extraction (NEXT)
Migrate (done): `server/client_store.go` -> `pkg/storage/`

**Tasks:**
1. Create storage interface
2. Copy SQLite implementation
3. Move data models
4. Copy migration logic
5. Add tests
6. Update imports in server

**Estimated effort:** 4-6 hours

### Phase 3: Auth Extraction
Migrate `server/session.go` + `server/utils.go` to `pkg/auth/`

**Estimated effort:** 2-3 hours

### Phase 4: Messaging Extraction
Migrate `server/client_manager.go` to `pkg/messaging/`

**Estimated effort:** 3-4 hours

### Phase 5: Proxy Extraction
Migrate `server/proxy_handler.go` to `pkg/proxy/`

**Estimated effort:** 3-4 hours

### Phase 6: Terminal Extraction
Migrate `server/terminal_proxy.go` to `pkg/terminal/`

**Estimated effort:** 2-3 hours

### Phase 7: API Extraction
Migrate `server/web_handlers.go` + `server/admin_*.go` to `pkg/api/`

**Estimated effort:** 4-6 hours

### Phase 8: Integration
- Update services container
- Wire all packages
- Update main.go
- Integration testing

**Estimated effort:** 4-6 hours

### Phase 9: Testing & Cleanup
- Add unit tests for each package
- Add integration tests
- Archive old files
- Final verification

**Estimated effort:** 6-8 hours

**Total Timeline:** 30-44 hours (1-2 weeks of focused work)

---

## Before and After Examples

### Example 1: Storage Operations

**Before:**
```go
// In server/handlers.go
func (s *Server) HandleClientList(w http.ResponseWriter, r *http.Request) {
    clients, err := s.store.GetAllClients()
    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(clients)
}
```

**After:**
```go
// In pkg/api/handlers.go
type APIHandler struct {
    storage storage.Store
}

func (h *APIHandler) ListClients(c *gin.Context) {
    clients, err := h.storage.GetAllClients()
    if err != nil {
        c.JSON(http.StatusInternalServerError, api.Error("database_error", err))
        return
    }
    c.JSON(http.StatusOK, clients)
}

// In pkg/storage/store.go
type Store interface {
    GetAllClients() ([]*Client, error)
    SaveClient(*Client) error
    // ...
}

type SQLiteStore struct {
    db *sql.DB
}

func (s *SQLiteStore) GetAllClients() ([]*Client, error) {
    // Implementation
}
```

### Example 2: Message Handling

**Before:**
```go
// In server/handlers.go (1000+ lines)
func (s *Server) handleMessage(client *Client, msg *Message) error {
    switch msg.Type {
    case "command":
        // Handle command
    case "screenshot":
        // Handle screenshot
    case "keylogger":
        // Handle keylogger
    // ... 50 more cases ...
    }
}
```

**After:**
```go
// In pkg/messaging/handlers.go
type MessageHandler struct {
    registry messaging.Registry
    storage storage.Store
}

func (h *MessageHandler) HandleCommand(ctx context.Context, msg *Message) error {
    // Single responsibility: handle command messages
}

func (h *MessageHandler) HandleScreenshot(ctx context.Context, msg *Message) error {
    // Single responsibility: handle screenshot
}

// Routed from pkg/messaging/registry.go
func (r *Registry) RouteMessage(clientID string, msg *Message) error {
    handler := r.handlers[msg.Type]
    return handler.Handle(context.Background(), msg)
}
```

### Example 3: Proxy Operations

**Before:**
```go
// In server/proxy_handler.go (1000+ lines)
type ProxyManager struct {
    manager *ClientManager
    store *ClientStore
    connections map[string]*ProxyConnection
    pools map[string]*ConnectionPool
    mu sync.RWMutex
    // ... 20 more fields ...
}
```

**After:**
```go
// In pkg/proxy/manager.go
type Manager struct {
    registry messaging.Registry
    storage storage.Store
}

func (m *Manager) CreateProxy(clientID, remoteHost string, port int) (string, error) {
    // Create proxy
}

// In pkg/proxy/pool.go
type Pool struct {
    connections []*PooledConnection
}

func (p *Pool) Get() (net.Conn, error) {
    // Get pooled connection
}

// In pkg/proxy/tunnel.go
type Tunnel struct {
    id string
    conn net.Conn
}

func (t *Tunnel) Forward() error {
    // Forward data
}
```

---

## Package Interfaces

### Storage Package
```go
type Store interface {
    // Clients
    SaveClient(client *Client) error
    GetClient(id string) (*Client, error)
    GetAllClients() ([]*Client, error)
    DeleteClient(id string) error
    
    // Proxies
    SaveProxy(proxy *Proxy) error
    GetProxy(id string) (*Proxy, error)
    DeleteProxy(id string) error
    
    // Users
    GetUser(username string) (*User, error)
    SaveUser(user *User) error
}
```

### Messaging Package
```go
type Registry interface {
    Register(conn Connection) error
    Unregister(id string) error
    Get(id string) (Connection, error)
    GetAll() []Connection
    Broadcast(msg *Message) error
}

type MessageHandler interface {
    Handle(ctx context.Context, clientID string, msg *Message) error
}
```

### Auth Package
```go
type Authenticator interface {
    Authenticate(payload *AuthPayload) (bool, error)
    ValidateToken(token string) (bool, error)
}

type SessionManager interface {
    CreateSession(userID, username string) (string, error)
    ValidateSession(sessionID string) (bool, error)
    RevokeSession(sessionID string) error
}
```

### Proxy Package
```go
type Manager interface {
    CreateProxy(clientID, remoteHost string, remotePort int) (string, error)
    CloseProxy(proxyID string) error
    GetProxy(proxyID string) (*Proxy, error)
    ListAll() ([]*Proxy, error)
}

type Pool interface {
    Get() (net.Conn, error)
    Put(conn net.Conn) error
    Close() error
}
```

---

## Implementation Strategy

### Strategy 1: Incremental Refactoring (RECOMMENDED)
Migrate one package at a time while keeping old code.

**Advantages:**
- Low risk of breaking existing functionality
- Can test incrementally
- Easy to revert if needed

**Approach:**
1. Create new package with interface
2. Copy logic from old location
3. Update imports gradually
4. Once complete, archive old code

### Strategy 2: Big Bang Refactoring
Refactor all at once.

**Advantages:**
- Faster completion
- Cleaner git history

**Disadvantages:**
- High risk
- Harder to debug issues
- Large PR difficult to review

### Recommended: Incremental with Feature Branches
```bash
# Feature branch for each package
git checkout -b refactor/storage-package
# Migrate storage package
git commit -m "refactor: extract storage to pkg/storage"
git push origin refactor/storage-package

# Create PR for review
# Once approved and merged, move to next package
```

---

## Testing Strategy

### Unit Tests Per Package
```
pkg/storage/store_test.go      - Storage interface tests
pkg/auth/authenticator_test.go - Auth logic tests
pkg/messaging/registry_test.go - Message registry tests
pkg/proxy/manager_test.go      - Proxy management tests
pkg/terminal/session_test.go   - Terminal session tests
pkg/api/handlers_test.go       - API handler tests
```

### Integration Tests
```
integration_test.go            - Cross-package tests
e2e_test.go                   - End-to-end scenarios
```

### Example Unit Test
```go
// pkg/storage/store_test.go
func TestStoreSaveClient(t *testing.T) {
    store := NewMemoryStore() // Mock implementation
    
    client := &Client{ID: "test123", Hostname: "test-host"}
    err := store.SaveClient(client)
    
    require.NoError(t, err)
    
    retrieved, _ := store.GetClient("test123")
    require.Equal(t, "test-host", retrieved.Hostname)
}
```

---

## Rollout Plan

### Week 1: Planning & Preparation
- Day 1-2: Review and finalize architecture
- Day 3-4: Create package structure (DONE)
- Day 5: Team discussion and buy-in

### Week 2-3: Phase 2-5 (Storage, Auth, Messaging, Proxy)
- One package per 1-2 days
- Code review after each
- Incremental integration

### Week 3-4: Phase 6-7 (Terminal, API)
- Complete remaining packages
- Full integration testing

### Week 4-5: Testing & Cleanup
- Comprehensive testing
- Documentation updates
- Archive old code
- Performance verification

---

## Risk Mitigation

### Risk: Breaking Existing Code
**Mitigation:**
- Keep old code during migration
- Add wrapper functions
- Comprehensive test suite
- Gradual import updates

### Risk: Merge Conflicts
**Mitigation:**
- Use feature branches
- Frequent small PRs
- Regular rebases
- Clear communication

### Risk: Performance Degradation
**Mitigation:**
- Profile before/after
- Minimize indirection in hot paths
- Keep interfaces slim
- Benchmark critical operations

---

## Success Criteria

| Criteria | Metric |
|----------|--------|
| Code Organization | All packages < 300 lines (except api & storage) |
| Test Coverage | > 80% coverage per package |
| Build Time | No increase in build time |
| Performance | No measurable performance degradation |
| Documentation | All packages documented with examples |
| Team Adoption | 100% of new code follows structure |

---

## Next Steps

1. **Review this document** - Get team alignment
2. **Start Phase 2** - Begin storage package migration
3. **Create feature branch** - `refactor/storage-package`
4. **Migrate storage** - Follow the plan
5. **Code review** - Get team feedback
6. **Merge and iterate** - Move to next package

---

## Questions?

Refer to:
- `PACKAGE_STRUCTURE_REFACTORING.md` - Detailed migration guide
- `ARCHITECTURE_IMPROVEMENTS.md` - Overall architecture
- `DEVELOPER_MIGRATION_GUIDE.md` - Code migration examples

