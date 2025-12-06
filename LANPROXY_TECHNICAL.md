# LanProxy Integration - Technical Reference

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                      Web UI Layer                                │
├─────────────────────────────────────────────────────────────────┤
│  dashboard-new.html │ client-details.html │ terminal.html       │
│        │                     │                     │             │
│        └─────────────────────┴─────────────────────┘             │
│                       │                                          │
├─────────────────────────────────────────────────────────────────┤
│                    API Layer (HTTP/REST)                         │
├─────────────────────────────────────────────────────────────────┤
│  /api/clients      /api/files      /api/processes               │
│  /api/proxy/*      /api/client     /api/screenshot              │
│        │                │                │                       │
└────────┼────────────────┼────────────────┼──────────────────────┘
         │                │                │
┌─────────────────────────────────────────────────────────────────┐
│                    Handler Layer (Go)                            │
├─────────────────────────────────────────────────────────────────┤
│  WebHandler (web_handlers.go)                                   │
│  ProxyHandler (proxy_handler.go)                                │
│  TerminalProxy (terminal_proxy.go)                              │
│        │                                                         │
└────────┼─────────────────────────────────────────────────────────┘
         │
┌─────────────────────────────────────────────────────────────────┐
│                  Business Logic Layer (Go)                       │
├─────────────────────────────────────────────────────────────────┤
│  ClientManager      ProxyManager      TerminalProxy             │
│  SessionManager     ClientStore                                 │
│        │                │                     │                 │
└────────┼────────────────┼─────────────────────┼─────────────────┘
         │                │                     │
┌─────────────────────────────────────────────────────────────────┐
│              Remote Client Connection Layer                      │
├─────────────────────────────────────────────────────────────────┤
│  WebSocket (/ws) ← TCP Connection ← Client Agents               │
└─────────────────────────────────────────────────────────────────┘
```

## File Locations

### Frontend Components
```
web/templates/
├── dashboard-new.html          (1089 lines) - Enhanced dashboard
├── client-details.html         (950 lines)  - Client control page
├── dashboard.html              (670 lines)  - Original dashboard
├── terminal.html               (256 lines)  - Terminal interface
├── files.html                  (407 lines)  - File browser
└── login.html                  (existing)   - Login page
```

### Backend Components
```
server/
├── proxy_handler.go            (504 lines)  - NEW Proxy management
├── handlers.go                 (703 lines)  - UPDATED Server definition
├── web_handlers.go             (588 lines)  - UPDATED Web routes
├── client_manager.go           (existing)   - Client lifecycle
├── terminal_proxy.go           (existing)   - Terminal sessions
├── session.go                  (existing)   - Session management
├── client_store.go             (existing)   - Database persistence
└── main.go                     (existing)   - Entry point
```

### Documentation
```
├── LANPROXY_INTEGRATION.md     (NEW) Implementation details
├── LANPROXY_QUICKSTART.md      (NEW) User guide
└── LANPROXY_TECHNICAL.md       (This file) Technical reference
```

## Data Structures

### ProxyConnection
```go
type ProxyConnection struct {
    ID          string          // Unique identifier
    ClientID    string          // Associated client
    LocalPort   int             // Server listening port
    RemoteHost  string          // Client target host
    RemotePort  int             // Client target port
    Protocol    string          // tcp, http, https
    Status      string          // active, inactive, error
    BytesIn     int64           // Download bytes
    BytesOut    int64           // Upload bytes
    CreatedAt   time.Time       // Creation timestamp
    LastActive  time.Time       // Last activity
    listener    net.Listener    // TCP listener
    mu          sync.RWMutex    // Thread safety
}
```

### Client (Enhanced)
```go
type Client struct {
    ID          string                          // Unique ID
    Conn        *websocket.Conn                // WebSocket connection
    Metadata    *common.ClientMetadata         // Client info
    Mu          sync.RWMutex                   // Thread safety
    Proxies     map[string]*ProxyConnection   // Active proxies
    // ... additional fields
}
```

### ClientMetadata (from common package)
```go
type ClientMetadata struct {
    ID          string          // Machine ID
    Hostname    string          // Computer name
    OS          string          // Operating system
    Arch        string          // Architecture
    IP          string          // Local IP
    PublicIP    string          // Public IP
    Status      string          // online/offline
    Version     string          // Client version
    LastSeen    time.Time       // Last heartbeat
    // ... additional fields
}
```

## API Endpoints Reference

### Dashboard & Pages
```
GET  /dashboard-new               - Enhanced dashboard page
GET  /client-details?id={ID}     - Client management page
GET  /terminal?client={ID}       - Terminal page (existing)
GET  /files?client={ID}          - File browser page (existing)
```

### Client Management APIs
```
GET  /api/clients                - List all clients
GET  /api/client?id={ID}         - Get specific client
POST /api/login                  - User authentication
POST /api/logout                 - User logout
```

### File Management APIs
```
GET  /api/files?client_id={ID}&path=/        - List files
GET  /api/files/browse?client_id={ID}        - Browse files (legacy)
GET  /api/files/drives?client_id={ID}        - Get drives (Windows)
GET  /api/files/download?file={path}         - Download file
POST /api/files/upload                        - Upload file (planned)
```

### Process Management APIs
```
GET  /api/processes?client_id={ID}           - List processes
POST /api/processes/kill?pid={PID}           - Kill process (planned)
GET  /api/processes/monitor?client_id={ID}   - Monitor process (planned)
```

### Proxy APIs
```
POST /api/proxy/create           - Create proxy tunnel
GET  /api/proxy/list?client_id=  - List proxies for client
GET  /api/proxy/list             - List all proxies
POST /api/proxy/close?id=        - Close proxy
GET  /api/proxy/stats?id=        - Get proxy stats
```

### System Management APIs
```
GET  /api/screenshot?client_id=  - Capture screenshot (existing)
POST /api/command                - Execute command (existing)
POST /api/update/global          - Global client update (existing)
```

## Request/Response Examples

### Create Proxy
```javascript
// Request
POST /api/proxy/create
Content-Type: application/json

{
  "client_id": "machine-001",
  "remote_host": "192.168.1.100",
  "remote_port": 3306,
  "local_port": 3307,
  "protocol": "tcp"
}

// Response (200 OK)
{
  "id": "machine-001-3307-1704850000",
  "client_id": "machine-001",
  "local_port": 3307,
  "remote_host": "192.168.1.100",
  "remote_port": 3306,
  "protocol": "tcp",
  "status": "active",
  "bytes_in": 0,
  "bytes_out": 0,
  "created_at": "2024-01-10T14:30:00Z",
  "last_active": "2024-01-10T14:30:00Z"
}
```

### List Files
```javascript
// Request
GET /api/files?client_id=machine-001&path=/home/user

// Response (200 OK)
[
  {
    "name": "Documents",
    "path": "/home/user/Documents",
    "size": 0,
    "modified": 1704806400,
    "is_dir": true
  },
  {
    "name": "config.ini",
    "path": "/home/user/config.ini",
    "size": 2560,
    "modified": 1704793200,
    "is_dir": false
  }
]
```

### List Processes
```javascript
// Request
GET /api/processes?client_id=machine-001

// Response (200 OK)
[
  {
    "name": "svchost.exe",
    "pid": 4,
    "cpu": 2.5,
    "memory": 15.3,
    "status": "running"
  },
  {
    "name": "explorer.exe",
    "pid": 1024,
    "cpu": 5.1,
    "memory": 45.6,
    "status": "running"
  }
]
```

## Frontend JavaScript Functions

### Dashboard (dashboard-new.html)
```javascript
loadClients()              - Fetch and render client list
selectClient(event, client) - Select client for viewing
showClientDetails(client)  - Display details panel
closeClientDetails()       - Hide details panel
updateStats()              - Update statistics cards
openClientPanel()          - Open control window
sendCommand()              - Open terminal
confirmRemove()            - Confirm client removal
confirmUninstall()         - Confirm client uninstall
```

### Client Details (client-details.html)
```javascript
loadClientDetails()        - Load client info
updateClientDisplay()      - Update display fields
switchTab(tabName)         - Switch between tabs
browseFolder()             - Load folder contents
loadFiles(path)            - Fetch file listing
renderFileList(files)      - Display files in table
executeTerminalCommand()   - Run terminal command
loadProcesses()            - Fetch process list
renderProcessList()        - Display processes
executeAction(action)      - Execute system action
```

## Backend Go Functions

### ProxyManager (proxy_handler.go)
```go
NewProxyManager(manager *ClientManager)           - Create manager
CreateProxyConnection(clientID, host, ports...)   - New proxy
acceptConnections(conn *ProxyConnection)         - Accept connections
relayConnection(proxy, conn)                      - Relay data
CloseProxyConnection(id string)                   - Close proxy
GetProxyConnection(id string)                     - Get proxy
ListProxyConnections(clientID string)             - List client proxies
ListAllProxyConnections()                         - List all proxies
```

### Server Handlers (handlers.go / proxy_handler.go)
```go
HandleProxyCreate(w, r)                          - POST /api/proxy/create
HandleProxyList(w, r)                            - GET /api/proxy/list
HandleProxyClose(w, r)                           - POST /api/proxy/close
HandleProxyStats(w, r)                           - GET /api/proxy/stats
HandleClientGet(w, r)                            - GET /api/client
HandleFilesAPI(w, r)                             - GET /api/files
HandleProcessesAPI(w, r)                         - GET /api/processes
ProxyFileServer(w, r)                            - GET /api/proxy-file
```

### WebHandler (web_handlers.go)
```go
HandleDashboardNew(w, r)                         - GET /dashboard-new
HandleClientDetails(w, r)                        - GET /client-details
RegisterWebRoutes(mux *http.ServeMux)             - Register all routes
```

## Security Features

### Authentication
- Session-based authentication
- Session ID validation on protected routes
- Automatic session expiration after 24 hours
- HttpOnly cookies prevent XSS attacks

### Input Validation
- Path traversal prevention in file browser
- Client ID validation on all endpoints
- Port number range validation
- Protocol whitelist validation

### Output Encoding
- HTML entity encoding for dynamic content
- JSON content-type headers
- XSS prevention through template auto-escaping

### Error Handling
- Consistent HTTP status codes
- Safe error messages (no system paths leaked)
- Graceful degradation on failures
- Detailed logging for debugging

## Performance Considerations

### Caching
- Client list cached in memory
- Session information cached with TTL
- File system listings not cached (dynamic)
- Process lists refreshed on demand

### Connection Pooling
- Proxy connections reuse TCP connections
- WebSocket connections persist
- Graceful connection cleanup
- Automatic reconnection support

### Scalability
- Goroutines for concurrent client handling
- Mutex locks for thread-safe operations
- Non-blocking I/O operations
- Connection limits per client

## Deployment Checklist

- [ ] Compile Go binaries: `go build ./server`
- [ ] Verify Go version: `go version` (1.20+)
- [ ] Copy HTML templates to `web/templates/`
- [ ] Set web credentials: `--web-user admin --web-pass secure`
- [ ] Enable TLS if needed: `--tls --cert cert.pem --key key.pem`
- [ ] Test connectivity: `curl http://localhost:8080/login`
- [ ] Verify client connections to `/ws`
- [ ] Test proxy creation endpoint
- [ ] Verify logs for errors
- [ ] Monitor system resources

## Troubleshooting Guide

### Compile Errors
```
# GetClient mismatch error
Error: assignment mismatch: 1 variable but pm.manager.GetClient returns 2 values
Solution: Use: client, exists := pm.manager.GetClient(id)
         Check: if !exists { ... }
```

### Runtime Issues
```
# Client not found
Cause: Client ID doesn't match or client disconnected
Solution: Verify client ID in request
         Check client connection status in logs

# File path error
Cause: Invalid path or permission denied
Solution: Use absolute paths
         Verify path exists on client
         Check file permissions
```

### Performance Issues
```
# Slow file listing
Cause: Large directory or slow I/O
Solution: Add pagination (page parameter)
         Implement filtering
         Use separate goroutine for I/O

# High memory usage
Cause: Many concurrent proxies
Solution: Limit concurrent connections
         Implement proxy cleanup
         Monitor connection lifecycle
```

## Integration Points with Existing Code

### ClientManager Integration
```go
// Access existing client
client, exists := s.manager.GetClient(clientID)

// Get all clients
clients := s.manager.GetAllClients()

// Subscribe to client events
s.manager.RegisterHandler(handler)
```

### WebHandler Integration
```go
// Register new routes
mux.HandleFunc("/api/route", wh.RequireAuth(handler))

// Access session info
session, _ := wh.sessionMgr.GetSession(sessionID)
```

### TerminalProxy Integration
```go
// WebSocket for interactive terminal
mux.HandleFunc("/api/terminal", s.terminalProxy.HandleTerminalWebSocket)

// Send command through terminal
s.terminalProxy.SendCommand(clientID, command)
```

## Future Enhancement Opportunities

1. **Real-time Dashboards**: WebSocket-based live updates
2. **Advanced Filtering**: Search and filter proxies
3. **Bandwidth Throttling**: Limit proxy bandwidth
4. **Load Balancing**: Distribute clients across servers
5. **HA Setup**: High availability with multiple servers
6. **API Authentication**: OAuth2 or API keys
7. **Webhook Support**: Event-driven integrations
8. **GraphQL API**: Alternative to REST

## Version Information

- **Implementation Date**: January 2024
- **Go Version**: 1.20+
- **Browser Support**: Chrome 90+, Firefox 88+, Safari 14+
- **Frontend Framework**: Vanilla JavaScript (no dependencies)
- **Backend Dependencies**: Standard Go library + gorilla/websocket

## Support & Documentation

- Main documentation: `LANPROXY_INTEGRATION.md`
- User guide: `LANPROXY_QUICKSTART.md`
- This reference: `LANPROXY_TECHNICAL.md`
- Original README: `README.md`

---

Last Updated: January 2024
Version: 1.0
