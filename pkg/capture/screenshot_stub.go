//go:build windows || noscreenshot
// +build windows noscreenshot

package capture

import (
	"time"

	"gorat/pkg/protocol"
)

// ScreenshotCapture handles screenshot functionality (stub implementation)
type ScreenshotCapture struct{}

// NewScreenshotCapture creates a new screenshot capture
func NewScreenshotCapture() *ScreenshotCapture {
	return &ScreenshotCapture{}
}

// Capture takes a screenshot and returns the data (stub implementation)
func (sc *ScreenshotCapture) Capture(payload *protocol.ScreenshotPayload) *protocol.ScreenshotDataPayload {
	return &protocol.ScreenshotDataPayload{
		Timestamp: time.Now(),
		Error:     "Screenshot not supported on this platform",
	}
}

// CaptureAllDisplays captures screenshots from all displays (stub implementation)
func (sc *ScreenshotCapture) CaptureAllDisplays(payload *protocol.ScreenshotPayload) []*protocol.ScreenshotDataPayload {
	return []*protocol.ScreenshotDataPayload{
		{
			Timestamp: time.Now(),
			Error:     "Screenshot not supported on this platform",
		},
	}
}

// CaptureRegion captures a specific region of the screen (stub implementation)
func (sc *ScreenshotCapture) CaptureRegion(x, y, width, height int, payload *protocol.ScreenshotPayload) *protocol.ScreenshotDataPayload {
	return &protocol.ScreenshotDataPayload{
		Timestamp: time.Now(),
		Error:     "Screenshot not supported on this platform",
	}
}
