//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	swHide                = 0
	seeMaskNoCloseProcess = 0x00000040
)

type shellExecuteInfo struct {
	cbSize         uint32
	fMask          uint32
	hwnd           uintptr
	lpVerb         *uint16
	lpFile         *uint16
	lpParameters   *uint16
	lpDirectory    *uint16
	nShow          int32
	hInstApp       uintptr
	lpIDList       uintptr
	lpClass        *uint16
	hkeyClass      uintptr
	dwHotKey       uint32
	hIconOrMonitor uintptr
	hProcess       uintptr
}

var (
	shell32             = syscall.NewLazyDLL("shell32.dll")
	shellExecuteEx      = shell32.NewProc("ShellExecuteExW")
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	waitForSingleObject = kernel32.NewProc("WaitForSingleObject")
	closeHandle         = kernel32.NewProc("CloseHandle")
)

func printPDFToPrinter(pdfPath, printerName string) error {
	verb, err := syscall.UTF16PtrFromString("printto")
	if err != nil {
		return err
	}
	file, err := syscall.UTF16PtrFromString(pdfPath)
	if err != nil {
		return err
	}
	printer, err := syscall.UTF16PtrFromString(fmt.Sprintf("%q", printerName))
	if err != nil {
		return err
	}

	info := shellExecuteInfo{
		cbSize:       uint32(unsafe.Sizeof(shellExecuteInfo{})),
		fMask:        seeMaskNoCloseProcess,
		lpVerb:       verb,
		lpFile:       file,
		lpParameters: printer,
		nShow:        swHide,
	}
	ret, _, callErr := shellExecuteEx.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return fmt.Errorf("Windows PDF print handoff failed: %w", callErr)
	}

	// Wait until the PDF handler has handed the job to the spooler before
	// printReceipt removes the temporary PDF file.
	if info.hProcess != 0 {
		waitForSingleObject.Call(info.hProcess, 30000)
		closeHandle.Call(info.hProcess)
	}
	return nil
}
