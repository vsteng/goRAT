package client

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"gorat/common"
)

// Updater handles client self-update
type Updater struct {
	currentVersion string
	executablePath string
}

// NewUpdater creates a new updater
func NewUpdater(version string) *Updater {
	execPath, _ := os.Executable()
	return &Updater{
		currentVersion: version,
		executablePath: execPath,
	}
}

// Update performs the update process
func (u *Updater) Update(payload *common.UpdatePayload) *common.UpdateStatusPayload {
	result := &common.UpdateStatusPayload{
		Status:  "downloading",
		Message: fmt.Sprintf("Downloading version %s", payload.Version),
	}

	// Download new version
	tempFile, err := u.downloadUpdate(payload.DownloadURL)
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("Download failed: %v", err)
		return result
	}
	defer os.Remove(tempFile)

	// Verify checksum
	if payload.Checksum != "" {
		result.Status = "verifying"
		result.Message = "Verifying download"

		valid, err := u.verifyChecksum(tempFile, payload.Checksum)
		if err != nil || !valid {
			result.Status = "failed"
			result.Error = "Checksum verification failed"
			return result
		}
	}

	// Install update
	result.Status = "installing"
	result.Message = "Installing update"

	if err := u.installUpdate(tempFile); err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("Installation failed: %v", err)
		return result
	}

	result.Status = "complete"
	result.Message = fmt.Sprintf("Updated to version %s", payload.Version)

	log.Printf("Update completed successfully to version %s", payload.Version)
	return result
}

// downloadUpdate downloads the update file
func (u *Updater) downloadUpdate(url string) (string, error) {
	// Create temporary file
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, fmt.Sprintf("client_update_%d", os.Getpid()))

	out, err := os.Create(tempFile)
	if err != nil {
		return "", err
	}
	defer out.Close()

	// Download file
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Copy to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		os.Remove(tempFile)
		return "", err
	}

	log.Printf("Downloaded update to: %s", tempFile)
	return tempFile, nil
}

// verifyChecksum verifies the downloaded file's checksum
func (u *Updater) verifyChecksum(filePath, expectedChecksum string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false, err
	}

	actualChecksum := hex.EncodeToString(hash.Sum(nil))
	return actualChecksum == expectedChecksum, nil
}

// installUpdate installs the downloaded update
func (u *Updater) installUpdate(newBinaryPath string) error {
	// Make backup of current executable
	backupPath := u.executablePath + ".backup"
	if err := u.copyFile(u.executablePath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %v", err)
	}

	// Make new binary executable
	if err := os.Chmod(newBinaryPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %v", err)
	}

	// Replace current executable
	// On Windows, we may need to rename current process and replace on next boot
	if runtime.GOOS == "windows" {
		return u.installUpdateWindows(newBinaryPath, backupPath)
	}

	return u.installUpdateUnix(newBinaryPath, backupPath)
}

// installUpdateUnix installs update on Unix-like systems
func (u *Updater) installUpdateUnix(newBinaryPath, backupPath string) error {
	// Remove current executable
	if err := os.Remove(u.executablePath); err != nil {
		return fmt.Errorf("failed to remove current executable: %v", err)
	}

	// Move new binary to executable path
	if err := os.Rename(newBinaryPath, u.executablePath); err != nil {
		// Restore backup on failure
		os.Rename(backupPath, u.executablePath)
		return fmt.Errorf("failed to install new executable: %v", err)
	}

	// Remove backup after successful install
	os.Remove(backupPath)

	log.Printf("Update installed successfully at: %s", u.executablePath)
	return nil
}

// installUpdateWindows installs update on Windows
func (u *Updater) installUpdateWindows(newBinaryPath, backupPath string) error {
	// On Windows, we can't replace a running executable directly
	// Strategy: Rename current to .old, copy new binary, then restart

	oldPath := u.executablePath + ".old"

	// Rename current executable
	if err := os.Rename(u.executablePath, oldPath); err != nil {
		return fmt.Errorf("failed to rename current executable: %v", err)
	}

	// Copy new binary to executable path
	if err := u.copyFile(newBinaryPath, u.executablePath); err != nil {
		// Restore on failure
		os.Rename(oldPath, u.executablePath)
		return fmt.Errorf("failed to copy new executable: %v", err)
	}

	log.Printf("Update installed successfully at: %s (old version saved as .old)", u.executablePath)
	log.Printf("Please restart the client to use the new version")

	return nil
}

// copyFile copies a file from src to dst
func (u *Updater) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Copy file permissions
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, sourceInfo.Mode())
}

// RestartClient restarts the client application
func (u *Updater) RestartClient() error {
	// Get current process arguments
	args := os.Args

	// Start new process
	_, err := os.StartProcess(u.executablePath, args, &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})

	if err != nil {
		return fmt.Errorf("failed to start new process: %v", err)
	}

	log.Printf("New client process started, exiting current process")

	// Exit current process
	os.Exit(0)
	return nil
}
