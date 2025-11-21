//go:build !windows
// +build !windows

package client

import "mww2.com/server_manager/common"

// getDrivesWindows is a stub for non-Windows systems
func getDrivesWindows() []common.DriveInfo {
	return []common.DriveInfo{}
}
