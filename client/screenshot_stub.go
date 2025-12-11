//go:build noscreenshot
// +build noscreenshot

package client

import (
	"time"

	"gorat/common"
)

// ScreenshotCapture handles screenshot functionality (stub implementation)
type ScreenshotCapture struct{}

// NewScreenshotCapture creates a new screenshot capture
func NewScreenshotCapture() *ScreenshotCapture {
	return &ScreenshotCapture{}
}

// Capture takes a screenshot and returns the data (stub)
func (sc *ScreenshotCapture) Capture(payload *common.ScreenshotPayload) *common.ScreenshotDataPayload {
	return &common.ScreenshotDataPayload{
		Timestamp: time.Now(),
		Error:     "Screenshot functionality not available (built with noscreenshot tag)",
	}
}

// CaptureAllDisplays captures screenshots from all displays (stub)
func (sc *ScreenshotCapture) CaptureAllDisplays(payload *common.ScreenshotPayload) []*common.ScreenshotDataPayload {
	return []*common.ScreenshotDataPayload{
		{
			Timestamp: time.Now(),
			Error:     "Screenshot functionality not available (built with noscreenshot tag)",
		},
	}
}

// captureDisplay captures a specific display (stub)
func (sc *ScreenshotCapture) captureDisplay(displayIndex int, payload *common.ScreenshotPayload) *common.ScreenshotDataPayload {
	return &common.ScreenshotDataPayload{
		Timestamp: time.Now(),
		Error:     "Screenshot functionality not available (built with noscreenshot tag)",
	}
}

// CaptureRegion captures a specific region of the screen (stub)
func (sc *ScreenshotCapture) CaptureRegion(x, y, width, height int, payload *common.ScreenshotPayload) *common.ScreenshotDataPayload {
	return &common.ScreenshotDataPayload{
		Timestamp: time.Now(),
		Error:     "Screenshot functionality not available (built with noscreenshot tag)",
	}
}
