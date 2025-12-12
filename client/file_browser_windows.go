//go:build windows
// +build windows

package client

import (
	"syscall"
	"unsafe"

	"gorat/pkg/protocol"
)

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	getLogicalDrives    = kernel32.NewProc("GetLogicalDrives")
	getDriveTypeW       = kernel32.NewProc("GetDriveTypeW")
	getVolumeInfoW      = kernel32.NewProc("GetVolumeInformationW")
	getDiskFreeSpaceExW = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// getDrivesWindows returns a list of available drives on Windows
func getDrivesWindows() []protocol.DriveInfo {
	var drives []protocol.DriveInfo

	// Get logical drives bitmask
	ret, _, _ := getLogicalDrives.Call()
	if ret == 0 {
		return drives
	}

	// Iterate through A-Z
	for i := 0; i < 26; i++ {
		if ret&(1<<uint(i)) != 0 {
			driveLetter := string(rune('A' + i))
			drivePath := driveLetter + ":\\"

			driveInfo := protocol.DriveInfo{
				Name: drivePath,
			}

			// Get drive type
			driveType := getDriveType(drivePath)
			driveInfo.Type = driveType

			// Get volume information (label)
			volumeLabel := getVolumeLabel(drivePath)
			driveInfo.Label = volumeLabel

			// Get disk space information
			totalSize, freeSize := getDiskSpace(drivePath)
			driveInfo.TotalSize = totalSize
			driveInfo.FreeSize = freeSize

			drives = append(drives, driveInfo)
		}
	}

	return drives
}

// getDriveType returns the type of drive (fixed, removable, etc.)
func getDriveType(drivePath string) string {
	pathPtr, _ := syscall.UTF16PtrFromString(drivePath)
	ret, _, _ := getDriveTypeW.Call(uintptr(unsafe.Pointer(pathPtr)))

	switch ret {
	case 0:
		return "unknown"
	case 1:
		return "invalid"
	case 2:
		return "removable"
	case 3:
		return "fixed"
	case 4:
		return "remote"
	case 5:
		return "cdrom"
	case 6:
		return "ramdisk"
	default:
		return "unknown"
	}
}

// getVolumeLabel returns the volume label for a drive
func getVolumeLabel(drivePath string) string {
	pathPtr, _ := syscall.UTF16PtrFromString(drivePath)
	volumeNameBuffer := make([]uint16, 256)

	ret, _, _ := getVolumeInfoW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&volumeNameBuffer[0])),
		uintptr(len(volumeNameBuffer)),
		0, 0, 0, 0, 0,
	)

	if ret == 0 {
		return ""
	}

	return syscall.UTF16ToString(volumeNameBuffer)
}

// getDiskSpace returns total and free space for a drive
func getDiskSpace(drivePath string) (totalSize, freeSize int64) {
	pathPtr, _ := syscall.UTF16PtrFromString(drivePath)
	var freeBytesAvailable, totalBytes, totalFreeBytes int64

	ret, _, _ := getDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)

	if ret == 0 {
		return 0, 0
	}

	return totalBytes, freeBytesAvailable
}
