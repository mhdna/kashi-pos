//go:build windows

package main

// Kiosk mode: while GB Cloud runs, the till shows nothing else.
//
// Auto-hiding the taskbar wasn't enough: Windows brings it back whenever
// another program takes focus or flashes its button, and the program's own
// window (a Microsoft 365 sign-in, an AnyDesk session window, a "finish
// setting up" nag) lands on top of the till anyway. So for as long as the
// app runs:
//
//   - Explorer, which draws the taskbar, Start, the tray, the desktop and
//     notifications, is asked to exit, the same clean exit as the old
//     "Exit Explorer" taskbar menu item, so Windows doesn't restart it.
//   - The till's window stays topmost, and whenever another program's window
//     takes focus the till takes it straight back, so a barcode scan never
//     types into someone else's window.
//   - The Windows key, Alt+Tab, Alt+Esc, Ctrl+Esc and Alt+F4 do nothing
//     while the till is in front.
//   - Minimized windows (AnyDesk's session window, say) aren't drawn.
//   - Other programs are closed (cash machine mode, cash_machine.go).
//
// Closing the app (Quit -> Close App) starts Explorer again, so that's the
// way to Windows. Shut Down and Restart in the same dialog leave it off.
// Ctrl+Alt+Del and Ctrl+Shift+Esc (Task Manager, which the till lets come
// to the front) still work; from Task Manager, Run new task -> explorer
// brings Windows back if GB Cloud has crashed. Nothing here is stored:
// Explorer is still Windows' shell, so a restart always brings it back.
//
// Off in `wails dev` builds, and on a till with KASHI_KIOSK=false.

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	pFindWindowW              = user32.NewProc("FindWindowW")
	pSetWinEventHook          = user32.NewProc("SetWinEventHook")
	pUnhookWinEvent           = user32.NewProc("UnhookWinEvent")
	pSetWindowsHookExW        = user32.NewProc("SetWindowsHookExW")
	pUnhookWindowsHookEx      = user32.NewProc("UnhookWindowsHookEx")
	pCallNextHookEx           = user32.NewProc("CallNextHookEx")
	pPostThreadMessageW       = user32.NewProc("PostThreadMessageW")
	pGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	pSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	pBringWindowToTop         = user32.NewProc("BringWindowToTop")
	pGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	pAttachThreadInput        = user32.NewProc("AttachThreadInput")
	pIsIconic                 = user32.NewProc("IsIconic")
	pIsWindowVisible          = user32.NewProc("IsWindowVisible")
	pEnumWindows              = user32.NewProc("EnumWindows")
	pGetAsyncKeyState         = user32.NewProc("GetAsyncKeyState")
	pSystemParametersInfoW    = user32.NewProc("SystemParametersInfoW")
	pArrangeIconicWindows     = user32.NewProc("ArrangeIconicWindows")
	pGetDesktopWindow         = user32.NewProc("GetDesktopWindow")
)

const (
	eventSystemForeground = 0x0003
	wineventOutOfContext  = 0x0000
	whKeyboardLL          = 13
	wmQuit                = 0x0012
	// wmExitExplorer is the undocumented message behind the taskbar's
	// "Exit Explorer" menu item: Explorer closes cleanly and Winlogon
	// doesn't restart it.
	wmExitExplorer = 0x05B4

	vkTab     = 0x09
	vkShift   = 0x10
	vkEscape  = 0x1B
	vkLWin    = 0x5B
	vkRWin    = 0x5C
	vkF4      = 0x73
	vkControl = 0x11
	// llkhfAltDown is KBDLLHOOKSTRUCT.flags' "Alt is held" bit.
	llkhfAltDown = 0x20

	swRestore     = 9
	swpNoActivate = 0x0010

	processQueryLimitedInformation = 0x1000

	spiGetMinimizedMetrics = 0x002B
	spiSetMinimizedMetrics = 0x002C
	arwHide                = 0x0008 // MINIMIZEDMETRICS.iArrange: don't draw minimized windows
)

var (
	hwndTopmost   = ^uintptr(0) // HWND_TOPMOST, (HWND)-1
	hwndNoTopmost = ^uintptr(1) // HWND_NOTOPMOST, (HWND)-2
)

