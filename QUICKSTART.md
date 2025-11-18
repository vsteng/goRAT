# Quick Start Guide

Get the Server Manager system up and running in 5 minutes.

## Prerequisites

- Go 1.21+ installed
- Domain name (for production) or `localhost` (for testing)

## Development/Testing Setup (No Nginx)

### 1. Build

```bash
# Clone and build
git clone <repository-url>
cd servermanager
make build
```

### 2. Generate TLS Certificates

```bash
chmod +x scripts/generate-certs.sh
./scripts/generate-certs.sh
```

This creates self-signed certificates in `certs/`.

### 3. Start Server

```bash
# Start with TLS on port 8443
./bin/server -addr :8443 -tls -cert certs/server.crt -key certs/server.key -token mysecrettoken
```

Output:
```
2024/01/15 10:00:00 Server starting with TLS on :8443
2024/01/15 10:00:00 Server started successfully
```

### 4. Start Client

```bash
# In a new terminal
./bin/client -server wss://localhost:8443/ws -token mysecrettoken
```

Output:
```
2024/01/15 10:00:05 Generated machine ID: a1b2c3d4e5f6...
2024/01/15 10:00:05 Machine ID cached at: ~/.config/servermanager/machine-id
2024/01/15 10:00:05 Connected to server
2024/01/15 10:00:05 Authentication successful
```

**Note:** You'll see a certificate warning because it's self-signed. This is expected in development.

### 5. Test Connection

```bash
# In another terminal, list connected clients
curl http://localhost:8443/api/clients
```

Output:
```json
[
  {
    "id": "a1b2c3d4e5f6...",
    "os": "linux",
    "arch": "amd64",
    "hostname": "my-laptop",
    "ip": "127.0.0.1",
    "status": "online",
    "connected_at": "2024-01-15T10:00:05Z",
    "last_seen": "2024-01-15T10:00:10Z"
  }
]
```

## Production Setup (With Nginx)

See [DEPLOYMENT.md](DEPLOYMENT.md) for complete production deployment guide.

### Quick Production Steps

1. **Build server:**
   ```bash
   make build
   ```

2. **Install nginx and certbot:**
   ```bash
   sudo apt install nginx certbot python3-certbot-nginx
   ```

3. **Get TLS certificate:**
   ```bash
   sudo certbot --nginx -d your-domain.com
   ```

4. **Configure nginx:**
   ```bash
   sudo cp configs/nginx.conf /etc/nginx/sites-available/servermanager
   # Edit file: update domain and cert paths
   sudo ln -s /etc/nginx/sites-available/servermanager /etc/nginx/sites-enabled/
   sudo nginx -t
   sudo systemctl reload nginx
   ```

5. **Start server (HTTP mode):**
   ```bash
   ./bin/server -addr :8080 -token $(openssl rand -hex 32)
   ```

6. **Connect client:**
   ```bash
   ./bin/client -server wss://your-domain.com/ws -token <your-token>
   ```

## Client Auto-Start

Enable auto-start so the client launches on boot:

### Linux
```bash
./bin/client -server wss://your-domain.com/ws -token <token> -autostart
```

This creates a systemd user service.

**Verify:**
```bash
systemctl --user status ServerManagerClient.service
```

### Windows
```cmd
client.exe -server wss://your-domain.com/ws -token <token> -autostart
```

This adds a registry entry to start on login.

**Verify:**
```cmd
reg query "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v ServerManagerClient
```

## Client Monitor

The monitor ensures the client is always running:

```bash
# Start monitor (will start client if not running)
./bin/client_monitor -client /path/to/client -- -server wss://your-domain.com/ws -token <token>
```

Monitor features:
- Checks if client is running every 10 seconds
- Automatically restarts if client crashes
- Logs all activity

## Testing Features

### 1. Execute Remote Command

```bash
curl -X POST http://localhost:8443/api/command \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "<machine-id>",
    "command": {
      "command": "hostname",
      "timeout": 10
    }
  }'
```

### 2. Browse Files

