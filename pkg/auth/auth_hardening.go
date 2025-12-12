package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// RateLimiter prevents brute force attacks using token bucket algorithm
type RateLimiter struct {
	mu          sync.Mutex
	attempts    map[string]*clientAttempts
	maxAttempts int
	windowSize  time.Duration
	cleanupTime time.Duration
}

type clientAttempts struct {
	attempts     int
	lastAttempt  time.Time
	blockedUntil time.Time
	resetTime    time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxAttempts int, windowSize time.Duration) *RateLimiter {
	rl := &RateLimiter{
		attempts:    make(map[string]*clientAttempts),
		maxAttempts: maxAttempts,
		windowSize:  windowSize,
		cleanupTime: 24 * time.Hour,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// AllowRequest checks if the request should be allowed
func (rl *RateLimiter) AllowRequest(identifier string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	attempt, exists := rl.attempts[identifier]

	if !exists {
		// First request from this identifier
		rl.attempts[identifier] = &clientAttempts{
			attempts:    1,
			lastAttempt: now,
			resetTime:   now.Add(rl.windowSize),
		}
		return true
	}

	// Check if blocked
	if attempt.blockedUntil.After(now) {
		return false
	}

	// Reset if window has passed
	if now.After(attempt.resetTime) {
		attempt.attempts = 1
		attempt.lastAttempt = now
		attempt.resetTime = now.Add(rl.windowSize)
		attempt.blockedUntil = time.Time{}
		return true
	}

	// Within window - check attempt count
	attempt.attempts++
	attempt.lastAttempt = now

	if attempt.attempts > rl.maxAttempts {
		// Block for exponential backoff: base 15 min * 2^violations
		violations := attempt.attempts - rl.maxAttempts
		blockDuration := 15 * time.Minute
		if violations > 0 && violations < 10 {
			blockDuration = time.Duration(15*(1<<uint(violations-1))) * time.Minute
		}
		attempt.blockedUntil = now.Add(blockDuration)
		return false
	}

	return true
}

// GetAttempts returns current attempt count for an identifier
func (rl *RateLimiter) GetAttempts(identifier string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if attempt, exists := rl.attempts[identifier]; exists {
		now := time.Now()
		if now.After(attempt.resetTime) {
			return 0
		}
		return attempt.attempts
	}
	return 0
}

// IsBlocked checks if an identifier is currently blocked
func (rl *RateLimiter) IsBlocked(identifier string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if attempt, exists := rl.attempts[identifier]; exists {
		return attempt.blockedUntil.After(time.Now())
	}
	return false
}

// Reset clears the rate limit for an identifier
func (rl *RateLimiter) Reset(identifier string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.attempts, identifier)
}

// cleanup periodically removes old entries
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for id, attempt := range rl.attempts {
			if now.Sub(attempt.lastAttempt) > rl.cleanupTime {
				delete(rl.attempts, id)
			}
		}
		rl.mu.Unlock()
	}
}

// PasswordHasher provides secure password hashing with bcrypt
type PasswordHasher struct {
	cost int
}

// NewPasswordHasher creates a new password hasher
func NewPasswordHasher() *PasswordHasher {
	return &PasswordHasher{
		cost: bcrypt.DefaultCost,
	}
}

// Hash generates a bcrypt hash of the password
func (ph *PasswordHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), ph.cost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// Verify compares a password with its hash
func (ph *PasswordHasher) Verify(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// CSRFTokenManager generates and validates CSRF tokens
type CSRFTokenManager struct {
	mu     sync.RWMutex
	tokens map[string]*csrfToken
}

type csrfToken struct {
	token     string
	sessionID string
	createdAt time.Time
	expiresAt time.Time
}

// NewCSRFTokenManager creates a new CSRF token manager
func NewCSRFTokenManager() *CSRFTokenManager {
	ctm := &CSRFTokenManager{
		tokens: make(map[string]*csrfToken),
	}

	// Start cleanup goroutine
	go ctm.cleanup()

	return ctm
}

// GenerateToken generates a new CSRF token for a session
func (ctm *CSRFTokenManager) GenerateToken(sessionID string) (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	token := hex.EncodeToString(b)
	now := time.Now()

	ctm.mu.Lock()
	ctm.tokens[token] = &csrfToken{
		token:     token,
		sessionID: sessionID,
		createdAt: now,
		expiresAt: now.Add(1 * time.Hour),
	}
	ctm.mu.Unlock()

	return token, nil
}

// ValidateToken validates and consumes a CSRF token
func (ctm *CSRFTokenManager) ValidateToken(token, sessionID string) bool {
	ctm.mu.Lock()
	defer ctm.mu.Unlock()

	ct, exists := ctm.tokens[token]
	if !exists {
		return false
	}

	// Check session matches
	if ct.sessionID != sessionID {
		return false
	}

	// Check expiration
	if time.Now().After(ct.expiresAt) {
		delete(ctm.tokens, token)
		return false
	}

	// Consume token (one-time use)
	delete(ctm.tokens, token)
	return true
}

// cleanup removes expired tokens
func (ctm *CSRFTokenManager) cleanup() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ctm.mu.Lock()
		now := time.Now()
		for token, ct := range ctm.tokens {
			if now.After(ct.expiresAt) {
				delete(ctm.tokens, token)
			}
		}
		ctm.mu.Unlock()
	}
}