// kioskAllowedPrograms may come to the front over the till: Task Manager,
// the way out if the till stops responding, and Windows' administrator
// approval prompt (when it isn't on its own secure screen), which a printer
// port change in the Devices dialog can raise.
var kioskAllowedPrograms = map[string]bool{"taskmgr.exe": true, "consent.exe": true}

// kbdLLHookStruct is the Win32 KBDLLHOOKSTRUCT.
type kbdLLHookStruct struct {
	vkCode, scanCode, flags, time uint32
	extraInfo                     uintptr
}

var kiosk struct {
	mu sync.Mutex
	// threadID is the hook thread's, 0 while kiosk mode is off.
	threadID uint32
	// window is the till's top-level window.
	window atomic.Uintptr
	// reclaims holds when focus was last taken back, to give up on a
	// program that fights for it rather than spin.
	reclaims []time.Time

	foregroundCallback uintptr
	keyboardCallback   uintptr

	// approving is set while Windows' administrator approval prompt is up
	// (beginApprovalPrompt): the till neither takes focus back nor stays on
	// top, so the prompt can be seen and answered.
	approving atomic.Bool
}

// enterKiosk turns kiosk mode on once the till's window is showing.
func enterKiosk() {
	if isDevMode || !envBool("KASHI_KIOSK", true) {
		return
	}
	window := findAppWindow()
	if window == 0 {
		log.Println("kiosk: the till's window wasn't found, leaving Windows as is")
		return
	}
	kiosk.window.Store(window)

	kiosk.mu.Lock()
	if kiosk.threadID != 0 {
		kiosk.mu.Unlock()
		return
	}
	started := make(chan uint32)
	go runKioskHooks(started)
	kiosk.threadID = <-started
	kiosk.mu.Unlock()

	exitExplorer()
	hideMinimizedWindows()
	takeForeground(window, 0)
	startCashMachine()
}

// beginApprovalPrompt gets the till ready for Windows' administrator
// approval prompt (a "runas" ShellExecute) and returns the window to name
// as its owner. Windows only pops the prompt up for a request from the
// window in front; one with no owner waits as a flashing taskbar button,
// which kiosk mode has hidden, so the setup sat waiting for an approval
// nobody could see. Call done once the prompt has been answered.
func beginApprovalPrompt() (owner uintptr, done func()) {
	owner = kiosk.window.Load()
	if owner == 0 {
		// Kiosk mode is off: the till's window is the one in front.
		if foreground, _, _ := pGetForegroundWindow.Call(); windowProcess(foreground) == uint32(os.Getpid()) {
			owner = foreground
		}
	}
	kiosk.approving.Store(true)
	if window := kiosk.window.Load(); window != 0 {
		pSetWindowPos.Call(window, hwndNoTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	}
	return owner, func() {
		kiosk.approving.Store(false)
		kiosk.mu.Lock()
		on := kiosk.threadID != 0
		kiosk.mu.Unlock()
		if window := kiosk.window.Load(); on && window != 0 {
			takeForeground(window, 0)
		}
	}
}

// minimizedMetrics is the Win32 MINIMIZEDMETRICS struct.
type minimizedMetrics struct {
	size, width, horzGap, vertGap, arrange int32
}

// hideMinimizedWindows stops Windows drawing minimized windows as little
// title bars in the bottom-left corner, which it does once Explorer has
// gone: Explorer turns that off while it runs, as minimized windows live on
// its taskbar. Without it, AnyDesk's minimized session window sat on top of
// the till. The change isn't saved; Explorer sets it again when it starts.
func hideMinimizedWindows() {
	metrics := minimizedMetrics{}
	metrics.size = int32(unsafe.Sizeof(metrics))
	pSystemParametersInfoW.Call(spiGetMinimizedMetrics, uintptr(metrics.size), uintptr(unsafe.Pointer(&metrics)), 0)
	if metrics.arrange&arwHide != 0 {
		return
	}
	metrics.arrange |= arwHide
	if r, _, err := pSystemParametersInfoW.Call(spiSetMinimizedMetrics, uintptr(metrics.size), uintptr(unsafe.Pointer(&metrics)), 0); r == 0 {
		log.Printf("kiosk: hide minimized windows: %v", err)
		return
	}
	// Windows already minimized move to the new (hidden) arrangement.
	desktop, _, _ := pGetDesktopWindow.Call()
	pArrangeIconicWindows.Call(desktop)
}

// leaveKiosk turns kiosk mode off and, unless the app is closing for a
// shutdown or an update, starts Explorer again.
func leaveKiosk() {
	stopCashMachine()
	kiosk.mu.Lock()
	if kiosk.threadID != 0 {
		pPostThreadMessageW.Call(uintptr(kiosk.threadID), wmQuit, 0, 0)
		kiosk.threadID = 0
	}
	kiosk.mu.Unlock()
	if !kioskKeepShellOff.Load() {
		startExplorer()
	}
}

// findAppWindow waits for the till's visible top-level window.
func findAppWindow() uintptr {
	pid := uint32(os.Getpid())
	var found uintptr
	callback := windows.NewCallback(func(hwnd, _ uintptr) uintptr {
		if visible, _, _ := pIsWindowVisible.Call(hwnd); visible == 0 {
			return 1
		}
		if windowProcess(hwnd) != pid {
			return 1
		}
		found = hwnd
		return 0
	})
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		pEnumWindows.Call(callback, 0)
		if found != 0 {
			return found
		}
		time.Sleep(200 * time.Millisecond)
	}
	return 0
}

