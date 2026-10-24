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
// Mouse: any USB mouse in raw input's device list, other than a touch
// screen's (which Windows lists as a mouse too).
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

	hidDLL                 = windows.NewLazySystemDLL("hid.dll")
	pHidDGetProductString  = hidDLL.NewProc("HidD_GetProductString")
	pHidDGetManufacturerSt = hidDLL.NewProc("HidD_GetManufacturerString")

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
	ridiDeviceInfo      = 0x2000000b
	rimTypeMouse        = 0
	rimTypeHID          = 2
	// A HID collection with this usage page and usage is a touch screen.
	hidUsagePageDigitizer = 0x0D
	hidUsageTouchScreen   = 0x04
	rimTypeKeyboard       = 1
	riKeyBreak            = 0x01 // RAWKEYBOARD.Flags: the key went up

	vkReturn = 0x0D
	vkTabKey = 0x09

	// A scanner types a whole barcode with keys this close together and
	// ends it with Enter (or Tab); a person is never that fast.
	scanKeyGap  = 50 * time.Millisecond
	scanMinKeys = 6

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

// ridDeviceInfoHID is RID_DEVICE_INFO with its HID member, padded to the
// union's full size (the keyboard member is the largest).
type ridDeviceInfoHID struct {
	size, kind                      uint32
	vendorID, productID, versionNum uint32
	usagePage, usage                uint16
	_                               [8]byte
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
	log.Println("devices: listening for scanner and keyboard input")
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
		// RAWINPUT is sized for its largest member (the mouse), more than a
		// keyboard needs; Windows refuses a buffer smaller than that.
		var buf [8]uint64
		size := uint32(unsafe.Sizeof(buf))
		header := unsafe.Offsetof(rawKeyboardInput{}.makeCode)
		r, _, err := pGetRawInputData.Call(lParam, ridInput, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), header)
		in := (*rawKeyboardInput)(unsafe.Pointer(&buf[0]))
		if int32(r) <= 0 {
			logRawInputFailure(err)
		} else if in.kind == rimTypeKeyboard && in.device != 0 && in.flags&riKeyBreak == 0 {
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

var rawInputFailureLogged bool

func logRawInputFailure(err error) {
	if !rawInputFailureLogged {
		rawInputFailureLogged = true
		log.Printf("devices: read raw input: %v", err)
	}
}

// onDeviceKey learns, from the timing of its keys, that a device is the
// scanner.
func onDeviceKey(device uintptr, vKey uint16, now time.Time) {
	c := inputListener.cadence[device]
	if c == nil {
		c = &keyCadence{}
		inputListener.cadence[device] = c
	}
	if !c.last.IsZero() && now.Sub(c.last) < scanKeyGap {
		c.run++
	} else {
		c.run = 1
	}
	c.last = now

	if (vKey == vkReturn || vKey == vkTabKey) && c.run >= scanMinKeys {
		if id := inputDeviceID(device); id != "" {
			learnScanner(id)
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
	return deviceIDFromPath(rawInputDevicePath(device))
}

// rawInputDevicePath is the device's interface path, \\?\HID#VID_...#...
func rawInputDevicePath(device uintptr) string {
	var n uint32
	pGetRawInputDeviceInfoW.Call(device, ridiDeviceName, 0, uintptr(unsafe.Pointer(&n)))
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n)
	if r, _, _ := pGetRawInputDeviceInfoW.Call(device, ridiDeviceName, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))); int32(r) <= 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

func deviceIDFromPath(path string) string {
	path = strings.ToUpper(path)
	if id := vidPidPattern.FindString(path); id != "" {
		return id
	}
	return path
}

// deviceNames caches usbDeviceName by device ID; only watchDevices uses it.
var deviceNames = map[string]string{}

// usbDeviceName is the name a HID device gives itself ("Honeywell Voyager
// 1250g"), or its USB ID when it gives none. Opening it with no access
// rights is allowed even for the keyboards and mice Windows holds open.
func usbDeviceName(id, path string) string {
	if name, ok := deviceNames[id]; ok {
		return name
	}
	name := id
	if handle, err := windows.CreateFile(windows.StringToUTF16Ptr(path), 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0); err == nil {
		product := hidString(pHidDGetProductString, handle)
		maker := hidString(pHidDGetManufacturerSt, handle)
		windows.CloseHandle(handle)
		switch {
		case product != "" && maker != "" && !strings.Contains(strings.ToLower(product), strings.ToLower(maker)):
			name = maker + " " + product
		case product != "":
			name = product
		}
	}
	deviceNames[id] = name
	return name
}

func hidString(proc *windows.LazyProc, handle windows.Handle) string {
	var buf [127]uint16 // the USB limit, plus the terminator
	if r, _, _ := proc.Call(uintptr(handle), uintptr(unsafe.Pointer(&buf[0])), unsafe.Sizeof(buf)); r == 0 {
		return ""
	}
	return strings.TrimSpace(windows.UTF16ToString(buf[:]))
}

// presentInputDevices returns the keyboards and mice plugged in now, by ID
// with their names, or nil when Windows can't say.
func presentInputDevices() (keyboards, mice map[string]string) {
	var n uint32
	entrySize := unsafe.Sizeof(rawInputDeviceListEntry{})
	pGetRawInputDeviceList.Call(0, uintptr(unsafe.Pointer(&n)), entrySize)
	if n == 0 {
		return nil, nil
	}
	list := make([]rawInputDeviceListEntry, n+4) // room for one plugged in meanwhile
	n = uint32(len(list))
	r, _, _ := pGetRawInputDeviceList.Call(uintptr(unsafe.Pointer(&list[0])), uintptr(unsafe.Pointer(&n)), entrySize)
	if int32(r) < 0 {
		return nil, nil
	}
	keyboards, mice = map[string]string{}, map[string]string{}
	touchScreens := map[string]bool{}
	for _, entry := range list[:r] {
		var into map[string]string
		switch entry.kind {
		case rimTypeKeyboard:
			into = keyboards
		case rimTypeMouse:
			into = mice
		case rimTypeHID:
			if isTouchScreen(entry.device) {
				touchScreens[deviceIDFromPath(rawInputDevicePath(entry.device))] = true
			}
			continue
		default:
			continue
		}
		path := rawInputDevicePath(entry.device)
		if id := deviceIDFromPath(path); id != "" {
			into[id] = usbDeviceName(id, path)
		}
	}
	// A touch screen also reports itself as a mouse (and sometimes a
	// keyboard), which would keep those lights green with neither plugged
	// in. Its touch collection shares the USB ID, which gives it away.
	for id := range touchScreens {
		delete(mice, id)
		delete(keyboards, id)
	}
	return keyboards, mice
}

// isTouchScreen reports whether a raw input HID collection is a touch
// screen (a touchpad or pen is a different usage and isn't counted).
func isTouchScreen(device uintptr) bool {
	info := ridDeviceInfoHID{}
	info.size = uint32(unsafe.Sizeof(info))
	size := info.size
	r, _, _ := pGetRawInputDeviceInfoW.Call(device, ridiDeviceInfo, uintptr(unsafe.Pointer(&info)), uintptr(unsafe.Pointer(&size)))
	return int32(r) > 0 && info.kind == rimTypeHID &&
		info.usagePage == hidUsagePageDigitizer && info.usage == hidUsageTouchScreen
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
