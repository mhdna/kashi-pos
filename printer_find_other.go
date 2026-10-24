//go:build !windows

package main

import "errors"

// Finding printers reads Windows' print spooler; see printer_find_windows.go.

func listPrinterQueues() ([]PrinterChoice, error) { return []PrinterChoice{}, nil }

func liveUSBPrinterPorts() ([]PrinterPort, error) { return nil, nil }

func setPrinterPort(string, string) error {
	return errors.New("changing a printer's port only works on Windows")
}