func windowProcess(hwnd uintptr) uint32 {
	var pid uint32
	pGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return pid
}

// programName is the lower-case file name of the process's executable.
func programName(pid uint32) string {
	h, err := windows.OpenProcess(processQueryLimitedInformation, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return ""
	}
	return strings.ToLower(filepath.Base(windows.UTF16ToString(buf[:size])))
}

// runKioskHooks installs the focus and keyboard hooks on a thread of its own
// and pumps its messages until leaveKiosk posts WM_QUIT to it.
func runKioskHooks(started chan<- uint32) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if kiosk.foregroundCallback == 0 {
		kiosk.foregroundCallback = windows.NewCallback(onForegroundChanged)
		kiosk.keyboardCallback = windows.NewCallback(onKeyboard)
	}
	eventHook, _, err := pSetWinEventHook.Call(eventSystemForeground, eventSystemForeground, 0,
		kiosk.foregroundCallback, 0, 0, wineventOutOfContext)
	if eventHook == 0 {
		log.Printf("kiosk: focus hook: %v", err)
	}
	module, _, _ := pGetModuleHandleW.Call(0)
	keyboardHook, _, err := pSetWindowsHookExW.Call(whKeyboardLL, kiosk.keyboardCallback, module, 0)
	if keyboardHook == 0 {
		log.Printf("kiosk: keyboard hook: %v", err)
	}
	started <- windows.GetCurrentThreadId()

	var m msgW
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	if eventHook != 0 {
		pUnhookWinEvent.Call(eventHook)
	}
	if keyboardHook != 0 {
		pUnhookWindowsHookEx.Call(keyboardHook)
	}
	if window := kiosk.window.Load(); window != 0 {
		pSetWindowPos.Call(window, hwndNoTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	}
}

