//go:build !windows
// +build !windows

package client

import "gorat/pkg/protocol"

// getDrivesWindows is a stub for non-Windows systems
func getDrivesWindows() []protocol.DriveInfo {
	return []protocol.DriveInfo{}
}
