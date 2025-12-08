package server

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// ServerInstanceManager manages single instance enforcement and lifecycle control for the server.
type ServerInstanceManager struct {
	pidFile string
}

// NewServerInstanceManager creates a new server instance manager.
func NewServerInstanceManager() *ServerInstanceManager {
	pidDir := getServerPIDDir()
	return &ServerInstanceManager{pidFile: filepath.Join(pidDir, "server.pid")}
}

// getServerPIDDir returns the directory for server PID file.
func getServerPIDDir() string {
	if runtime.GOOS == "windows" {
		if dir := os.Getenv("PROGRAMDATA"); dir != "" {
			return filepath.Join(dir, "ServerManager")
		}
		return filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local", "ServerManager")
	}
	// Unix-like systems
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "server-manager")
	}
	return filepath.Join(os.TempDir(), "server-manager")
}

// PIDFile returns the path to the PID file.
func (im *ServerInstanceManager) PIDFile() string { return im.pidFile }

// WritePID writes current process PID to file, creating directory if needed.
func (im *ServerInstanceManager) WritePID() error {
	if err := os.MkdirAll(filepath.Dir(im.pidFile), 0o700); err != nil {
		return err
	}
	pid := os.Getpid()
	return os.WriteFile(im.pidFile, []byte(strconv.Itoa(pid)), 0o600)
}

// ReadPID reads PID from file.
func (im *ServerInstanceManager) ReadPID() (int, error) {
	data, err := os.ReadFile(im.pidFile)
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return pid, nil
}

// RemovePID deletes PID file.
func (im *ServerInstanceManager) RemovePID() { _ = os.Remove(im.pidFile) }

// IsProcessRunning tries to detect if a PID refers to a running process.
func IsServerProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	switch runtime.GOOS {
	case "windows":
		// Use tasklist filter
		out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid)).Output()
		if err != nil {
			return false
		}
		// If PID appears exactly once in output lines
		return strings.Contains(string(out), strconv.Itoa(pid))
	default:
		proc, err := os.FindProcess(pid)
		if err != nil {
			return false
		}
		err = proc.Signal(syscall.Signal(0))
		return err == nil
	}
}

// IsRunning reports whether an existing server instance (via PID file) is alive.
func (im *ServerInstanceManager) IsRunning() (bool, int) {
	pid, err := im.ReadPID()
	if err != nil {
		return false, 0
	}
	if IsServerProcessRunning(pid) {
		return true, pid
	}
	// Stale PID file.
	im.RemovePID()
	return false, 0
}

// Kill attempts to terminate the process recorded in the PID file.
func (im *ServerInstanceManager) Kill() error {
	pid, err := im.ReadPID()
	if err != nil {
		return err
	}
	if !IsServerProcessRunning(pid) {
		im.RemovePID()
		return errors.New("process not running")
	}
	switch runtime.GOOS {
	case "windows":
		// Use taskkill
		if err := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/F").Run(); err != nil {
			return fmt.Errorf("taskkill failed: %w", err)
		}
	default:
		proc, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		if err := proc.Signal(syscall.SIGTERM); err != nil {
			// Try SIGKILL as fallback.
			_ = proc.Signal(syscall.SIGKILL)
		}
	}
	im.RemovePID()
	return nil
}
