# Authentication & Security Hardening

## Overview
This document describes the security hardening improvements made to the authentication system.

## Changes Implemented

### 1. **Rate Limiting (IP-based Brute Force Protection)**
- **File**: `pkg/auth/auth_hardening.go`
- **Implementation**: `RateLimiter` with token bucket algorithm
- **Features**:
  - Limits login attempts to 5 per 15-minute window per IP
  - Exponential backoff: blocked for 15 min, then 30 min, 60 min, etc. (up to 2^9)
  - Automatic cleanup of old entries after 24 hours
- **Logging**: Failed attempts logged with IP address and username
- **Prevention**: Effectively prevents brute force attacks

### 2. **Password Hashing Upgrade (SHA256 → Bcrypt)**
- **File**: `pkg/auth/auth_hardening.go`
- **Change**: Upgraded from SHA256 (weak) to bcrypt (strong)
- **Features**:
  - Bcrypt with default cost factor (cost = 12)
  - Automatic password verification in login
  - Migration path for existing SHA256 hashes (in `pkg/auth/migration.go`)
- **Security**: Bcrypt is slow by design, further reducing brute force effectiveness
- **Breaking Change**: Existing SHA256 password hashes in database need re-hashing
  - Old hashes detected and flagged for user password resets
  - New passwords automatically use bcrypt

### 3. **Session Context Security (IP/User-Agent Binding)**
- **Files**: `pkg/auth/interfaces.go`, `pkg/auth/session.go`
- **Implementation**: Sessions now track:
  - Client IP address
  - User Agent string
  - Verification status (first request vs. subsequent)
- **Features**:
  - First request: Allow if User-Agent matches (IP may change)
  - Subsequent requests: Enforce both IP and User-Agent match
  - Hijacking detection: Sessions terminated if IP/User-Agent mismatch
- **Methods Added**:
  - `UpdateSessionContext()`: Set IP/User-Agent on first access
  - `VerifySessionContext()`: Verify IP/User-Agent on subsequent requests
  - `IsValidForRequest()`: Check if session is valid for request

### 4. **CSRF Token Management**
- **File**: `pkg/auth/auth_hardening.go`
- **Implementation**: `CSRFTokenManager`
- **Features**:
  - Generates random 32-byte CSRF tokens
  - One-time use (consumed on validation)
  - 1-hour expiration
  - Per-session tokens (tied to session ID)
- **Status**: Created but not yet integrated into forms (ready for implementation)

### 5. **Security Headers**
- **File**: `server/web_handlers.go` (RegisterGinRoutes method)
- **Headers Implemented**:
  - `X-Frame-Options: SAMEORIGIN` - Prevent clickjacking
  - `X-Content-Type-Options: nosniff` - Prevent MIME sniffing
  - `X-XSS-Protection: 1; mode=block` - XSS protection
  - `Content-Security-Policy` - Restrict resource loading
  - `Strict-Transport-Security` - Enforce HTTPS (when available)
  - `Referrer-Policy: strict-origin-when-cross-origin` - Limit referrer leaking
  - `Permissions-Policy` - Disable dangerous features
  - `Cache-Control` - Prevent caching of sensitive pages

### 6. **Enhanced Audit Logging**
- **File**: `server/web_handlers.go` (HandleLoginAPI)
- **Log Events**:
  - ✅ `LOGIN SUCCESS`: Username, IP, User-Agent
  - ⚠️ `LOGIN FAILED`: Username, IP, reason
  - ⚠️ `RATE LIMITED`: IP, attempt count
  - ⚠️ `SESSION VERIFICATION FAILED`: IP/User-Agent mismatch
  - ❌ `SESSION CREATION FAILED`: System errors
- **Security**: Logs don't expose which field (username/password) was wrong (constant-time error)

### 7. **Client IP Extraction**
- **File**: `pkg/auth/auth_hardening.go`
- **Function**: `GetClientIP(remoteAddr, xForwardedFor string)`
- **Features**:
  - Handles X-Forwarded-For header for proxies
  - Falls back to RemoteAddr
  - Validates IP addresses

## Security Hardening Summary

