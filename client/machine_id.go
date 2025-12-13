//go:build !windows
// +build !windows

package client

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v3/host"
)

// MachineIDGenerator generates unique machine identifiers
type MachineIDGenerator struct {
	cacheDir string
}

// NewMachineIDGenerator creates a new machine ID generator
func NewMachineIDGenerator() *MachineIDGenerator {
	cacheDir := getDefaultCacheDir()
	return &MachineIDGenerator{
		cacheDir: cacheDir,
	}
}

// GetMachineID returns a unique machine ID, generating and caching if necessary
func (m *MachineIDGenerator) GetMachineID() (string, error) {
	// Try to read from cache first
	cachedID, err := m.readCachedID()
	if err == nil && cachedID != "" {
		return cachedID, nil
	}

	// Generate new machine ID
	machineID, err := m.generateMachineID()
	if err != nil {
		return "", err
	}

	// Cache the generated ID
	if err := m.writeCachedID(machineID); err != nil {
		// Log warning but continue - ID is still valid
		fmt.Printf("Warning: Failed to cache machine ID: %v\n", err)
	}

	return machineID, nil
}

// generateMachineID generates a unique machine ID based on system info
func (m *MachineIDGenerator) generateMachineID() (string, error) {
	var parts []string

	// Get hostname
	hostname, err := os.Hostname()
	if err == nil {
		parts = append(parts, hostname)
	}

	// Get host UUID (most reliable)
	if info, err := host.Info(); err == nil {
		if info.HostID != "" {
			parts = append(parts, info.HostID)
		}
	}

	// Get OS-specific identifiers
	switch runtime.GOOS {
	case "linux":
		// Try to read machine-id
		if id, err := m.readLinuxMachineID(); err == nil && id != "" {
			parts = append(parts, id)
		}
	case "windows":
		// Try to read Windows machine GUID
		if id, err := m.readWindowsMachineID(); err == nil && id != "" {
			parts = append(parts, id)
		}
	case "darwin":
		// Try to read macOS hardware UUID
		if id, err := m.readDarwinMachineID(); err == nil && id != "" {
			parts = append(parts, id)
		}
	}

	if len(parts) == 0 {
		return "", fmt.Errorf("unable to generate machine ID: no identifiers found")
	}

	// Combine all parts and hash
	combined := strings.Join(parts, "-")
	hash := sha256.Sum256([]byte(combined))
	machineID := hex.EncodeToString(hash[:16]) // Use first 16 bytes (32 hex chars)

	return machineID, nil
}

// readLinuxMachineID reads the Linux machine-id
func (m *MachineIDGenerator) readLinuxMachineID() (string, error) {
	paths := []string{
		"/etc/machine-id",
		"/var/lib/dbus/machine-id",
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(data)), nil
		}
	}

	return "", fmt.Errorf("machine-id not found")
}

// readWindowsMachineID reads the Windows machine GUID
// Windows-specific implementation supplied in machine_id_windows.go
// This stub is excluded from Windows build.
func (m *MachineIDGenerator) readWindowsMachineID() (string, error) {
	return "", fmt.Errorf("not implemented")
}

// readDarwinMachineID reads the macOS hardware UUID
func (m *MachineIDGenerator) readDarwinMachineID() (string, error) {
	// This is a placeholder - macOS implementation would use IOKit
	// For now, we rely on host.Info().HostID which gets this on macOS
	return "", fmt.Errorf("not implemented")
}

// readCachedID reads the cached machine ID
func (m *MachineIDGenerator) readCachedID() (string, error) {
	idFile := filepath.Join(m.cacheDir, "machine-id")
	data, err := os.ReadFile(idFile)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// writeCachedID writes the machine ID to cache
func (m *MachineIDGenerator) writeCachedID(id string) error {
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(m.cacheDir, 0700); err != nil {
		return err
	}

	idFile := filepath.Join(m.cacheDir, "machine-id")
	return os.WriteFile(idFile, []byte(id), 0600)
}

// getDefaultCacheDir returns the default cache directory for the application
func getDefaultCacheDir() string {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "ServerManagerClient")
		}
		return filepath.Join(os.Getenv("USERPROFILE"), ".servermanager")
	case "darwin":
		home := os.Getenv("HOME")
		return filepath.Join(home, "Library", "Application Support", "ServerManagerClient")
	default: // Linux and others
		home := os.Getenv("HOME")
		xdgCache := os.Getenv("XDG_CACHE_HOME")
		if xdgCache != "" {
			return filepath.Join(xdgCache, "servermanager")
		}
		return filepath.Join(home, ".cache", "servermanager")
	}
}