// SessionHardener enhances session security with IP/User-Agent tracking
type SessionHardener struct {
	mu       sync.RWMutex
	sessions map[string]*EnhancedSession
}

// EnhancedSession extends Session with security metadata
type EnhancedSession struct {
	ID           string
	Username     string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	ClientIP     string
	UserAgent    string
	LastActivity time.Time
	Verified     bool // Has IP/User-Agent been verified at least once
}

// NewSessionHardener creates a new session hardener
func NewSessionHardener() *SessionHardener {
	return &SessionHardener{
		sessions: make(map[string]*EnhancedSession),
	}
}

// CreateSession creates an enhanced session
func (sh *SessionHardener) CreateSession(sessionID, username, clientIP, userAgent string, expiresAt time.Time) *EnhancedSession {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	session := &EnhancedSession{
		ID:           sessionID,
		Username:     username,
		CreatedAt:    time.Now(),
		ExpiresAt:    expiresAt,
		ClientIP:     clientIP,
		UserAgent:    userAgent,
		LastActivity: time.Now(),
		Verified:     false,
	}

	sh.sessions[sessionID] = session
	return session
}

// VerifySession checks if session matches IP/User-Agent
func (sh *SessionHardener) VerifySession(sessionID, clientIP, userAgent string) bool {
	sh.mu.RLock()
	defer sh.mu.RUnlock()

	session, exists := sh.sessions[sessionID]
	if !exists {
		return false
	}

	// Allow first request from different IP if user-agent matches
	if !session.Verified {
		return userAgent == session.UserAgent
	}

	// After verification, require both IP and User-Agent to match
	return session.ClientIP == clientIP && session.UserAgent == userAgent
}

// UpdateActivity updates last activity time and marks as verified
func (sh *SessionHardener) UpdateActivity(sessionID string) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	if session, exists := sh.sessions[sessionID]; exists {
		session.LastActivity = time.Now()
		session.Verified = true
	}
}

// DeleteSession removes a session
func (sh *SessionHardener) DeleteSession(sessionID string) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	delete(sh.sessions, sessionID)
}

// GetClientIP extracts the real client IP, supporting Cloudflare and common proxies.
// It prefers CF-Connecting-IP, then X-Forwarded-For (first IP), then X-Real-IP, then RemoteAddr.
func GetClientIP(remoteAddr, xForwardedFor string) string {
	// Try X-Forwarded-For header first (for proxies)
	if xForwardedFor != "" {
		// Take first IP if comma-separated
		ips := xForwardedFor
		for i, ch := range xForwardedFor {
			if ch == ',' {
				ips = xForwardedFor[:i]
				break
			}
		}
		if ip := net.ParseIP(strings.TrimSpace(ips)); ip != nil {
			return strings.TrimSpace(ips)
		}
	}

	// Fall back to RemoteAddr
	if remoteAddr != "" {
		host, _, err := net.SplitHostPort(remoteAddr)
		if err == nil && host != "" {
			return host
		}
		// Try without port
		if ip := net.ParseIP(remoteAddr); ip != nil {
			return remoteAddr
		}
	}

	return "unknown"
}

// GetClientIPFromRequest reads headers directly from the request with Cloudflare support.
// Order: CF-Connecting-IP -> X-Forwarded-For (first IP) -> X-Real-IP -> RemoteAddr.
func GetClientIPFromRequest(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	// Cloudflare specific header
	if cfIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cfIP != "" {
		if ip := net.ParseIP(cfIP); ip != nil {
			return cfIP
		}
	}
	// X-Forwarded-For (first IP)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// first value before comma
		part := xff
		for i, ch := range xff {
			if ch == ',' {
				part = xff[:i]
				break
			}
		}
		part = strings.TrimSpace(part)
		if ip := net.ParseIP(part); ip != nil {
			return part
		}
	}
	// X-Real-IP
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		if ip := net.ParseIP(xri); ip != nil {
			return xri
		}
	}
	// RemoteAddr
	return GetClientIP(r.RemoteAddr, "")
}
