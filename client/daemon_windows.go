//go:build windows
// +build windows

package client

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
)

// Daemonize runs the client as a background service on Windows
func Daemonize() error {
	// Always spawn a new detached child. The parent should not set DAEMON_MODE.

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Pass through remaining args
	cmd := exec.Command(execPath, os.Args[1:]...)

	// Ensure child knows it's running in daemon mode
	cmd.Env = append(os.Environ(), "DAEMON_MODE=1")

	// Detach window / run in background (CREATE_NO_WINDOW)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}

	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start detached process: %v", err)
	}

	log.Printf("[daemon] Spawned detached child PID=%d", cmd.Process.Pid)
	os.Exit(0)
	return nil
}

// isRunningDetached checks if the process is running detached
func isRunningDetached() bool {
	// Check if we have a console window
	// This is a simplified check - in production you might want more sophisticated detection
	return os.Getenv("DAEMON_MODE") == "1"
}

// IsDaemon checks if the process is running as a service/daemon
func IsDaemon() bool {
	return isRunningDetached()
}
