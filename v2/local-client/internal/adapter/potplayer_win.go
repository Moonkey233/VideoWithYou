//go:build windows

package adapter

import (
	"errors"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmKeyDown = 0x0100
	wmKeyUp   = 0x0101
	swRestore = 9
)

var (
	user32                   = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows          = user32.NewProc("EnumWindows")
	procGetWindowTextW       = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	procIsWindowVisible      = user32.NewProc("IsWindowVisible")
	procSetForegroundWindow  = user32.NewProc("SetForegroundWindow")
	procShowWindow           = user32.NewProc("ShowWindow")
	procPostMessageW         = user32.NewProc("PostMessageW")
)

func sendHotkeyToPotPlayer(hotkey Hotkey) error {
	hwnd := findPotPlayerWindow()
	if hwnd == 0 {
		return errors.New("potplayer window not found")
	}

	_ = showWindow(hwnd)
	_ = setForegroundWindow(hwnd)

	for _, mod := range hotkey.Modifiers {
		_ = postMessage(hwnd, wmKeyDown, uintptr(mod))
	}
	_ = postMessage(hwnd, wmKeyDown, uintptr(hotkey.Key))
	_ = postMessage(hwnd, wmKeyUp, uintptr(hotkey.Key))
	for i := len(hotkey.Modifiers) - 1; i >= 0; i-- {
		_ = postMessage(hwnd, wmKeyUp, uintptr(hotkey.Modifiers[i]))
	}
	return nil
}

func findPotPlayerWindow() windows.HWND {
	var found windows.HWND
	callback := windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if !isWindowVisible(hwnd) {
			return 1
		}
		title := getWindowTitle(hwnd)
		if strings.Contains(strings.ToLower(title), "potplayer") {
			found = windows.HWND(hwnd)
			return 0
		}
		return 1
	})

	_, _, _ = procEnumWindows.Call(callback, 0)
	return found
}

func isPotPlayerAvailable() bool {
	return findPotPlayerWindow() != 0
}

func isWindowVisible(hwnd uintptr) bool {
	ret, _, _ := procIsWindowVisible.Call(hwnd)
	return ret != 0
}

func getWindowTitle(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if length == 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	_, _, _ = procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf)
}

func setForegroundWindow(hwnd windows.HWND) error {
	ret, _, err := procSetForegroundWindow.Call(uintptr(hwnd))
	if ret == 0 {
		return err
	}
	return nil
}

func showWindow(hwnd windows.HWND) error {
	ret, _, err := procShowWindow.Call(uintptr(hwnd), swRestore)
	if ret == 0 {
		return err
	}
	return nil
}

func postMessage(hwnd windows.HWND, msg uint32, wparam uintptr) error {
	ret, _, err := procPostMessageW.Call(uintptr(hwnd), uintptr(msg), wparam, 0)
	if ret == 0 {
		if err == syscall.Errno(0) {
			return errors.New("postmessage failed")
		}
		return err
	}
	return nil
}
