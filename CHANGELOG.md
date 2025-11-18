# Changelog - Production Deployment Updates

## Version 2.0 - Production-Ready Architecture

**Date:** 2024-11-18

### Overview

Refactored the Server Manager system for production deployment with nginx reverse proxy, enhanced security, and simplified client configuration.

---

## Major Changes

### 1. Nginx Reverse Proxy Support

**Server Changes:**
- Added `-tls` flag to enable/disable TLS mode (default: `false`)
- Changed default port from `:8443` to `:8080` for nginx backend
- Server now runs in HTTP mode by default (nginx handles TLS termination)
- TLS mode still available for development/testing with `-tls` flag

**Files Modified:**
- `server/main.go` - Added TLS flag and port configuration
- `server/handlers.go` - Conditional TLS support in `Start()` method

**Benefits:**
- Simplified certificate management (nginx handles it)
- Better performance with nginx load balancing
- Standard production architecture
- Easier integration with Let's Encrypt

### 2. Automatic Machine ID Generation

**Client Changes:**
- Removed dependency on configuration files
- Client automatically generates unique machine ID from system hardware
- Machine ID cached in platform-specific directories
- No manual configuration required

**Implementation:**
- New file: `client/machine_id.go`
  - `MachineIDGenerator` struct with platform-specific ID generation
  - SHA256 hash of: hostname + host UUID + OS-specific identifiers
  - Caching in:
    - Windows: `%APPDATA%\ServerManager\machine-id`
    - Linux: `~/.config/servermanager/machine-id`
    - macOS: `~/Library/Application Support/servermanager/machine-id`

**Files Modified:**
- `client/main.go` - Integrated machine ID generation, removed config file support

**Benefits:**
- Zero-configuration deployment
- Hardware-based unique identifiers
- Persistent across restarts
- Automatic conflict resolution

### 3. Unique Client ID Enforcement

**Server Changes:**
- Server now enforces unique machine IDs
- When duplicate ID detected, automatically closes old connection
- New connection takes precedence

**Files Modified:**
- `server/client_manager.go` 
  - Added `IsClientIDRegistered()` method
  - Modified `Run()` to close duplicate connections

**Benefits:**
- Prevents ID conflicts
- Automatic failover (new connection replaces old)
- Better client tracking

### 4. Mandatory TLS Verification

**Client Changes:**
- Removed `TLSSkipVerify` option from client configuration
- Client always enforces TLS certificate verification
- `InsecureSkipVerify` hardcoded to `false`
- `MinVersion` set to TLS 1.2

**Files Modified:**
- `client/main.go` - Removed `TLSSkipVerify` field, hardcoded secure settings

**Benefits:**
- Enforced security best practices
- No option to disable certificate verification
- Protection against MITM attacks

---

## New Files

### Configuration
- `configs/nginx.conf` - Production-ready nginx configuration with:
  - TLS termination
  - WebSocket proxy support
  - Rate limiting
  - Security headers
  - OCSP stapling
  - HTTP to HTTPS redirect

