//go:build windows

package main

// Windows side of the device status (devices.go).
//
// Scanner and keyboard: a hidden window registers for raw keyboard input
// (RIDEV_INPUTSINK, so it hears keys whichever window has focus, and
// RIDEV_DEVNOTIFY, so it hears plugging and unplugging). Raw input names the
// device each key came from, which the ordinary key events the webview gets
// don't. Keys injected by AnyDesk or the on-screen keyboard have no device
// and are ignored.
//
// Printer: the spooler's status for the receipt printer. Windows marks a USB
// printer offline when it's unplugged or switched off.

import (
	"log"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	pRegisterRawInputDevices = user32.NewProc("RegisterRawInputDevices")
	pGetRawInputData         = user32.NewProc("GetRawInputData")
	pGetRawInputDeviceInfoW  = user32.NewProc("GetRawInputDeviceInfoW")
	pGetRawInputDeviceList   = user32.NewProc("GetRawInputDeviceList")

	winspoolDLL    = windows.NewLazySystemDLL("winspool.drv")
	pOpenPrinterW  = winspoolDLL.NewProc("OpenPrinterW")
	pGetPrinterW   = winspoolDLL.NewProc("GetPrinterW")
	pClosePrinterW = winspoolDLL.NewProc("ClosePrinter")
)

const (
	wmInput             = 0x00FF
	wmInputDeviceChange = 0x00FE
	ridevInputSink      = 0x00000100
	ridevDevNotify      = 0x00002000
	ridInput            = 0x10000003
	ridiDeviceName      = 0x20000007
	rimTypeKeyboard     = 1
	riKeyBreak          = 0x01 // RAWKEYBOARD.Flags: the key went up

	vkReturn = 0x0D
	vkTabKey = 0x09

	// A scanner types a whole barcode with keys this close together and
	// ends it with Enter (or Tab); a person is never that fast.
	scanKeyGap     = 50 * time.Millisecond
	scanMinKeys    = 6
	humanKeyGapMin = 80 * time.Millisecond
	humanKeyGapMax = 1500 * time.Millisecond

	printerAttributeWorkOffline = 0x00000400
	// The PRINTER_STATUS_* bits that mean a receipt won't come out.
	printerStatusDown = 0x00000002 | // ERROR
		0x00000010 | // PAPER_OUT
		0x00000040 | // PAPER_PROBLEM
		0x00000080 | // OFFLINE
		0x00001000 | // NOT_AVAILABLE
		0x00400000 // DOOR_OPEN
)

type rawInputDevice struct {
	usagePage, usage uint16
	flags            uint32
	target           uintptr
}

type rawInputDeviceListEntry struct {
	device uintptr
	kind   uint32
}

// rawKeyboardInput is RAWINPUT for a keyboard: RAWINPUTHEADER + RAWKEYBOARD.
type rawKeyboardInput struct {
	kind, size        uint32
	device, wParam    uintptr
	makeCode, flags   uint16
	reserved, vKey    uint16
	message, extraInf uint32
}

// printerInfo2 is PRINTER_INFO_2W.
type printerInfo2 struct {
	serverName, printerName, shareName, portName, driverName, comment,
	location, devMode, sepFile, printProcessor, datatype, parameters,
	securityDescriptor uintptr
	attributes, priority, defaultPriority, startTime, untilTime, status,
	jobs, averagePPM uint32
}

// keyCadence is the timing of one device's recent keys.
type keyCadence struct {
	last time.Time
	run  int // keys in a row, each within scanKeyGap of the one before
}

var vidPidPattern = regexp.MustCompile(`VID_[0-9A-F]{4}&PID_[0-9A-F]{4}`)

// The listener's state; only its own thread touches it.
var inputListener struct {
	ids     map[uintptr]string
	cadence map[uintptr]*keyCadence
}

func startInputDeviceListener() {
	started := make(chan bool)
	go runInputDeviceListener(started)
	if !<-started {
		log.Println("devices: raw input unavailable; scanner and keyboard show as unknown")
	}
}

