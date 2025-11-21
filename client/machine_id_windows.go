//go:build windows
// +build windows

package client

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v3/host"
	"golang.org/x/sys/windows/registry"
)

// MachineIDGenerator generates unique machine identifiers
type MachineIDGenerator struct {
	cacheDir string
}

// NewMachineIDGenerator creates a new machine ID generator
func NewMachineIDGenerator() *MachineIDGenerator {
	cacheDir := getDefaultCacheDir()
	return &MachineIDGenerator{cacheDir: cacheDir}
}

// GetMachineID returns a unique machine ID, generating and caching if necessary
func (m *MachineIDGenerator) GetMachineID() (string, error) {
	if cachedID, err := m.readCachedID(); err == nil && cachedID != "" {
		return cachedID, nil
	}
	machineID, err := m.generateMachineID()
	if err != nil {
		return "", err
	}
	if err := m.writeCachedID(machineID); err != nil {
		fmt.Printf("Warning: Failed to cache machine ID: %v\n", err)
	}
	return machineID, nil
}

func (m *MachineIDGenerator) generateMachineID() (string, error) {
	var parts []string
	if hostname, err := os.Hostname(); err == nil {
		parts = append(parts, hostname)
	}
	if info, err := host.Info(); err == nil {
		if info.HostID != "" {
			parts = append(parts, info.HostID)
		}
	}
	if id, err := m.readWindowsMachineID(); err == nil && id != "" {
		parts = append(parts, id)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("unable to generate machine ID: no identifiers found")
	}
	combined := strings.Join(parts, "-")
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:16]), nil
}

// readWindowsMachineID reads MachineGuid from registry
func (m *MachineIDGenerator) readWindowsMachineID() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\\Microsoft\\Cryptography`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()
	val, _, err := key.GetStringValue("MachineGuid")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(val), nil
}

// Stub for non-Windows functions
func (m *MachineIDGenerator) readLinuxMachineID() (string, error) {
	return "", fmt.Errorf("not implemented")
}
func (m *MachineIDGenerator) readDarwinMachineID() (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *MachineIDGenerator) readCachedID() (string, error) {
	idFile := filepath.Join(m.cacheDir, "machine-id")
	data, err := ioutil.ReadFile(idFile)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (m *MachineIDGenerator) writeCachedID(id string) error {
	if err := os.MkdirAll(m.cacheDir, 0700); err != nil {
		return err
	}
	idFile := filepath.Join(m.cacheDir, "machine-id")
	return ioutil.WriteFile(idFile, []byte(id), 0600)
}

func getDefaultCacheDir() string {
	appData := os.Getenv("APPDATA")
	if appData != "" {
		return filepath.Join(appData, "ServerManagerClient")
	}
	return filepath.Join(os.Getenv("USERPROFILE"), ".servermanager")
}
