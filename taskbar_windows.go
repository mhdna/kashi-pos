//go:build windows

package main

// Taskbar auto-hide while the till is running.
//
// Windows only hides the taskbar over a fullscreen window while that window
// is the active one. Anything that takes focus -- an AnyDesk session popup, a
// printer driver's dialog -- leaves GB Cloud fullscreen but inactive, and the
// taskbar comes back on top of the till. With auto-hide on, the taskbar stays
// out of the way until someone presses the Windows key or touches the bottom
// edge of the screen.
//
// This flips the shell's own "Automatically hide the taskbar" setting through
// SHAppBarMessage (no registry edit, no Explorer restart) and puts back what
// the till had when the app exits. A crash leaves auto-hide on, which is
// harmless; the setting can be changed back by hand in Taskbar settings.

import (
	"log"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	pSHAppBarMessage = shell32.NewProc("SHAppBarMessage")
	pFindWindowW     = user32.NewProc("FindWindowW")
)

const (
	abmGetState = 0x4
	abmSetState = 0xA
	absAutoHide = 0x1
)

// appBarData is the Win32 APPBARDATA struct; the uintptr fields give it the
// same padding as the C layout on both 32- and 64-bit builds.
type appBarData struct {
	cbSize           uint32
	hWnd             uintptr
	uCallbackMessage uint32
	uEdge            uint32
	rc               struct{ left, top, right, bottom int32 }
	lParam           uintptr
}

var taskbar struct {
	mu sync.Mutex
	// savedState is the taskbar state from before autoHideTaskbar changed
	// it, kept for restoreTaskbar; changed is false when nothing was touched.
	savedState uintptr
	changed    bool
}

func taskbarWindow() uintptr {
	className, _ := windows.UTF16PtrFromString("Shell_TrayWnd")
	hwnd, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(className)), 0)
	return hwnd
}

func appBarMessage(message uint32, data *appBarData) uintptr {
	data.cbSize = uint32(unsafe.Sizeof(*data))
	result, _, _ := pSHAppBarMessage.Call(uintptr(message), uintptr(unsafe.Pointer(data)))
	return result
}

// autoHideTaskbar turns on taskbar auto-hide, remembering the previous state.
func autoHideTaskbar() {
	if isDevMode {
		return
	}
	taskbar.mu.Lock()
	defer taskbar.mu.Unlock()
	if taskbar.changed {
		return
	}
	hwnd := taskbarWindow()
	if hwnd == 0 {
		log.Println("taskbar: Shell_TrayWnd not found, leaving it as is")
		return
	}
	data := appBarData{hWnd: hwnd}
	state := appBarMessage(abmGetState, &data)
	if state&absAutoHide != 0 {
		return
	}
	data.lParam = state | absAutoHide
	appBarMessage(abmSetState, &data)
	taskbar.savedState = state
	taskbar.changed = true
}

// restoreTaskbar puts back the taskbar state autoHideTaskbar replaced.
func restoreTaskbar() {
	taskbar.mu.Lock()
	defer taskbar.mu.Unlock()
	if !taskbar.changed {
		return
	}
	data := appBarData{hWnd: taskbarWindow(), lParam: taskbar.savedState}
	appBarMessage(abmSetState, &data)
	taskbar.changed = false
}
