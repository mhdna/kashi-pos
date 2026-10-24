package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

// Finding the receipt printer again, from the Devices dialog (admins only).
//
// A USB receipt printer plugged into another socket, or replaced, often
// comes back on a new port (USB002 instead of USB001), sometimes as a new
// printer ("POS-80 (Copy 1)"), and the one in Settings sits offline. The
// fix used to be trying ports one by one in the printer's properties.
// FindPrinters lists the installed printers with their ports and whether
// they're ready, and the USB printer ports that have a printer plugged in
// right now, so the admin can pick the printer that's ready or move the
// receipt printer onto the live port.

// PrinterChoice is an installed printer.
type PrinterChoice struct {
	Name  string `json:"name"`
	Port  string `json:"port"`
	Ready bool   `json:"ready"`
	// Receipt marks the receipt printer in Settings.
	Receipt bool `json:"receipt"`
}

// PrinterPort is a USB printer port with a printer plugged in now.
type PrinterPort struct {
	Port string `json:"port"`
	// Device is the plugged-in printer's USB ID.
	Device string `json:"device"`
	// UsedBy are the installed printers on this port.
	UsedBy []string `json:"usedBy"`
}

// PrinterSearch is what FindPrinters found.
type PrinterSearch struct {
	Printers       []PrinterChoice `json:"printers"`
	LivePorts      []PrinterPort   `json:"livePorts"`
	ReceiptPrinter string          `json:"receiptPrinter"`
}

// FindPrinters lists the installed printers and the live USB printer ports.
func (a *App) FindPrinters() (PrinterSearch, error) {
	search := PrinterSearch{Printers: []PrinterChoice{}, LivePorts: []PrinterPort{}}
	if a.store != nil {
		if settings, err := a.store.GetSettings(a.ctx); err == nil {
			search.ReceiptPrinter = settings.ReceiptPrinter
		}
	}
	printers, err := listPrinterQueues()
	if err != nil {
		return search, err
	}
	byPort := map[string][]string{}
	for i := range printers {
		printers[i].Receipt = printers[i].Name == search.ReceiptPrinter
		for _, port := range strings.Split(printers[i].Port, ",") {
			port = strings.TrimSpace(port)
			byPort[port] = append(byPort[port], printers[i].Name)
		}
	}
	// Ready ones first, the receipt printer first among equals.
	sort.SliceStable(printers, func(i, j int) bool {
		if printers[i].Ready != printers[j].Ready {
			return printers[i].Ready
		}
		return printers[i].Receipt && !printers[j].Receipt
	})
	search.Printers = printers

	ports, err := liveUSBPrinterPorts()
	if err != nil {
		log.Printf("devices: list USB printer ports: %v", err)
	}
	for _, port := range ports {
		port.UsedBy = byPort[port.Port]
		if port.UsedBy == nil {
			port.UsedBy = []string{}
		}
		search.LivePorts = append(search.LivePorts, port)
	}
	return search, nil
}

// UseReceiptPrinter makes name the receipt printer in Settings.
func (a *App) UseReceiptPrinter(name string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	s, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return err
	}
	if err := a.UpdateSettings(s.BranchName, s.BranchCode, s.ApiKey, s.PrinterID, s.PrinterSize, name, s.ScreenPort,
		s.ReceiptWidth, s.ReceiptHeight, s.TaxRate, s.RoundingMode, s.RoundingCurrency, s.ReceiptEnabled != 0,
		s.SalesInventoryID, s.ExchangeRate, s.MaxOverpaymentCents); err != nil {
		return err
	}
	a.emitSettingsChanged()
	recheckDevices()
	return nil
}

// MovePrinterToPort points an installed printer at another port.
func (a *App) MovePrinterToPort(name, port string) error {
	if err := setPrinterPort(name, port); err != nil {
		return err
	}
	recheckDevices()
	return nil
}

// ForgetScanner drops the learned scanner, so the next one that scans is
// taken as the scanner.
func (a *App) ForgetScanner() error {
	devices.mu.Lock()
	devices.learned = learnedDevices{}
	devices.loaded = true
	devices.scannerName = ""
	devices.mu.Unlock()
	path, err := learnedDevicesPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	recheckDevices()
	return nil
}
