//go:build windows
// +build windows

package client

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// AutoStart handles automatic startup configuration
type AutoStart struct {
	appName  string
	execPath string
}

// NewAutoStart creates a new AutoStart instance
func NewAutoStart(appName string) *AutoStart {
	execPath, _ := os.Executable()
	return &AutoStart{
		appName:  appName,
		execPath: execPath,
	}
}

// Enable enables auto-start for the application
func (as *AutoStart) Enable() error {
	// Windows: Add to registry Run key
	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %v", err)
	}
	defer key.Close()

	err = key.SetStringValue(as.appName, as.execPath)
	if err != nil {
		return fmt.Errorf("failed to set registry value: %v", err)
	}

	return nil
}

// Disable disables auto-start for the application
func (as *AutoStart) Disable() error {
	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %v", err)
	}
	defer key.Close()

	err = key.DeleteValue(as.appName)
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("failed to delete registry value: %v", err)
	}

	return nil
}

// IsEnabled checks if auto-start is enabled
func (as *AutoStart) IsEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	_, _, err = key.GetStringValue(as.appName)
	return err == nil
}

// EnableStartupFolder adds shortcut to startup folder (alternative method)
func (as *AutoStart) EnableStartupFolder() error {
	startupDir := filepath.Join(os.Getenv("APPDATA"),
		`Microsoft\Windows\Start Menu\Programs\Startup`)

	// Note: Creating .lnk files properly requires COM/OLE
	// This is a simplified version - for production use a library like
	// github.com/go-ole/go-ole to create proper shortcuts

	// For now, we'll create a batch file as a workaround
	batchPath := filepath.Join(startupDir, as.appName+".bat")
	content := fmt.Sprintf("@echo off\nstart \"\" \"%s\"\n", as.execPath)

	return os.WriteFile(batchPath, []byte(content), 0644)
}

// DisableStartupFolder removes shortcut from startup folder
func (as *AutoStart) DisableStartupFolder() error {
	startupDir := filepath.Join(os.Getenv("APPDATA"),
		`Microsoft\Windows\Start Menu\Programs\Startup`)

	batchPath := filepath.Join(startupDir, as.appName+".bat")
	shortcutPath := filepath.Join(startupDir, as.appName+".lnk")

	// Remove both batch and shortcut if they exist
	os.Remove(batchPath)
	os.Remove(shortcutPath)

	return nil
}