// onForegroundChanged is the WinEventProc for EVENT_SYSTEM_FOREGROUND.
func onForegroundChanged(_, _, hwnd, _, _, _, _ uintptr) uintptr {
	window := kiosk.window.Load()
	if hwnd == 0 || window == 0 || kiosk.approving.Load() {
		return 0
	}
	pid := windowProcess(hwnd)
	switch {
	case pid == uint32(os.Getpid()):
		// The till, or one of its own dialogs: keep it above everything.
		pSetWindowPos.Call(window, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	case kioskAllowedPrograms[programName(pid)]:
		// Let it show over the till until the till is used again.
		pSetWindowPos.Call(window, hwndNoTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	case fightingForFocus():
		// A program that takes focus straight back; the till stays topmost
		// and the next change of focus tries again.
	default:
		takeForeground(window, hwnd)
	}
	return 0
}

// fightingForFocus reports whether focus has been taken back too often in
// the last second.
func fightingForFocus() bool {
	now := time.Now()
	recent := kiosk.reclaims[:0]
	for _, t := range kiosk.reclaims {
		if now.Sub(t) < time.Second {
			recent = append(recent, t)
		}
	}
	kiosk.reclaims = append(recent, now)
	return len(kiosk.reclaims) > 10
}

// takeForeground brings the till's window back to the front and gives it
// focus. Windows only lets the program that has focus hand it on, so this
// borrows the current foreground window's input state for the call.
func takeForeground(window, from uintptr) {
	if iconic, _, _ := pIsIconic.Call(window); iconic != 0 {
		pShowWindow.Call(window, swRestore)
	}
	me := windows.GetCurrentThreadId()
	var fromThread uintptr
	if from != 0 {
		fromThread, _, _ = pGetWindowThreadProcessId.Call(from, 0)
	}
	attached := false
	if fromThread != 0 && uint32(fromThread) != me {
		r, _, _ := pAttachThreadInput.Call(uintptr(me), fromThread, 1)
		attached = r != 0
	}
	pSetWindowPos.Call(window, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
	pBringWindowToTop.Call(window)
	pSetForegroundWindow.Call(window)
	if attached {
		pAttachThreadInput.Call(uintptr(me), fromThread, 0)
	}
}

// onKeyboard is the low-level keyboard hook: it swallows the keys that
// would leave the till while the till is in front, but not the device
// listener's release of a stuck key.
func onKeyboard(code, wParam, lParam uintptr) uintptr {
	if int32(code) >= 0 {
		if foreground, _, _ := pGetForegroundWindow.Call(); foreground == kiosk.window.Load() {
			key := (*kbdLLHookStruct)(unsafe.Pointer(lParam))
			if key.extraInfo != stuckKeyRelease && blockedKey(key) {
				return 1
			}
		}
	}
	r, _, _ := pCallNextHookEx.Call(0, code, wParam, lParam)
	return r
}

func blockedKey(key *kbdLLHookStruct) bool {
	alt := key.flags&llkhfAltDown != 0
	switch key.vkCode {
	case vkLWin, vkRWin:
		return true
	case vkTab, vkF4:
		return alt
	case vkEscape:
		// Ctrl+Shift+Esc opens Task Manager, the way out; Ctrl+Esc opens Start.
		return alt || (keyDown(vkControl) && !keyDown(vkShift))
	}
	return false
}

func keyDown(vk uintptr) bool {
	state, _, _ := pGetAsyncKeyState.Call(vk)
	return state&0x8000 != 0
}

func taskbarWindow() uintptr {
	className, _ := windows.UTF16PtrFromString("Shell_TrayWnd")
	hwnd, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(className)), 0)
	return hwnd
}

// exitExplorer asks Explorer to close and waits for its taskbar to go.
func exitExplorer() {
	tray := taskbarWindow()
	if tray == 0 {
		return
	}
	pPostMessageW.Call(tray, wmExitExplorer, 0, 0)
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		time.Sleep(100 * time.Millisecond)
		if taskbarWindow() == 0 {
			return
		}
	}
	log.Println("kiosk: Explorer didn't exit; the till stays on top of the taskbar instead")
}

// startExplorer brings back the taskbar and desktop if Explorer isn't
// running. Started with no shell running, Explorer becomes the shell.
func startExplorer() {
	if taskbarWindow() != 0 {
		return
	}
	cmd := exec.Command(filepath.Join(os.Getenv("WINDIR"), "explorer.exe"))
	if err := cmd.Start(); err != nil {
		log.Printf("kiosk: start Explorer: %v", err)
		return
	}
	_ = cmd.Process.Release()
}

// startPowerCommand asks Windows to shut down or restart in a few seconds,
// time for the app to close cleanly first.
func startPowerCommand(restart bool) error {
	mode := "/s"
	if restart {
		mode = "/r"
	}
	return exec.Command(filepath.Join(os.Getenv("WINDIR"), "System32", "shutdown.exe"), mode, "/t", "3").Run()
}
