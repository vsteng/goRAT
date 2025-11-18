//go:build linux || darwin
// +build linux darwin

package client

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"text/template"
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
	if runtime.GOOS == "linux" {
		return as.enableLinuxSystemd()
	}
	return fmt.Errorf("auto-start not implemented for %s", runtime.GOOS)
}

// Disable disables auto-start for the application
func (as *AutoStart) Disable() error {
	if runtime.GOOS == "linux" {
		return as.disableLinuxSystemd()
	}
	return fmt.Errorf("auto-start not implemented for %s", runtime.GOOS)
}

// IsEnabled checks if auto-start is enabled
func (as *AutoStart) IsEnabled() bool {
	if runtime.GOOS == "linux" {
		servicePath := as.getSystemdServicePath()
		_, err := os.Stat(servicePath)
		return err == nil
	}
	return false
}

// enableLinuxSystemd creates a systemd user service
func (as *AutoStart) enableLinuxSystemd() error {
	servicePath := as.getSystemdServicePath()

	// Create systemd directory if it doesn't exist
	serviceDir := filepath.Dir(servicePath)
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd directory: %v", err)
	}

	// Create service file
	serviceContent := as.generateSystemdService()
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %v", err)
	}

	// Reload systemd and enable service
	// Note: This requires systemd to be available
	// The service will start on next login

	return nil
}

// disableLinuxSystemd removes the systemd user service
func (as *AutoStart) disableLinuxSystemd() error {
	servicePath := as.getSystemdServicePath()

	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %v", err)
	}

	return nil
}

// getSystemdServicePath returns the path to the systemd service file
func (as *AutoStart) getSystemdServicePath() string {
	usr, _ := user.Current()
	return filepath.Join(usr.HomeDir, ".config", "systemd", "user", as.appName+".service")
}

// generateSystemdService generates systemd service file content
func (as *AutoStart) generateSystemdService() string {
	tmpl := `[Unit]
Description={{.AppName}} Client Service
After=network.target

[Service]
Type=simple
ExecStart={{.ExecPath}}
Restart=always
RestartSec=10

[Install]
WantedBy=default.target
`

	t := template.Must(template.New("service").Parse(tmpl))
	var result string
	data := struct {
		AppName  string
		ExecPath string
	}{
		AppName:  as.appName,
		ExecPath: as.execPath,
	}

	var buf []byte
	w := &writer{buf: &buf}
	t.Execute(w, data)
	result = string(buf)

	return result
}

// writer is a simple writer implementation
type writer struct {
	buf *[]byte
}

func (w *writer) Write(p []byte) (n int, err error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}

// EnableRC creates an init script for systems using rc.local
func (as *AutoStart) EnableRC() error {
	// Add to /etc/rc.local (requires root)
	// This is a fallback for systems without systemd
	rcLocalPath := "/etc/rc.local"

	// Check if rc.local exists
	if _, err := os.Stat(rcLocalPath); os.IsNotExist(err) {
		return fmt.Errorf("rc.local not found - systemd is preferred")
	}

	// Note: Modifying /etc/rc.local requires root privileges
	// This is provided as a reference implementation

	return fmt.Errorf("modifying rc.local requires root privileges")
}