The client will receive file browser commands via WebSocket. You can implement a simple web UI or send messages directly via WebSocket client.

### 3. Take Screenshot

Send a screenshot command via WebSocket to receive a PNG image.

### 4. Monitor Logs

**Server logs:**
```bash
# Server output (if running in terminal)
# Shows connections, commands, errors

# Client output
# Shows connection status, received commands, health checks
```

## Architecture Overview

```
Production:
┌────────┐  HTTPS/WSS   ┌───────┐  HTTP/WS   ┌────────┐
│ Client ├─────────────>│ Nginx ├───────────>│ Server │
└────────┘  (Port 443)  └───────┘ (Port 8080)└────────┘
            • TLS                              • Token Auth
            • Verify Cert                      • Unique IDs

Development:
┌────────┐  WSS (TLS)  ┌────────┐
│ Client ├────────────>│ Server │
└────────┘ (Port 8443) └────────┘
           • Self-signed cert
           • Direct connection
```

## Configuration Summary

### Server Flags
| Flag | Description | Default | Required |
|------|-------------|---------|----------|
| `-addr` | Listen address | `:8080` | No |
| `-tls` | Enable TLS | `false` | No |
| `-cert` | TLS certificate | - | If `-tls` |
| `-key` | TLS private key | - | If `-tls` |
| `-token` | Auth token | - | Yes |

### Client Flags
| Flag | Description | Required |
|------|-------------|----------|
| `-server` | WebSocket URL (wss://...) | Yes |
| `-token` | Auth token | Yes |
| `-autostart` | Enable auto-start | No |

### Monitor Flags
| Flag | Description | Default |
|------|-------------|---------|
| `-client` | Client binary path | Required |
| `-interval` | Check interval | `10s` |
| `-restart-delay` | Delay before restart | `5s` |
| `-max-restarts` | Max restarts (-1 = unlimited) | `-1` |
| `-install` | Install client from path | - |

## Machine ID

The client automatically generates a unique machine ID based on:
- Hostname
- Host UUID (SMBIOS/DMI)
- OS-specific identifiers:
  - Windows: Machine GUID
  - Linux: `/etc/machine-id`
  - macOS: IOPlatformUUID

**ID is cached at:**
- Windows: `%APPDATA%\ServerManager\machine-id`
- Linux: `~/.config/servermanager/machine-id`
- macOS: `~/Library/Application Support/servermanager/machine-id`

**To regenerate ID:** Delete the cache file and restart client.

## Security Notes

- **Always use `wss://` (WebSocket Secure)** - never `ws://`
- **Client enforces TLS verification** - cannot skip certificate check
- **Use strong tokens** - generate with `openssl rand -hex 32`
- **Server rejects duplicate IDs** - each machine must have unique ID
- **Keep tokens secret** - never commit to version control

## Common Issues

### Client shows "certificate verification failed"
- **Development:** This is expected with self-signed certs. Client will still connect.
- **Production:** Ensure nginx has valid certificate from trusted CA (Let's Encrypt).

### "Duplicate client ID" error
- Another client with same machine ID is connected
- Server automatically disconnects the old connection
- If persistent, delete cached machine ID and restart

### Client can't connect
- Check server is running: `curl http://localhost:8080/health` (nginx mode) or `curl http://localhost:8443/health` (TLS mode)
- Verify token matches between client and server
- Check firewall allows port 443 (nginx) or 8443 (direct TLS)

## Next Steps

- Read [README.md](README.md) for detailed feature documentation
- See [DEPLOYMENT.md](DEPLOYMENT.md) for production deployment guide
- Review [KNOWN_ISSUES.md](KNOWN_ISSUES.md) for platform-specific issues
- Check API documentation for available endpoints

## Getting Help

If you encounter issues:
1. Check server logs for errors
2. Verify client can reach server: `ping your-domain.com`
3. Test WebSocket: `wscat -c wss://your-domain.com/ws` (install with `npm install -g wscat`)
4. Review nginx logs: `/var/log/nginx/servermanager_error.log`
