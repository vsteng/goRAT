# Proxy Implementation Guide

## Overview

This document describes the improved proxy implementation based on the lanproxy architecture. The proxy system enables creating TCP tunnels between the server and remote hosts through connected clients, similar to a VPN or SSH tunnel.

## Key Improvements

### 1. Two-Way WebSocket Communication
- **Before**: Proxy connections were not properly relayed through websocket
- **After**: Full bidirectional communication through websocket with proper message protocol

### 2. User Connection Tracking
Similar to lanproxy's approach:
- Each proxy connection maintains a map of active user connections
- User connections are properly tracked with unique IDs
- Proper cleanup when connections close

### 3. Port Mapping
- Added port-to-proxy-connection mapping for efficient lookups
- Similar to lanproxy's `portCmdChannelMapping`

## Architecture

### Components

#### ProxyConnection
```go
type ProxyConnection struct {
    ID            string                    // Unique proxy ID
    ClientID      string                    // Connected client ID
    LocalPort     int                       // Local listening port on server
    RemoteHost    string                    // Remote host to connect to (on client side)
    RemotePort    int                       // Remote port to connect to
    Protocol      string                    // "tcp", "http", or "https"
    Status        string                    // "active", "inactive", "error"
    BytesIn       int64                     // Bytes received from users
    BytesOut      int64                     // Bytes sent to users
    CreatedAt     time.Time                 // Creation time
    LastActive    time.Time                 // Last activity time
    listener      net.Listener              // TCP listener on local port
    userChannels  map[string]*net.Conn      // Active user connections
}
```

#### ProxyManager
- Manages all proxy connections
- Handles creation, deletion, and relay of connections
- Maintains port mappings for efficient lookup

### Message Flow

1. **Create Proxy Request**
   ```
   Client (HTTP) -> Server
   POST /api/proxy/create
   {
     "client_id": "machine-123",
     "remote_host": "192.168.1.100",
     "remote_port": 3389,
     "local_port": 13389,
     "protocol": "tcp"
   }
   ```

2. **User Connects to Proxy**
   ```
   User -> Server:LocalPort
   ```

3. **Proxy Connect Message**
   ```
   Server -> Client (WebSocket)
   {
     "type": "proxy_connect",
     "proxy_id": "machine-123-13389-1733",
     "user_id": "user-13389-1234567890",
     "remote_host": "192.168.1.100",
     "remote_port": 3389,
     "protocol": "tcp"
   }
   ```

4. **Client Connects to Remote**
   ```
   Client connects to 192.168.1.100:3389
   ```

5. **Proxy Data Messages** (bidirectional)
   ```
   Server -> Client or Client -> Server
   {
     "type": "proxy_data",
     "proxy_id": "machine-123-13389-1733",
     "user_id": "user-13389-1234567890",
     "data": <binary data as bytes>
   }
   ```

6. **Proxy Disconnect Message**
   ```
   Server -> Client or Client -> Server
   {
     "type": "proxy_disconnect",
     "proxy_id": "machine-123-13389-1733",
     "user_id": "user-13389-1234567890"
   }
   ```

## Implementation Details

### ProxyManager.CreateProxyConnection()
Creates a new proxy tunnel:
1. Verifies client is connected and websocket is active
2. Generates unique proxy ID
3. Starts TCP listener on specified local port
4. Registers port mapping
5. Starts accepting user connections

### ProxyManager.acceptConnections()
Accepts incoming user connections on the proxy listener:
1. Accepts TCP connection from user
2. Generates unique user ID
3. Tracks user connection in proxy
4. Calls `handleUserConnection()` for relay

### ProxyManager.handleUserConnection()
Handles bidirectional relay:
1. Sends `proxy_connect` message to client via websocket
2. Reads data from user connection
3. Sends `proxy_data` messages to client
4. Handles connection close and cleanup

### ProxyManager.HandleProxyDataFromClient()
Handles data coming back from client:
1. Locates proxy and user connection
2. Writes data to user connection
3. Updates statistics

### Server.readPump() - Modified
Updated websocket message reading:
1. Reads all messages as raw JSON first
2. Checks for proxy message types ("proxy_data", etc.)
3. Routes to proxy manager if needed
4. Otherwise parses as common.Message

## API Endpoints

