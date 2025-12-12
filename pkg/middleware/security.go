package middleware

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

// ValidatePath ensures a file path doesn't traverse outside the base directory
func ValidatePath(basePath, userPath string) (string, error) {
	// Remove any null bytes
	userPath = strings.ReplaceAll(userPath, "\x00", "")

	// Clean the path to normalize it
	fullPath := filepath.Join(basePath, userPath)
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("invalid base path: %w", err)
	}

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Ensure the absolute path starts with the base path
	if !strings.HasPrefix(absPath, absBase) {
		return "", fmt.Errorf("path traversal detected: %s not under %s", absPath, absBase)
	}

	return absPath, nil
}

// SecureCookie returns a properly secured HTTP cookie
func SecureCookie(name, value string, maxAge int, secure, httpOnly bool) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: httpOnly,
		Secure:   secure,
		SameSite: http.SameSite(2), // 2 = Lax
	}
}

// SessionCookie returns a session cookie with standard security settings
func SessionCookie(sessionID string) *http.Cookie {
	return SecureCookie("session_id", sessionID, 3600, true, true)
}

// ExpiredCookie returns a cookie that has been expired (for logout)
func ExpiredCookie(name string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSite(2), // 2 = Lax
	}
}