### Documentation
- `DEPLOYMENT.md` - Comprehensive production deployment guide including:
  - Architecture overview
  - Step-by-step setup (systemd, nginx, Let's Encrypt)
  - Security hardening
  - Monitoring and maintenance
  - Troubleshooting

- `QUICKSTART.md` - Quick start guide for both development and production:
  - 5-minute development setup
  - Production deployment steps
  - Configuration reference
  - Common issues

- `CHANGELOG.md` - This file

### Code
- `client/machine_id.go` - Machine ID generation and caching

---

## Modified Files

### Server

**server/main.go:**
- Added `-tls` flag (default: `false`)
- Changed default port from `:8443` to `:8080`
- Made `-cert` and `-key` optional (only required if `-tls` enabled)
- Added logging to indicate HTTP vs TLS mode

**server/handlers.go:**
- Added `UseTLS` field to `Config` struct
- Modified `Start()` to conditionally enable TLS
- Updated logging messages

**server/client_manager.go:**
- Added `IsClientIDRegistered()` method
- Modified `Run()` to handle duplicate client IDs
- Closes old connection when duplicate detected

### Client

**client/main.go:**
- Removed `Config` struct (no longer needed)
- Integrated `MachineIDGenerator` for automatic ID generation
- Removed config file parsing
- Removed `TLSSkipVerify` field
- Hardcoded secure TLS settings (`InsecureSkipVerify: false`, `MinVersion: TLS12`)
- Updated command-line flags (removed `-id`, kept `-server` and `-token`)

### Build System

**Makefile:**
- Updated `client` target to detect macOS 15+ and use `-tags noscreenshot`
- Changed all `client_monitor/*.go` to `./client_monitor` for proper build tag handling
- Added build note for macOS builds

**build.sh:**
- Already had macOS 15 detection (no changes needed)

### Documentation

**README.md:**
- Updated Features section to reflect new capabilities
- Rewrote Installation section for nginx deployment
- Rewrote Configuration section (removed config file examples)
- Added machine ID generation details
- Updated Security Considerations
- Expanded Troubleshooting section

---

## Command-Line Interface Changes

### Server

**Before:**
```bash
./server -addr :8443 -cert certs/server.crt -key certs/server.key -token secret
```

**After (Production - HTTP mode):**
```bash
./server -addr :8080 -token secret
```

**After (Development - TLS mode):**
```bash
./server -addr :8443 -tls -cert certs/server.crt -key certs/server.key -token secret
```

**New Flags:**
- `-tls` - Enable TLS mode (default: `false`)

**Changed Flags:**
- `-addr` default changed from `:8443` to `:8080`
- `-cert` and `-key` now optional (only required if `-tls` enabled)

### Client

**Before:**
```bash
./client -config client-config.json
# OR
./client -server wss://server:8443/ws -id client-001 -token secret -skip-tls
```

**After:**
```bash
./client -server wss://your-domain.com/ws -token secret
```

**Removed Flags:**
- `-config` - No longer supports config files
- `-id` - Machine ID auto-generated
- `-skip-tls` - Certificate verification always enforced

**Remaining Flags:**
- `-server` - WebSocket URL (required)
- `-token` - Authentication token (required)
- `-autostart` - Enable auto-start on boot (optional)

---

## Migration Guide

### For Existing Deployments

1. **Update Server Deployment:**
   ```bash
   # Install nginx if not already installed
   sudo apt install nginx
   
   # Configure nginx (see configs/nginx.conf)
   sudo cp configs/nginx.conf /etc/nginx/sites-available/servermanager
   sudo ln -s /etc/nginx/sites-available/servermanager /etc/nginx/sites-enabled/
   
   # Update systemd service to remove TLS flags
   # Change: -addr :8443 -cert ... -key ...
   # To:     -addr :8080
   
   sudo systemctl daemon-reload
   sudo systemctl restart servermanager
   ```

2. **Update Clients:**
   ```bash
   # Old clients with config files need to be updated
   
   # Before:
   ./client -config client-config.json
   
   # After (using same server URL and token from config):
   ./client -server wss://your-domain.com/ws -token <token-from-config>
   
   # Machine ID will be auto-generated on first run
   ```

3. **Handle Duplicate Machine IDs:**
   - If you previously assigned IDs manually, some machines may generate the same ID
   - Server will automatically close old connection when duplicate detected
   - To force new ID, delete cached ID file and restart client:
     - Windows: `del %APPDATA%\ServerManager\machine-id`
     - Linux: `rm ~/.config/servermanager/machine-id`
     - macOS: `rm ~/Library/Application\ Support/servermanager/machine-id`

---

## Breaking Changes

⚠️ **Important:** These changes are NOT backward compatible.

1. **Server Default Port Changed:**
   - Old: `:8443` (TLS)
   - New: `:8080` (HTTP for nginx)
   - **Action:** Update firewall rules, nginx config, or use `-addr :8443 -tls` for old behavior

2. **Client Config Files Removed:**
   - `client-config.json` no longer supported
   - **Action:** Extract server URL and token from config, pass as command-line flags

3. **Client ID Auto-Generated:**
   - `-id` flag removed
   - **Action:** Remove `-id` from startup scripts, ID generated automatically

4. **TLS Verification Always Enforced:**
   - `-skip-tls` / `tls_skip_verify` option removed
   - **Action:** Ensure server has valid TLS certificate (use Let's Encrypt with nginx)

---

## Security Improvements

1. **Mandatory TLS Verification:**
   - No option to skip certificate validation
   - Protects against MITM attacks

2. **Unique Client IDs:**
   - Hardware-based identification
   - Prevents ID spoofing
   - Automatic duplicate detection

3. **Token-Based Authentication:**
   - Already implemented, no changes
   - Use strong tokens: `openssl rand -hex 32`

4. **Nginx Security Features:**
   - Rate limiting on WebSocket and API endpoints
   - Security headers (HSTS, X-Frame-Options, etc.)
   - OCSP stapling
   - Modern TLS ciphers (TLS 1.2+)

---

## Performance Improvements

1. **Nginx Proxy:**
   - Better connection handling
   - Load balancing support
   - Static file serving
   - Gzip compression

2. **HTTP Backend:**
   - No TLS overhead on localhost connection
   - Nginx handles TLS encryption efficiently
   - Simpler server code

---

## Testing

All changes tested on:
- ✅ macOS 15 (Sequoia) - with screenshot stub
- ✅ Build system (Makefile and build.sh)
- ✅ Server startup (both HTTP and TLS modes)
- ✅ Client connection and machine ID generation
- ✅ Duplicate client ID handling

Pending testing:
- ⏳ Linux client deployment
- ⏳ Windows client deployment
- ⏳ Full nginx production setup
- ⏳ Let's Encrypt certificate integration

---

## Known Issues

See [KNOWN_ISSUES.md](KNOWN_ISSUES.md) for details:

1. **macOS 15+ Screenshot Support:**
   - Screenshot library incompatible with macOS 15+
   - Builds with `-tags noscreenshot` disable feature
   - Client returns error when screenshot requested

2. **Machine ID Regeneration:**
   - Changing hostname or hardware will generate new ID
   - May result in duplicate client entries on server
   - Workaround: Delete old entries or restart server

---

## Rollback Procedure

If you need to rollback to the previous version:

1. **Server:**
   ```bash
   # Revert to TLS mode
   ./server -addr :8443 -tls -cert certs/server.crt -key certs/server.key -token <token>
   ```

2. **Client:**
   ```bash
   # Use previous client binary with config file
   ./client-old -config client-config.json
   ```

3. **Remove nginx (if newly installed):**
   ```bash
   sudo systemctl stop nginx
   sudo systemctl disable nginx
   # Remove configuration
   sudo rm /etc/nginx/sites-enabled/servermanager
   ```

---

## Future Enhancements

Planned for next release:

- [ ] Web-based management UI
- [ ] Client grouping and bulk operations
- [ ] Database backend for client history
- [ ] Multi-tenancy support
- [ ] Role-based access control (RBAC)
- [ ] Client update mechanism with version checking
- [ ] Metrics and monitoring (Prometheus integration)
- [ ] Docker deployment option

---

## Contributors

- System refactoring and nginx integration
- Machine ID generation implementation
- Documentation updates
- Build system improvements

---

## Upgrade Checklist

Use this checklist when upgrading from v1.x to v2.0:

- [ ] Read this CHANGELOG completely
- [ ] Review [DEPLOYMENT.md](DEPLOYMENT.md) for production setup
- [ ] Install and configure nginx
- [ ] Obtain TLS certificates (Let's Encrypt or similar)
- [ ] Update server startup command (remove TLS flags)
- [ ] Test server connectivity: `curl http://localhost:8080/health`
- [ ] Update client deployment scripts (remove config file, use flags)
- [ ] Test client connection from one machine
- [ ] Verify machine ID generation and caching
- [ ] Deploy to remaining clients
- [ ] Monitor for duplicate ID issues
- [ ] Update documentation and runbooks
- [ ] Train team on new architecture

---

## Support

For issues or questions:

1. Check [QUICKSTART.md](QUICKSTART.md) for common setup issues
2. Review [DEPLOYMENT.md](DEPLOYMENT.md) for production deployment
3. See [KNOWN_ISSUES.md](KNOWN_ISSUES.md) for platform-specific problems
4. Check server and nginx logs for errors

---

**End of Changelog**
