//go:build windows

package main

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

func printReceiptToPrinter(data printReceiptData) error {
	result := make(chan error, 1)
	go func() { result <- printReceiptToPrinterBlocking(data) }()
	select {
	case err := <-result:
		return err
	case <-time.After(15 * time.Second):
		return fmt.Errorf("print receipt to %q timed out after 15 seconds", data.PrinterName)
	}
}

func printReceiptToPrinterBlocking(data printReceiptData) error {
	payload, err := buildESCPOSRasterReceipt(data)
	if err != nil {
		return err
	}
	printerName, err := syscall.UTF16PtrFromString(data.PrinterName)
	if err != nil {
		return fmt.Errorf("invalid printer name: %w", err)
	}

	winspool := syscall.NewLazyDLL("winspool.drv")
	openPrinter := winspool.NewProc("OpenPrinterW")
	closePrinter := winspool.NewProc("ClosePrinter")
	startDocPrinter := winspool.NewProc("StartDocPrinterW")
	endDocPrinter := winspool.NewProc("EndDocPrinter")
	startPagePrinter := winspool.NewProc("StartPagePrinter")
	endPagePrinter := winspool.NewProc("EndPagePrinter")
	writePrinter := winspool.NewProc("WritePrinter")

	type printerDefaults struct {
		datatype      *uint16
		devMode       uintptr
		desiredAccess uint32
	}
	type docInfo struct {
		docName    *uint16
		outputFile *uint16
		datatype   *uint16
	}

	var printer uintptr
	defaults := printerDefaults{desiredAccess: 0x00000008}
	if result, _, callErr := openPrinter.Call(uintptr(unsafe.Pointer(printerName)), uintptr(unsafe.Pointer(&printer)), uintptr(unsafe.Pointer(&defaults))); result == 0 {
		return fmt.Errorf("open printer %q: %w", data.PrinterName, callErr)
	}
	defer closePrinter.Call(printer)

	docName, _ := syscall.UTF16PtrFromString("Kashi POS Receipt")
	raw, _ := syscall.UTF16PtrFromString("RAW")
	document := docInfo{docName: docName, datatype: raw}
	if result, _, callErr := startDocPrinter.Call(printer, 1, uintptr(unsafe.Pointer(&document))); result == 0 {
		return fmt.Errorf("start receipt document: %w", callErr)
	}
	defer endDocPrinter.Call(printer)
	if result, _, callErr := startPagePrinter.Call(printer); result == 0 {
		return fmt.Errorf("start receipt page: %w", callErr)
	}
	defer endPagePrinter.Call(printer)

	var written uint32
	if result, _, callErr := writePrinter.Call(printer, uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), uintptr(unsafe.Pointer(&written))); result == 0 {
		return fmt.Errorf("write receipt data: %w", callErr)
	}
	if written != uint32(len(payload)) {
		return fmt.Errorf("printer accepted %d of %d receipt bytes", written, len(payload))
	}
	return nil
}
