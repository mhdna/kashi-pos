//go:build windows

package main

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	pEnumPrintersW = winspoolDLL.NewProc("EnumPrintersW")
	pSetPrinterW   = winspoolDLL.NewProc("SetPrinterW")

	setupapiDLL                       = windows.NewLazySystemDLL("setupapi.dll")
	pSetupDiGetClassDevsW             = setupapiDLL.NewProc("SetupDiGetClassDevsW")
	pSetupDiEnumDeviceInterfaces      = setupapiDLL.NewProc("SetupDiEnumDeviceInterfaces")
	pSetupDiGetDeviceInterfaceDetailW = setupapiDLL.NewProc("SetupDiGetDeviceInterfaceDetailW")
	pSetupDiOpenDeviceInterfaceRegKey = setupapiDLL.NewProc("SetupDiOpenDeviceInterfaceRegKey")
	pSetupDiDestroyDeviceInfoList     = setupapiDLL.NewProc("SetupDiDestroyDeviceInfoList")
)

const (
	printerEnumLocal       = 0x2
	printerEnumConnections = 0x4
	printerAllAccess       = 0x000F000C

	digcfPresent         = 0x2
	digcfDeviceInterface = 0x10
)

// guidDevInterfaceUSBPrint is GUID_DEVINTERFACE_USBPRINT: one interface per
// USB printer, whose "Port Name" is the USB00x port it prints through.
var guidDevInterfaceUSBPrint = windows.GUID{
	Data1: 0x28d78fad, Data2: 0x5a12, Data3: 0x11d1,
	Data4: [8]byte{0xae, 0x5b, 0x00, 0x00, 0xf8, 0x03, 0xa8, 0xc2},
}

type spDeviceInterfaceData struct {
	size     uint32
	class    windows.GUID
	flags    uint32
	reserved uintptr
}

type printerDefaults struct {
	datatype, devMode uintptr
	desiredAccess     uint32
}

func printerReady(info *printerInfo2) bool {
	return info.attributes&printerAttributeWorkOffline == 0 && info.status&printerStatusDown == 0
}

func utf16At(p uintptr) string {
	if p == 0 {
		return ""
	}
	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(p)))
}

// listPrinterQueues lists the printers installed on this computer.
func listPrinterQueues() ([]PrinterChoice, error) {
	flags := uintptr(printerEnumLocal | printerEnumConnections)
	var needed, count uint32
	pEnumPrintersW.Call(flags, 0, 2, 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&count)))
	if needed == 0 {
		return []PrinterChoice{}, nil
	}
	buf := make([]uintptr, (uintptr(needed)+unsafe.Sizeof(uintptr(0))-1)/unsafe.Sizeof(uintptr(0)))
	r, _, err := pEnumPrintersW.Call(flags, 0, 2, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed),
		uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&count)))
	if r == 0 {
		return nil, fmt.Errorf("list printers: %w", err)
	}
	infos := unsafe.Slice((*printerInfo2)(unsafe.Pointer(&buf[0])), count)
	printers := make([]PrinterChoice, 0, count)
	for i := range infos {
		printers = append(printers, PrinterChoice{
			Name:  utf16At(infos[i].printerName),
			Port:  utf16At(infos[i].portName),
			Ready: printerReady(&infos[i]),
		})
	}
	return printers, nil
}

// liveUSBPrinterPorts lists the USB printer ports with a printer plugged in.
func liveUSBPrinterPorts() ([]PrinterPort, error) {
	set, _, err := pSetupDiGetClassDevsW.Call(uintptr(unsafe.Pointer(&guidDevInterfaceUSBPrint)), 0, 0,
		digcfPresent|digcfDeviceInterface)
	if windows.Handle(set) == windows.InvalidHandle {
		return nil, err
	}
	defer pSetupDiDestroyDeviceInfoList.Call(set)

	var ports []PrinterPort
	for i := uintptr(0); ; i++ {
		data := spDeviceInterfaceData{}
		data.size = uint32(unsafe.Sizeof(data))
		if r, _, _ := pSetupDiEnumDeviceInterfaces.Call(set, 0, uintptr(unsafe.Pointer(&guidDevInterfaceUSBPrint)),
			i, uintptr(unsafe.Pointer(&data))); r == 0 {
			break
		}
		key, _, _ := pSetupDiOpenDeviceInterfaceRegKey.Call(set, uintptr(unsafe.Pointer(&data)), 0, registry.QUERY_VALUE)
		if windows.Handle(key) == windows.InvalidHandle {
			continue
		}
		port, _, _ := registry.Key(key).GetStringValue("Port Name")
		registry.Key(key).Close()
		if port == "" {
			continue
		}
		ports = append(ports, PrinterPort{Port: port, Device: deviceIDFromPath(interfacePath(set, &data))})
	}
	return ports, nil
}

