//go:build windows

package license

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// DPAPI with CRYPTPROTECT_LOCAL_MACHINE: any Windows user on this PC can open
// it (the till may run as a different account in cash-machine mode), no other
// PC can.
const (
	cryptprotectUIForbidden  = 0x1
	cryptprotectLocalMachine = 0x4
)

var dpapiEntropy = []byte("kashi-pos-license")

func blob(b []byte) *windows.DataBlob {
	if len(b) == 0 {
		return &windows.DataBlob{}
	}
	return &windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
}

func platformSeal(b []byte) ([]byte, error) {
	var out windows.DataBlob
	if err := windows.CryptProtectData(blob(b), nil, blob(dpapiEntropy), 0, nil, cryptprotectUIForbidden|cryptprotectLocalMachine, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}

func platformUnseal(b []byte) ([]byte, error) {
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(blob(b), nil, blob(dpapiEntropy), 0, nil, cryptprotectUIForbidden, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}
