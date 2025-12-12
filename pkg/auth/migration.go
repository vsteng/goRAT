package auth

import (
	"log"
	"strings"
)

// MigratePasswordHash converts a password from plain-text or old hash format to bcrypt
// This handles upgrading from SHA256 to bcrypt format
func MigratePasswordHash(oldHash string, plainTextPassword string) (string, error) {
	ph := NewPasswordHasher()

	// If we have a plain text password, hash it directly
	if plainTextPassword != "" {
		return ph.Hash(plainTextPassword)
	}

	// If it looks like a SHA256 hash (64 hex chars), we can't recover the password
	// The system needs to handle password resets for users with old hashes
	if len(oldHash) == 64 && !strings.Contains(oldHash, "$2a$") && !strings.Contains(oldHash, "$2b$") {
		log.Printf("⚠️ PASSWORD MIGRATION: Cannot auto-migrate SHA256 hash without plain text. User will need password reset.")
		return oldHash, nil // Return old hash to signal migration needed
	}

	// If it's already a bcrypt hash, return as-is
	if strings.HasPrefix(oldHash, "$2a$") || strings.HasPrefix(oldHash, "$2b$") {
		return oldHash, nil
	}

	return oldHash, nil
}

// NeedsMigration checks if a password hash needs to be upgraded to bcrypt
func NeedsMigration(hash string) bool {
	// SHA256 hashes are 64 hex characters
	// Bcrypt hashes start with $2a$ or $2b$
	if len(hash) == 64 && !strings.Contains(hash, "$") {
		return true
	}
	return false
}