| Issue | Before | After | Impact |
|-------|--------|-------|--------|
| Brute Force | No protection | Rate limiting (5/15min) + exponential backoff | High |
| Password Hashing | SHA256 (weak) | Bcrypt (strong, slow) | High |
| Session Hijacking | No IP/UA tracking | IP + User-Agent binding | High |
| XSS Attacks | Basic | CSP + XSS headers | Medium |
| Clickjacking | Unprotected | X-Frame-Options header | Medium |
| CSRF | No tokens | Token manager created (ready) | Medium |
| Caching | Possible | Disabled for sensitive pages | Medium |
| Audit Trail | Basic | Enhanced with IP, UA, event types | Medium |

## Integration Steps

### 1. Update Admin User on First Run
The system should re-hash the admin password using bcrypt when initializing:

```go
// In server initialization code
if !adminExists {
    hashedPassword, err := passwordHasher.Hash(adminPassword)
    // Create user with hashedPassword
}
```

### 2. Existing Users
Users with SHA256 password hashes can:
- **Option A**: Use "Forgot Password" to reset and get bcrypt hash
- **Option B**: Continue with old hash until next password change
- **Option C**: Admin can trigger password reset for users

### 3. Enable CSRF Protection (Future)
Integrate CSRF tokens in HTML forms:

```html
<form method="POST" action="/api/users">
    <input type="hidden" name="csrf_token" value="{{ .CSRFToken }}">
    <!-- form fields -->
</form>
```

## Testing

### Rate Limiting Test
```bash
# Make 6 login attempts from same IP within 15 minutes
for i in {1..6}; do
    curl -X POST http://localhost:8081/api/login \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"wrong"}' \
        -X-Forwarded-For "192.168.1.100"
    sleep 2
done
# 6th attempt should return 429 Too Many Requests
```

### Session Hijacking Test
```bash
# Login and get session cookie
COOKIE=$(curl -c - -X POST http://localhost:8081/api/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin"}' | grep session_id)

# Access protected endpoint with correct IP/User-Agent
curl -b "$COOKIE" http://localhost:8081/dashboard-new

# Try same cookie with different User-Agent (simulated)
curl -b "$COOKIE" -H "User-Agent: Different" http://localhost:8081/dashboard-new
# Should fail and redirect to login
```

### Bcrypt Verification Test
```bash
# Test password verification
go test ./pkg/auth -v -run TestPasswordHasher
```

## Configuration

### Rate Limiter Settings
In `server/web_handlers.go`:
```go
rateLimiter: auth.NewRateLimiter(
    5,              // maxAttempts
    15*time.Minute, // windowSize
)
```

### Session Timeout
Session timeout inherited from existing configuration (default: 1 hour)

### CSRF Token Expiration
1 hour (configurable in `CSRFTokenManager`)

## Dependencies

### New
- `golang.org/x/crypto` (bcrypt)

### Existing
- Standard library (crypto/rand, encoding/hex)
- Gin web framework

## Backwards Compatibility

- ✅ Existing sessions continue to work
- ✅ Old SHA256 passwords still accepted (with warning)
- ⚠️ Users with old hashes should reset passwords for security
- ✅ All endpoints remain the same

## Future Improvements

1. **MFA (Multi-Factor Authentication)**
   - TOTP/WebAuthn support
   - Per-user MFA settings

2. **Password Policy**
   - Minimum length enforcement
   - Complexity requirements
   - Password history tracking
   - Forced resets for old SHA256 hashes

3. **Account Lockout**
   - Lock account after N failed attempts
   - Admin unlock capability
   - Email notifications

4. **Session Management UI**
   - View active sessions
   - Revoke sessions remotely
   - Device management

5. **Audit Logging**
   - Persist auth logs to database
   - Search and filter capabilities
   - Export audit trail

6. **OAuth/SAML Integration**
   - Enterprise SSO support
   - Third-party providers

## Security Best Practices

1. **Always use HTTPS** in production (enforced via HSTS)
2. **Rotate session secrets** periodically
3. **Monitor audit logs** for suspicious patterns
4. **Update dependencies** regularly
5. **Use strong admin passwords** (at least 12 characters)
6. **Enable MFA** when implemented
7. **Review active sessions** periodically

## References

- [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
- [Bcrypt Documentation](https://pkg.go.dev/golang.org/x/crypto/bcrypt)
- [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [NIST Digital Identity Guidelines](https://pages.nist.gov/800-63-3/)