### Create Proxy
```
POST /api/proxy/create
Content-Type: application/json

{
  "client_id": "machine-id",
  "remote_host": "192.168.1.100",
  "remote_port": 3389,
  "local_port": 13389,
  "protocol": "tcp"
}

Response:
{
  "ID": "machine-id-13389-1733...",
  "ClientID": "machine-id",
  "LocalPort": 13389,
  "RemoteHost": "192.168.1.100",
  "RemotePort": 3389,
  "Protocol": "tcp",
  "Status": "active",
  "BytesIn": 0,
  "BytesOut": 0,
  "CreatedAt": "2025-12-06T...",
  "LastActive": "2025-12-06T..."
}
```

### List Proxies
```
GET /api/proxy/list?client_id=machine-id

Response:
[
  {
    "ID": "...",
    "ClientID": "...",
    ...
  }
]
```

### Close Proxy
```
POST /api/proxy/close?id=proxy-id-here

Response:
{
  "status": "closed"
}
```

### Get Proxy Stats
```
GET /api/proxy/stats?id=proxy-id-here

Response:
{
  "ID": "...",
  "BytesIn": 12345,
  "BytesOut": 54321,
  "Status": "active",
  ...
}
```

## Usage Example

### 1. Create a proxy to RDP server through client
```bash
curl -X POST http://localhost:8080/api/proxy/create \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "my-pc",
    "remote_host": "192.168.1.100",
    "remote_port": 3389,
    "local_port": 13389,
    "protocol": "tcp"
  }'
```

### 2. Connect to the proxy locally
```bash
# On your machine (Linux/Mac)
rdesktop localhost:13389

# Or on Windows
mstsc /v:localhost:13389
```

### 3. Monitor proxy statistics
```bash
curl http://localhost:8080/api/proxy/stats?id=my-pc-13389-1733...
```

### 4. Close proxy
```bash
curl -X POST http://localhost:8080/api/proxy/close?id=my-pc-13389-1733...
```

## Connection Lifecycle

```
1. Create proxy -> Listening on local port
   ↓
2. User connects to localhost:port -> Accept connection
   ↓
3. Send proxy_connect to client -> Client initiates connection to remote
   ↓
4. Client responds -> Connection established
   ↓
5. Bidirectional data relay via proxy_data messages
   ↓
6. User closes connection -> Send proxy_disconnect
   ↓
7. Connection cleaned up
```

## Error Handling

- **Client not found**: Returns error when client is offline
- **Websocket not connected**: Validates websocket is active before proxy creation
- **Port in use**: Returns error if local port already bound
- **Connection timeout**: User connections have configurable timeout
- **Invalid proxy ID**: Returns error when accessing non-existent proxy

## Differences from Original Implementation

| Aspect | Before | After |
|--------|--------|-------|
| Connection Relay | Not implemented (TODO placeholders) | Full websocket relay |
| User Tracking | Single connection per proxy | Multiple user connections tracked |
| Message Protocol | No defined protocol | Structured JSON messages |
| Port Management | No tracking | Port-to-proxy mapping |
| Cleanup | Basic | Proper cleanup of all connections |
| Statistics | Minimal | Bytes in/out tracking |

## Comparison with LanProxy

Both implementations follow similar principles:

### LanProxy Concepts Adopted:
1. **Channel Manager**: ProxyManager manages connections like ProxyChannelManager
2. **Port Mapping**: Maps external ports to client connections
3. **User Channels**: Tracks multiple user connections per proxy client
4. **Message Relay**: Bidirectional relay through control channel
5. **Lifecycle Management**: Proper connection state management

### Differences:
1. **Transport**: Lanproxy uses Netty/Java, we use Go WebSocket
2. **Protocol**: Lanproxy uses custom binary protocol, we use JSON over WebSocket
3. **Implementation**: Adapted for Go's net package instead of Netty channels

## Future Improvements

1. Add SSL/TLS encryption for proxy_data messages
2. Implement bandwidth throttling
3. Add proxy authentication
4. Implement connection pooling
5. Add proxy failover
6. Support UDP proxying
7. Add metrics and monitoring
8. Implement connection limits per proxy

## Troubleshooting

### Proxy Creation Fails
- Check client is connected: `GET /api/clients`
- Check port is available: `netstat -tulpn | grep :port`
- Check server logs for errors

### No Data Flow
- Verify websocket connection is active
- Check client-side proxy connection handler is implemented
- Enable debug logging

### Connection Timeout
- Increase timeout values if needed
- Check network connectivity between client and remote

## Configuration

Currently, all parameters are passed per-proxy:
- `local_port`: The port server listens on
- `remote_host`: Target host client connects to
- `remote_port`: Target port on remote host
- `protocol`: tcp/http/https (mainly for documentation)

To add global configuration, modify `Config` struct and pass to `ProxyManager`.
