package client

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

// InstanceManager manages single instance enforcement and lifecycle control.
type InstanceManager struct {
	pidFile string
}

// NewInstanceManager creates a new manager using the default cache dir.
func NewInstanceManager() *InstanceManager {
	dir := getDefaultCacheDir()
	return &InstanceManager{pidFile: filepath.Join(dir, "client.pid")}
}

// PIDFile returns the path to the PID file.
func (im *InstanceManager) PIDFile() string { return im.pidFile }

// WritePID writes current process PID to file, creating directory if needed.
func (im *InstanceManager) WritePID() error {
	if err := os.MkdirAll(filepath.Dir(im.pidFile), 0o700); err != nil {
		return err
	}
	pid := os.Getpid()
	return os.WriteFile(im.pidFile, []byte(strconv.Itoa(pid)), 0o600)
}

// ReadPID reads PID from file.
func (im *InstanceManager) ReadPID() (int, error) {
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
func (im *InstanceManager) RemovePID() { _ = os.Remove(im.pidFile) }

// IsProcessRunning tries to detect if a PID refers to a running process.
func IsProcessRunning(pid int) bool {
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

// IsRunning reports whether an existing instance (via PID file) is alive.
func (im *InstanceManager) IsRunning() (bool, int) {
	pid, err := im.ReadPID()
	if err != nil {
		return false, 0
	}
	if IsProcessRunning(pid) {
		return true, pid
	}
	// Stale PID file.
	im.RemovePID()
	return false, 0
}

// Kill attempts to terminate the process recorded in the PID file.
func (im *InstanceManager) Kill() error {
	pid, err := im.ReadPID()
	if err != nil {
		return err
	}
	if !IsProcessRunning(pid) {
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
