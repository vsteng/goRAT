package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"time"

	"gorat/common"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// CommandExecutor handles command execution
type CommandExecutor struct{}

// NewCommandExecutor creates a new command executor
func NewCommandExecutor() *CommandExecutor {
	return &CommandExecutor{}
}

// Execute executes a command and returns the result
func (e *CommandExecutor) Execute(payload *common.ExecuteCommandPayload) *common.CommandResultPayload {
	startTime := time.Now()

	result := &common.CommandResultPayload{
		Success:  false,
		ExitCode: -1,
	}

	// Create context with timeout
	timeout := time.Duration(payload.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second // Default timeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create command
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows: use cmd.exe
		args := []string{"/c", payload.Command}
		if len(payload.Args) > 0 {
			args = append(args, payload.Args...)
		}
		cmd = exec.CommandContext(ctx, "cmd.exe", args...)
	} else {
		// Linux/Unix: use sh
		args := []string{"-c", payload.Command}
		if len(payload.Args) > 0 {
			fullCmd := payload.Command + " " + joinArgs(payload.Args)
			args = []string{"-c", fullCmd}
		}
		cmd = exec.CommandContext(ctx, "sh", args...)
	}

	// Set working directory
	if payload.WorkDir != "" {
		cmd.Dir = payload.WorkDir
	}

	// Execute command and capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(startTime)

	// Convert output based on OS encoding
	output := e.decodeOutput(stdout.Bytes())
	errOutput := e.decodeOutput(stderr.Bytes())

	// Combine output and error
	if errOutput != "" {
		if output != "" {
			output = output + "\n" + errOutput
		} else {
			output = errOutput
		}
	}

	result.Output = output
	result.Duration = duration.Milliseconds()

	if err != nil {
		result.Error = err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
	} else {
		result.Success = true
		result.ExitCode = 0
	}

	return result
}

// decodeOutput decodes command output based on OS encoding
func (e *CommandExecutor) decodeOutput(data []byte) string {
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

// joinArgs joins command arguments with proper escaping
func joinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}

	result := ""
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		// Simple quoting for args with spaces
		if containsSpace(arg) {
			result += fmt.Sprintf(`"%s"`, arg)
		} else {
			result += arg
		}
	}
	return result
}

// containsSpace checks if a string contains spaces
func containsSpace(s string) bool {
	for _, c := range s {
		if c == ' ' || c == '\t' {
			return true
		}
	}
	return false
}
