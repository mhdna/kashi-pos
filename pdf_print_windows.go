//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	printerAccessUse     = 0x00000008
	startDocPrinterLevel = 1
)

type printerDefaults struct {
	datatype      *uint16
	devMode       uintptr
	desiredAccess uint32
}

type docInfo1 struct {
	docName    *uint16
	outputFile *uint16
	datatype   *uint16
}

var (
	winspool         = syscall.NewLazyDLL("winspool.drv")
	openPrinter      = winspool.NewProc("OpenPrinterW")
	closePrinter     = winspool.NewProc("ClosePrinter")
	startDocPrinter  = winspool.NewProc("StartDocPrinterW")
	endDocPrinter    = winspool.NewProc("EndDocPrinter")
	startPagePrinter = winspool.NewProc("StartPagePrinter")
	endPagePrinter   = winspool.NewProc("EndPagePrinter")
	writePrinter     = winspool.NewProc("WritePrinter")
)

func printReceiptToPrinter(data printReceiptData) error {
	payload := buildESCPosReceipt(data)
	printerName, err := syscall.UTF16PtrFromString(data.PrinterName)
	if err != nil {
		return err
	}

	var printer uintptr
	defaults := printerDefaults{desiredAccess: printerAccessUse}
	ret, _, callErr := openPrinter.Call(
		uintptr(unsafe.Pointer(printerName)),
		uintptr(unsafe.Pointer(&printer)),
		uintptr(unsafe.Pointer(&defaults)),
	)
	if ret == 0 {
		return fmt.Errorf("open printer %q failed: %w", data.PrinterName, callErr)
	}
	defer closePrinter.Call(printer)

	docName, _ := syscall.UTF16PtrFromString("Kashi POS Receipt")
	raw, _ := syscall.UTF16PtrFromString("RAW")
	docInfo := docInfo1{docName: docName, datatype: raw}
	if ret, _, callErr := startDocPrinter.Call(printer, startDocPrinterLevel, uintptr(unsafe.Pointer(&docInfo))); ret == 0 {
		return fmt.Errorf("start printer document failed: %w", callErr)
	}
	docStarted := true
	defer func() {
		if docStarted {
			endDocPrinter.Call(printer)
		}
	}()

	if ret, _, callErr := startPagePrinter.Call(printer); ret == 0 {
		return fmt.Errorf("start printer page failed: %w", callErr)
	}
	pageStarted := true
	defer func() {
		if pageStarted {
			endPagePrinter.Call(printer)
		}
	}()

	var written uint32
	if len(payload) == 0 {
		return fmt.Errorf("empty receipt payload")
	}
	if ret, _, callErr := writePrinter.Call(printer, uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), uintptr(unsafe.Pointer(&written))); ret == 0 {
		return fmt.Errorf("write receipt to printer failed: %w", callErr)
	}
	if written != uint32(len(payload)) {
		return fmt.Errorf("printer accepted %d of %d receipt bytes", written, len(payload))
	}
	return nil
}
