//go:build !windows
// +build !windows

package filebrowser

import "gorat/pkg/protocol"

func getDrivesWindows() []protocol.DriveInfo {
	return []protocol.DriveInfo{}
}
