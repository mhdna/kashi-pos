package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Connected devices on the nav rail: the barcode scanner, the receipt
// printer, the keyboard and the mouse, each green while it's there and red once it's
// gone.
//
// The printer is the receipt printer in Settings, as Windows' spooler sees
// it. Scanners reach Windows as keyboards, so the till learns which device
// is the scanner from how it types (devices_windows.go): a burst of keys a
// few milliseconds apart ending in Enter. That's kept in devices.json next to
// the branch registry, and a new scanner replaces the old one on its first
// scan. The keyboard is any other USB keyboard plugged in, used or not, and
// the mouse any USB mouse; a touch screen counts as neither.

const (
	deviceOK      = "ok"
	deviceOff     = "off"
	deviceUnknown = "unknown" // no printer chosen, or the scanner not seen scanning yet
)

// DeviceStatus is what the nav rail shows.
type DeviceStatus struct {
	Scanner  string `json:"scanner"`
	Printer  string `json:"printer"`
	Keyboard string `json:"keyboard"`
	Mouse    string `json:"mouse"`
	// The devices' names for the Devices dialog: the receipt printer as
	// Windows calls it, the others as they name themselves over USB. Empty
	// when there's nothing to name.
	PrinterName  string `json:"printerName"`
	ScannerName  string `json:"scannerName"`
	KeyboardName string `json:"keyboardName"`
	MouseName    string `json:"mouseName"`
}

// learnedDevices is the scanner the till has seen scan, by USB vendor and
// product ID (or the device path when there's none).
type learnedDevices struct {
	Scanner string `json:"scanner,omitempty"`
}

var devices struct {
	mu      sync.Mutex
	watch   sync.Once
	learned learnedDevices
	loaded  bool
	status  DeviceStatus
	// scannerName is the learned scanner's name from when it was last
	// plugged in, to name it while it's unplugged.
	scannerName string
	// recheck wakes watchDevices when a device is plugged in or out.
	recheck chan struct{}
}

func init() {
	devices.recheck = make(chan struct{}, 1)
}

func learnedDevicesPath() (string, error) {
	dir, err := branchRegistryDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "devices.json"), nil
}

// learnedInputDevices returns what the till has learned so far.
func learnedInputDevices() learnedDevices {
	devices.mu.Lock()
	defer devices.mu.Unlock()
	if !devices.loaded {
		devices.loaded = true
		if path, err := learnedDevicesPath(); err == nil {
			if data, err := os.ReadFile(path); err == nil {
				_ = json.Unmarshal(data, &devices.learned)
			}
		}
	}
	return devices.learned
}

// learnScanner records id as the scanner and wakes the watcher when that's
// news.
func learnScanner(id string) {
	learned := learnedInputDevices()
	if learned.Scanner == id {
		return
	}
	learned.Scanner = id
	devices.mu.Lock()
	devices.learned = learned
	devices.mu.Unlock()
	log.Printf("devices: learned scanner %q", id)
	if path, err := learnedDevicesPath(); err == nil {
		data, _ := json.Marshal(learned)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			log.Printf("devices: save %s: %v", path, err)
		}
	}
	recheckDevices()
}

// recheckDevices asks watchDevices to look again now.
func recheckDevices() {
	select {
	case devices.recheck <- struct{}{}:
	default:
	}
}

// GetDeviceStatus returns the latest device status for the nav rail.
func (a *App) GetDeviceStatus() DeviceStatus {
	devices.mu.Lock()
	defer devices.mu.Unlock()
	return devices.status
}

// startDeviceWatch starts watchDevices once; the database can be reopened.
func (a *App) startDeviceWatch() {
	devices.watch.Do(func() { go a.watchDevices() })
}

// watchDevices checks the devices every few seconds, and at once when one
// is plugged in or out, and tells the frontend when anything changed.
func (a *App) watchDevices() {
	startInputDeviceListener()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		if a.store != nil {
			stuckKeysLeftAlone.Store(!a.releaseStuckKeys())
		}
		status := a.checkDevices()
		devices.mu.Lock()
		changed := status != devices.status
		devices.status = status
		devices.mu.Unlock()
		if changed {
			wailsruntime.EventsEmit(a.ctx, "device_status_changed", status)
		}
		select {
		case <-ticker.C:
		case <-devices.recheck:
		}
	}
}

func (a *App) checkDevices() DeviceStatus {
	status := DeviceStatus{Scanner: deviceUnknown, Printer: deviceUnknown, Keyboard: deviceUnknown, Mouse: deviceUnknown}
	if keyboards, mice := presentInputDevices(); keyboards != nil {
		scanner := learnedInputDevices().Scanner
		status.Scanner = presence(scanner, keyboards)
		if name, ok := keyboards[scanner]; ok && scanner != "" {
			devices.mu.Lock()
			devices.scannerName = name
			devices.mu.Unlock()
		}
		devices.mu.Lock()
		status.ScannerName = devices.scannerName
		devices.mu.Unlock()
		status.Keyboard, status.KeyboardName = usbPresence(scanner, keyboards)
		status.Mouse, status.MouseName = usbPresence("", mice)
	}
	if a.store != nil {
		if settings, err := a.store.GetSettings(a.ctx); err == nil && settings.ReceiptPrinter != "" {
			status.PrinterName = settings.ReceiptPrinter
			status.Printer = printerStatus(settings.ReceiptPrinter)
		}
	}
	return status
}

// usbPresence is ok while a USB device in present other than except (the
// scanner, among keyboards) is plugged in, with the names of those plugged
// in. Devices without a USB ID are left out: Windows lists a PS/2 keyboard
// and mouse on most PCs whether they're there or not, and remote sessions
// add virtual ones.
func usbPresence(except string, present map[string]string) (state, names string) {
	var found []string
	for id, name := range present {
		if id != except && strings.HasPrefix(id, "VID_") {
			found = append(found, name)
		}
	}
	if len(found) == 0 {
		return deviceOff, ""
	}
	sort.Strings(found)
	return deviceOK, strings.Join(slices.Compact(found), ", ")
}

func presence(id string, present map[string]string) string {
	if id == "" {
		return deviceUnknown
	}
	if _, ok := present[id]; ok {
		return deviceOK
	}
	return deviceOff
}
