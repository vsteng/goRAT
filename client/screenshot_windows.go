//go:build windows && !noscreenshot
// +build windows,!noscreenshot

package client

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"syscall"
	"time"
	"unsafe"

	"mww2.com/server_manager/common"
)

var (
	user32 *syscall.LazyDLL
	gdi32  *syscall.LazyDLL

	procGetDC                  *syscall.LazyProc
	procReleaseDC              *syscall.LazyProc
	procGetSystemMetrics       *syscall.LazyProc
	procCreateCompatibleDC     *syscall.LazyProc
	procCreateCompatibleBitmap *syscall.LazyProc
	procSelectObject           *syscall.LazyProc
	procBitBlt                 *syscall.LazyProc
	procDeleteObject           *syscall.LazyProc
	procDeleteDC               *syscall.LazyProc
	procGetDIBits              *syscall.LazyProc
)

func initScreenshotDLLs() {
	if user32 != nil {
		return // Already initialized
	}
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32 = syscall.NewLazyDLL("gdi32.dll")
	procGetDC = user32.NewProc("GetDC")
	procReleaseDC = user32.NewProc("ReleaseDC")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject = gdi32.NewProc("SelectObject")
	procBitBlt = gdi32.NewProc("BitBlt")
	procDeleteObject = gdi32.NewProc("DeleteObject")
	procDeleteDC = gdi32.NewProc("DeleteDC")
	procGetDIBits = gdi32.NewProc("GetDIBits")
}

const (
	SM_CXSCREEN    = 0
	SM_CYSCREEN    = 1
	SRCCOPY        = 0x00CC0020
	BI_RGB         = 0
	DIB_RGB_COLORS = 0
)

type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type BITMAPINFO struct {
	BmiHeader BITMAPINFOHEADER
	BmiColors [1]uint32
}

// ScreenshotCapture handles screenshot functionality with RDP/Console support
type ScreenshotCapture struct{}

// NewScreenshotCapture creates a new screenshot capture
func NewScreenshotCapture() *ScreenshotCapture {
	return &ScreenshotCapture{}
}

// Capture takes a screenshot using Windows API (works in RDP and console)
func (sc *ScreenshotCapture) Capture(payload *common.ScreenshotPayload) *common.ScreenshotDataPayload {
	result := &common.ScreenshotDataPayload{
		Timestamp: time.Now(),
		Format:    "png",
	}

	img, err := sc.captureScreen()
	if err != nil {
		result.Error = err.Error()
		return result
	}

	bounds := img.Bounds()
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
func (sc *ScreenshotCapture) CaptureAllDisplays(payload *common.ScreenshotPayload) []*common.ScreenshotDataPayload {
	// For Windows, we'll capture the primary screen
	// Multi-monitor support can be added using EnumDisplayMonitors API
	return []*common.ScreenshotDataPayload{sc.Capture(payload)}
}

// captureDisplay captures a specific display
func (sc *ScreenshotCapture) captureDisplay(displayIndex int, payload *common.ScreenshotPayload) *common.ScreenshotDataPayload {
	// Default to primary display
	return sc.Capture(payload)
}

// CaptureRegion captures a specific region of the screen
func (sc *ScreenshotCapture) CaptureRegion(x, y, width, height int, payload *common.ScreenshotPayload) *common.ScreenshotDataPayload {
	result := &common.ScreenshotDataPayload{
		Timestamp: time.Now(),
		Format:    "png",
	}

	img, err := sc.captureScreenRegion(x, y, width, height)
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

// captureScreen captures the entire screen using Windows GDI API
// This works in both RDP sessions and console sessions
func (sc *ScreenshotCapture) captureScreen() (image.Image, error) {
	initScreenshotDLLs()
	// Get screen dimensions
	width, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	height, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)

	if width == 0 || height == 0 {
		return nil, fmt.Errorf("failed to get screen dimensions")
	}

	return sc.captureScreenRegion(0, 0, int(width), int(height))
}

// captureScreenRegion captures a specific region using Windows GDI API
func (sc *ScreenshotCapture) captureScreenRegion(x, y, width, height int) (image.Image, error) {
	// Get device context for the entire screen
	// Using NULL (0) gets the DC for the entire screen, which works in RDP
	hDCScreen, _, err := procGetDC.Call(0)
	if hDCScreen == 0 {
		return nil, fmt.Errorf("GetDC failed: %v", err)
	}
	defer procReleaseDC.Call(0, hDCScreen)

	// Create a compatible DC
	hDCMem, _, err := procCreateCompatibleDC.Call(hDCScreen)
	if hDCMem == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed: %v", err)
	}
	defer procDeleteDC.Call(hDCMem)

	// Create a compatible bitmap
	hBitmap, _, err := procCreateCompatibleBitmap.Call(hDCScreen, uintptr(width), uintptr(height))
	if hBitmap == 0 {
		return nil, fmt.Errorf("CreateCompatibleBitmap failed: %v", err)
	}
	defer procDeleteObject.Call(hBitmap)

	// Select the bitmap into the memory DC
	oldBitmap, _, _ := procSelectObject.Call(hDCMem, hBitmap)
	defer procSelectObject.Call(hDCMem, oldBitmap)

	// Copy the screen into the bitmap
	ret, _, err := procBitBlt.Call(
		hDCMem,
		0, 0,
		uintptr(width), uintptr(height),
		hDCScreen,
		uintptr(x), uintptr(y),
		SRCCOPY,
	)
	if ret == 0 {
		return nil, fmt.Errorf("BitBlt failed: %v", err)
	}

	// Prepare BITMAPINFO structure
	var bi BITMAPINFO
	bi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bi.BmiHeader))
	bi.BmiHeader.BiWidth = int32(width)
	bi.BmiHeader.BiHeight = -int32(height) // negative for top-down bitmap
	bi.BmiHeader.BiPlanes = 1
	bi.BmiHeader.BiBitCount = 32
	bi.BmiHeader.BiCompression = BI_RGB

	// Allocate memory for the bitmap bits
	bitmapSize := width * height * 4
	bitmapData := make([]byte, bitmapSize)

	// Get the bitmap bits
	ret, _, err = procGetDIBits.Call(
		hDCMem,
		hBitmap,
		0,
		uintptr(height),
		uintptr(unsafe.Pointer(&bitmapData[0])),
		uintptr(unsafe.Pointer(&bi)),
		DIB_RGB_COLORS,
	)
	if ret == 0 {
		return nil, fmt.Errorf("GetDIBits failed: %v", err)
	}

	// Convert BGRA to RGBA
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(bitmapData); i += 4 {
		// Windows bitmap is BGRA, we need RGBA
		b := bitmapData[i]
		g := bitmapData[i+1]
		r := bitmapData[i+2]
		a := bitmapData[i+3]

		img.Pix[i] = r
		img.Pix[i+1] = g
		img.Pix[i+2] = b
		img.Pix[i+3] = a
	}

	return img, nil
}