// interfacePath is a device interface's path, which names its USB IDs.
func interfacePath(set uintptr, data *spDeviceInterfaceData) string {
	// SP_DEVICE_INTERFACE_DETAIL_DATA_W: a DWORD cbSize, then the path.
	buf := make([]uint64, 256)
	cbSize := uint32(8) // the struct's size on 64-bit: DWORD + WCHAR, padded
	if unsafe.Sizeof(uintptr(0)) == 4 {
		cbSize = 6
	}
	*(*uint32)(unsafe.Pointer(&buf[0])) = cbSize
	if r, _, _ := pSetupDiGetDeviceInterfaceDetailW.Call(set, uintptr(unsafe.Pointer(data)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)*8), 0, 0); r == 0 {
		return ""
	}
	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + 4)))
}

// setPrinterPort points the printer at port. Changing a printer needs the
// "Manage this printer" right; if GB Cloud doesn't have it, Windows asks
// for an administrator's approval (UAC) to make the change.
func setPrinterPort(name, port string) error {
	err := setPrinterPortDirect(name, port)
	if err == nil {
		return nil
	}
	if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return err
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	command := fmt.Sprintf("Set-Printer -Name %s -PortName %s", quote(name), quote(port))
	owner, approved := beginApprovalPrompt()
	err = windows.ShellExecute(windows.Handle(owner), windows.StringToUTF16Ptr("runas"),
		windows.StringToUTF16Ptr("powershell.exe"),
		windows.StringToUTF16Ptr("-NoProfile -WindowStyle Hidden -Command "+command), nil, windows.SW_HIDE)
	approved()
	if err != nil {
		return fmt.Errorf("change the printer's port: %w", err)
	}
	for deadline := time.Now().Add(60 * time.Second); time.Now().Before(deadline); {
		time.Sleep(time.Second)
		if current, err := printerPort(name); err == nil && strings.EqualFold(current, port) {
			return nil
		}
	}
	return fmt.Errorf("the printer is still on its old port; was the Windows prompt approved?")
}

func openPrinterFor(name string, access uint32) (uintptr, error) {
	defaults := printerDefaults{desiredAccess: access}
	var handle uintptr
	defaultsPtr := uintptr(0)
	if access != 0 {
		defaultsPtr = uintptr(unsafe.Pointer(&defaults))
	}
	if r, _, err := pOpenPrinterW.Call(uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(name))),
		uintptr(unsafe.Pointer(&handle)), defaultsPtr); r == 0 {
		return 0, err
	}
	return handle, nil
}

// getPrinterInfo2 reads the printer's PRINTER_INFO_2; buf keeps it alive.
func getPrinterInfo2(handle uintptr) (*printerInfo2, []uintptr, error) {
	var needed uint32
	pGetPrinterW.Call(handle, 2, 0, 0, uintptr(unsafe.Pointer(&needed)))
	if needed < uint32(unsafe.Sizeof(printerInfo2{})) {
		return nil, nil, fmt.Errorf("read printer")
	}
	buf := make([]uintptr, (uintptr(needed)+unsafe.Sizeof(uintptr(0))-1)/unsafe.Sizeof(uintptr(0)))
	if r, _, err := pGetPrinterW.Call(handle, 2, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed),
		uintptr(unsafe.Pointer(&needed))); r == 0 {
		return nil, nil, fmt.Errorf("read printer: %w", err)
	}
	return (*printerInfo2)(unsafe.Pointer(&buf[0])), buf, nil
}

func printerPort(name string) (string, error) {
	handle, err := openPrinterFor(name, 0)
	if err != nil {
		return "", err
	}
	defer pClosePrinterW.Call(handle)
	info, _, err := getPrinterInfo2(handle)
	if err != nil {
		return "", err
	}
	return utf16At(info.portName), nil
}

func setPrinterPortDirect(name, port string) error {
	handle, err := openPrinterFor(name, printerAllAccess)
	if err != nil {
		return err
	}
	defer pClosePrinterW.Call(handle)
	info, buf, err := getPrinterInfo2(handle)
	if err != nil {
		return err
	}
	portPtr := windows.StringToUTF16Ptr(port)
	info.portName = uintptr(unsafe.Pointer(portPtr))
	// Leave the printer's permissions as they are.
	info.securityDescriptor = 0
	r, _, err := pSetPrinterW.Call(handle, 2, uintptr(unsafe.Pointer(&buf[0])), 0)
	if r == 0 {
		return fmt.Errorf("change the printer's port: %w", err)
	}
	runtime.KeepAlive(portPtr)
	return nil
}