func runInputDeviceListener(started chan<- bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	inputListener.ids = map[uintptr]string{}
	inputListener.cadence = map[uintptr]*keyCadence{}

	instance, _, _ := pGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString("GBCloudDeviceWatch")
	wc := wndClassExW{
		lpfnWndProc:   windows.NewCallback(inputListenerProc),
		hInstance:     instance,
		lpszClassName: className,
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	// A plain window that is never shown, rather than a message-only one:
	// raw input is reliably delivered to it.
	hwnd, _, err := pCreateWindowExW.Call(wsExToolWindow, uintptr(unsafe.Pointer(className)), 0,
		wsPopup, 0, 0, 0, 0, 0, 0, instance, 0)
	if hwnd == 0 {
		log.Printf("devices: create window: %v", err)
		started <- false
		return
	}
	rid := rawInputDevice{usagePage: 0x01, usage: 0x06, flags: ridevInputSink | ridevDevNotify, target: hwnd}
	if ok, _, err := pRegisterRawInputDevices.Call(uintptr(unsafe.Pointer(&rid)), 1, unsafe.Sizeof(rid)); ok == 0 {
		log.Printf("devices: register raw input: %v", err)
		started <- false
		return
	}
	started <- true

	var m msgW
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func inputListenerProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmInput:
		var in rawKeyboardInput
		size := uint32(unsafe.Sizeof(in))
		header := unsafe.Offsetof(in.makeCode)
		r, _, _ := pGetRawInputData.Call(lParam, ridInput, uintptr(unsafe.Pointer(&in)), uintptr(unsafe.Pointer(&size)), header)
		if int32(r) > 0 && in.kind == rimTypeKeyboard && in.device != 0 && in.flags&riKeyBreak == 0 {
			onDeviceKey(in.device, in.vKey, time.Now())
		}
	case wmInputDeviceChange:
		// The device handle is about to go stale or is new; names are
		// looked up again when needed.
		delete(inputListener.ids, lParam)
		delete(inputListener.cadence, lParam)
		recheckDevices()
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

// onDeviceKey learns, from the timing of its keys, whether a device is a
// scanner or a keyboard.
func onDeviceKey(device uintptr, vKey uint16, now time.Time) {
	c := inputListener.cadence[device]
	if c == nil {
		c = &keyCadence{}
		inputListener.cadence[device] = c
	}
	gap := now.Sub(c.last)
	first := c.last.IsZero()
	c.last = now
	if !first && gap < scanKeyGap {
		c.run++
	} else {
		c.run = 1
	}

	switch {
	case (vKey == vkReturn || vKey == vkTabKey) && c.run >= scanMinKeys:
		if id := inputDeviceID(device); id != "" {
			learnInputDevice(id, true)
		}
	case !first && gap >= humanKeyGapMin && gap <= humanKeyGapMax:
		if id := inputDeviceID(device); id != "" {
			learnInputDevice(id, false)
		}
	}
}

// inputDeviceID is the device's USB vendor and product ID, which stay the
// same whichever port it's plugged into, or its device path without them.
func inputDeviceID(device uintptr) string {
	if id, ok := inputListener.ids[device]; ok {
		return id
	}
	id := rawInputDeviceID(device)
	inputListener.ids[device] = id
	return id
}

func rawInputDeviceID(device uintptr) string {
	var n uint32
	pGetRawInputDeviceInfoW.Call(device, ridiDeviceName, 0, uintptr(unsafe.Pointer(&n)))
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n)
	if r, _, _ := pGetRawInputDeviceInfoW.Call(device, ridiDeviceName, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))); int32(r) <= 0 {
		return ""
	}
	path := strings.ToUpper(windows.UTF16ToString(buf))
	if id := vidPidPattern.FindString(path); id != "" {
		return id
	}
	return path
}

// presentInputDevices returns the IDs of the keyboards plugged in now.
func presentInputDevices() map[string]bool {
	var n uint32
	entrySize := unsafe.Sizeof(rawInputDeviceListEntry{})
	pGetRawInputDeviceList.Call(0, uintptr(unsafe.Pointer(&n)), entrySize)
	if n == 0 {
		return nil
	}
	list := make([]rawInputDeviceListEntry, n+4) // room for one plugged in meanwhile
	n = uint32(len(list))
	r, _, _ := pGetRawInputDeviceList.Call(uintptr(unsafe.Pointer(&list[0])), uintptr(unsafe.Pointer(&n)), entrySize)
	if int32(r) < 0 {
		return nil
	}
	present := map[string]bool{}
	for _, entry := range list[:r] {
		if entry.kind != rimTypeKeyboard {
			continue
		}
		if id := rawInputDeviceID(entry.device); id != "" {
			present[id] = true
		}
	}
	return present
}

// printerStatus asks the spooler whether the printer can print now.
func printerStatus(name string) string {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return deviceOff
	}
	var handle uintptr
	if r, _, _ := pOpenPrinterW.Call(uintptr(unsafe.Pointer(namePtr)), uintptr(unsafe.Pointer(&handle)), 0); r == 0 {
		return deviceOff // not installed
	}
	defer pClosePrinterW.Call(handle)

	var needed uint32
	pGetPrinterW.Call(handle, 2, 0, 0, uintptr(unsafe.Pointer(&needed)))
	if needed < uint32(unsafe.Sizeof(printerInfo2{})) {
		return deviceUnknown
	}
	// uintptr-sized elements keep the struct aligned.
	buf := make([]uintptr, (uintptr(needed)+unsafe.Sizeof(uintptr(0))-1)/unsafe.Sizeof(uintptr(0)))
	if r, _, _ := pGetPrinterW.Call(handle, 2, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed))); r == 0 {
		return deviceUnknown
	}
	info := (*printerInfo2)(unsafe.Pointer(&buf[0]))
	if info.attributes&printerAttributeWorkOffline != 0 || info.status&printerStatusDown != 0 {
		return deviceOff
	}
	return deviceOK
}
