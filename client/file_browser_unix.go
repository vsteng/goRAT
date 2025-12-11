//go:build !windows
// +build !windows

package client

import "gorat/common"

// getDrivesWindows is a stub for non-Windows systems
func getDrivesWindows() []common.DriveInfo {
	return []common.DriveInfo{}
}
