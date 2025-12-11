package client

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"gorat/common"
)

// TerminalSession represents an active terminal session
type TerminalSession struct {
	ID     string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	mu     sync.Mutex
	done   chan struct{}
}

// TerminalManager manages terminal sessions
type TerminalManager struct {
	sessions map[string]*TerminalSession
	mu       sync.RWMutex
	onOutput func(sessionID, data string)
	onError  func(sessionID, data string)
}

// NewTerminalManager creates a new terminal manager
func NewTerminalManager() *TerminalManager {
	return &TerminalManager{
		sessions: make(map[string]*TerminalSession),
	}
}

// SetOutputCallback sets the callback for terminal output
func (tm *TerminalManager) SetOutputCallback(callback func(sessionID, data string)) {
	tm.onOutput = callback
}

// SetErrorCallback sets the callback for terminal errors
func (tm *TerminalManager) SetErrorCallback(callback func(sessionID, data string)) {
	tm.onError = callback
}

// StartSession starts a new terminal session
func (tm *TerminalManager) StartSession(sessionID, shell string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check if session already exists
	if _, exists := tm.sessions[sessionID]; exists {
		return fmt.Errorf("session already exists: %s", sessionID)
	}

	// Determine shell command
	shellCmd := tm.getShellCommand(shell)

	// Create command
	cmd := exec.Command(shellCmd[0], shellCmd[1:]...)

	// Get stdin, stdout, stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start command
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return fmt.Errorf("failed to start shell: %w", err)
	}

	// Create session
	session := &TerminalSession{
		ID:     sessionID,
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		done:   make(chan struct{}),
	}

	tm.sessions[sessionID] = session

	// Start reading output
	go tm.readOutput(session)
	go tm.readError(session)

	// Monitor process
	go tm.monitorProcess(session)

	log.Printf("Started terminal session: %s", sessionID)
	return nil
}

// getShellCommand returns the appropriate shell command for the OS
func (tm *TerminalManager) getShellCommand(shell string) []string {
	if shell != "" {
		return []string{shell}
	}

	switch runtime.GOOS {
	case "windows":
		return []string{"cmd.exe"}
	case "darwin", "linux":
		// Try to use bash, fallback to sh
		return []string{"/bin/bash"}
	default:
		return []string{"/bin/sh"}
	}
}

// WriteInput writes input to a terminal session
func (tm *TerminalManager) WriteInput(sessionID, data string) error {
	tm.mu.RLock()
	session, exists := tm.sessions[sessionID]
	tm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.stdin == nil {
		return fmt.Errorf("session stdin closed: %s", sessionID)
	}

	_, err := session.stdin.Write([]byte(data))
	return err
}

// StopSession stops a terminal session
func (tm *TerminalManager) StopSession(sessionID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	session, exists := tm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Close stdin to signal end of input
	if session.stdin != nil {
		session.stdin.Close()
	}

	// Kill the process and its children
	if session.cmd != nil && session.cmd.Process != nil {
		killProcessTree(session.cmd.Process)
	}

	// Wait for process to finish (with timeout)
	go func() {
		done := make(chan error, 1)
		go func() {
			done <- session.cmd.Wait()
		}()

		select {
		case <-done:
			// Process finished normally
		case <-time.After(2 * time.Second):
			// Force kill if still running
			if session.cmd.Process != nil {
				session.cmd.Process.Kill()
			}
		}

		close(session.done)
	}()

	delete(tm.sessions, sessionID)
	log.Printf("Stopped terminal session: %s", sessionID)

	return nil
}

// killProcessTree kills a process and all its children
func killProcessTree(proc *os.Process) error {
	if proc == nil {
		return nil
	}

	if runtime.GOOS == "windows" {
		// On Windows, use taskkill to kill process tree
		cmd := exec.Command("taskkill", "/PID", fmt.Sprintf("%d", proc.Pid), "/T", "/F")
		return cmd.Run()
	} else {
		// On Unix, try to send SIGTERM to process group
		// First try SIGTERM for graceful shutdown
		proc.Signal(os.Interrupt)

		// Wait a bit for graceful shutdown
		time.Sleep(500 * time.Millisecond)

		// Force kill if still running
		return proc.Kill()
	}
}

// readOutput reads stdout from the terminal
func (tm *TerminalManager) readOutput(session *TerminalSession) {
	scanner := bufio.NewScanner(session.stdout)
	scanner.Split(bufio.ScanBytes) // Read byte by byte for real-time output

	buffer := make([]byte, 0, 1024)

	for scanner.Scan() {
		b := scanner.Bytes()[0]
		buffer = append(buffer, b)

		// Send output in chunks or when newline is encountered
		if b == '\n' || len(buffer) >= 512 {
			if tm.onOutput != nil {
				// Decode output based on OS encoding
				decodedOutput := tm.decodeOutput(buffer)
				tm.onOutput(session.ID, decodedOutput)
			}
			buffer = buffer[:0]
		}
	}

	// Send any remaining data
	if len(buffer) > 0 && tm.onOutput != nil {
		decodedOutput := tm.decodeOutput(buffer)
		tm.onOutput(session.ID, decodedOutput)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading stdout for session %s: %v", session.ID, err)
	}
}

// readError reads stderr from the terminal
func (tm *TerminalManager) readError(session *TerminalSession) {
	scanner := bufio.NewScanner(session.stderr)

	for scanner.Scan() {
		if tm.onError != nil {
			// Decode error output based on OS encoding
			decodedError := tm.decodeOutput(scanner.Bytes())
			tm.onError(session.ID, decodedError)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading stderr for session %s: %v", session.ID, err)
	}
}

// monitorProcess monitors the terminal process and cleans up when it exits
func (tm *TerminalManager) monitorProcess(session *TerminalSession) {
	session.cmd.Wait()

	tm.mu.Lock()
	delete(tm.sessions, session.ID)
	tm.mu.Unlock()

	log.Printf("Terminal session ended: %s", session.ID)

	// Send exit notification
	if tm.onOutput != nil {
		tm.onOutput(session.ID, "\r\nSession ended\r\n")
	}
}

// decodeOutput decodes terminal output based on OS encoding
func (tm *TerminalManager) decodeOutput(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// On Windows, try to detect and convert from GBK to UTF-8
	if runtime.GOOS == "windows" {
		// Try GBK decoding
		reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewDecoder())
		decoded, err := io.ReadAll(reader)
		if err == nil {
			return string(decoded)
		}
	}

	// Default: assume UTF-8
	return string(data)
}

// HandleStartTerminal handles a start terminal message
func HandleStartTerminal(tm *TerminalManager, payload *common.StartTerminalPayload) error {
	return tm.StartSession(payload.SessionID, payload.Shell)
}

// HandleTerminalInput handles terminal input
func HandleTerminalInput(tm *TerminalManager, payload *common.TerminalInputPayload) error {
	return tm.WriteInput(payload.SessionID, payload.Data)
}

// HandleStopTerminal handles terminal stop
func HandleStopTerminal(tm *TerminalManager, sessionID string) error {
	return tm.StopSession(sessionID)
}
