package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Connected devices on the nav rail: the barcode scanner, the receipt
// printer and the keyboard, each green while it's there and red once it's
// gone.
//
// The printer is the receipt printer in Settings, as Windows' spooler sees
// it. Scanners and keyboards both reach Windows as keyboards, so the till
// learns which is which from how they type (devices_windows.go): a burst of
// keys a few milliseconds apart ending in Enter is the scanner, keys at a
// person's pace are the keyboard. What it learned is kept in devices.json
// next to the branch registry, and a new scanner or keyboard replaces the
// old one the first time it's used.

const (
	deviceOK      = "ok"
	deviceOff     = "off"
	deviceUnknown = "unknown" // no printer chosen, or not seen in use yet
)

// DeviceStatus is what the nav rail shows.
type DeviceStatus struct {
	Scanner  string `json:"scanner"`
	Printer  string `json:"printer"`
	Keyboard string `json:"keyboard"`
	// PrinterName is the receipt printer the status is for.
	PrinterName string `json:"printerName"`
}

// learnedDevices are the input devices the till has seen scan and type,
// by vendor and product ID (or the device path when there's none).
type learnedDevices struct {
	Scanner  string `json:"scanner,omitempty"`
	Keyboard string `json:"keyboard,omitempty"`
}

var devices struct {
	mu      sync.Mutex
	watch   sync.Once
	learned learnedDevices
	loaded  bool
	status  DeviceStatus
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

// learnInputDevice records id as the scanner (or the keyboard) and wakes
// the watcher when that's news.
func learnInputDevice(id string, scanner bool) {
	learned := learnedInputDevices()
	if scanner {
		if learned.Scanner == id {
			return
		}
		learned.Scanner = id
		if learned.Keyboard == id {
			learned.Keyboard = ""
		}
	} else {
		if learned.Keyboard == id || learned.Scanner == id {
			return
		}
		learned.Keyboard = id
	}
	devices.mu.Lock()
	devices.learned = learned
	devices.mu.Unlock()
	log.Printf("devices: learned scanner=%q keyboard=%q", learned.Scanner, learned.Keyboard)
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
	status := DeviceStatus{Scanner: deviceUnknown, Printer: deviceUnknown, Keyboard: deviceUnknown}
	learned := learnedInputDevices()
	if learned.Scanner != "" || learned.Keyboard != "" {
		present := presentInputDevices()
		if present != nil {
			status.Scanner = presence(learned.Scanner, present)
			status.Keyboard = presence(learned.Keyboard, present)
		}
	}
	if a.store != nil {
		if settings, err := a.store.GetSettings(a.ctx); err == nil && settings.ReceiptPrinter != "" {
			status.PrinterName = settings.ReceiptPrinter
			status.Printer = printerStatus(settings.ReceiptPrinter)
		}
	}
	return status
}

func presence(id string, present map[string]bool) string {
	switch {
	case id == "":
		return deviceUnknown
	case present[id]:
		return deviceOK
	default:
		return deviceOff
	}
}
