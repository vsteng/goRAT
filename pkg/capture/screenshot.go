//go:build !windows && !noscreenshot
// +build !windows,!noscreenshot

package capture

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"time"

	"gorat/pkg/protocol"

	"github.com/kbinani/screenshot"
)

// ScreenshotCapture handles screenshot functionality
type ScreenshotCapture struct{}

// NewScreenshotCapture creates a new screenshot capture
func NewScreenshotCapture() *ScreenshotCapture {
	return &ScreenshotCapture{}
}

// Capture takes a screenshot and returns the data
func (sc *ScreenshotCapture) Capture(payload *protocol.ScreenshotPayload) *protocol.ScreenshotDataPayload {
	result := &protocol.ScreenshotDataPayload{
		Timestamp: time.Now(),
		Format:    "png",
	}

	// Get the primary display
	numDisplays := screenshot.NumActiveDisplays()
	if numDisplays == 0 {
		result.Error = "No active displays found"
		return result
	}

	// Capture screenshot from primary display (display 0)
	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Width = bounds.Dx()
	result.Height = bounds.Dy()

	// Encode image
	var buf bytes.Buffer
	quality := payload.Quality
	if quality == 0 {
		quality = 85 // Default quality
	}

	if quality < 100 {
		// Use JPEG for compression
		result.Format = "jpg"
		opts := &jpeg.Options{Quality: quality}
		err = jpeg.Encode(&buf, img, opts)
	} else {
		// Use PNG for lossless
		result.Format = "png"
		err = png.Encode(&buf, img)
	}

	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Data = buf.Bytes()
	return result
}

// CaptureAllDisplays captures screenshots from all displays
func (sc *ScreenshotCapture) CaptureAllDisplays(payload *protocol.ScreenshotPayload) []*protocol.ScreenshotDataPayload {
	numDisplays := screenshot.NumActiveDisplays()
	results := make([]*protocol.ScreenshotDataPayload, numDisplays)

	for i := 0; i < numDisplays; i++ {
		results[i] = sc.captureDisplay(i, payload)
	}

	return results
}

// captureDisplay captures a specific display
func (sc *ScreenshotCapture) captureDisplay(displayIndex int, payload *protocol.ScreenshotPayload) *protocol.ScreenshotDataPayload {
	result := &protocol.ScreenshotDataPayload{
		Timestamp: time.Now(),
		Format:    "png",
	}

	bounds := screenshot.GetDisplayBounds(displayIndex)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Width = bounds.Dx()
	result.Height = bounds.Dy()

	// Encode image
	var buf bytes.Buffer
	quality := payload.Quality
	if quality == 0 {
		quality = 85
	}

	if quality < 100 {
		result.Format = "jpg"
		opts := &jpeg.Options{Quality: quality}
		err = jpeg.Encode(&buf, img, opts)
	} else {
		result.Format = "png"
		err = png.Encode(&buf, img)
	}

	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Data = buf.Bytes()
	return result
}

// CaptureRegion captures a specific region of the screen
func (sc *ScreenshotCapture) CaptureRegion(x, y, width, height int, payload *protocol.ScreenshotPayload) *protocol.ScreenshotDataPayload {
	result := &protocol.ScreenshotDataPayload{
		Timestamp: time.Now(),
		Format:    "png",
	}

	bounds := image.Rect(x, y, x+width, y+height)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Width = width
	result.Height = height

	// Encode image
	var buf bytes.Buffer
	quality := payload.Quality
	if quality == 0 {
		quality = 85
	}

	if quality < 100 {
		result.Format = "jpg"
		opts := &jpeg.Options{Quality: quality}
		err = jpeg.Encode(&buf, img, opts)
	} else {
		result.Format = "png"
		err = png.Encode(&buf, img)
	}

	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Data = buf.Bytes()
	return result
}
