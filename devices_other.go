//go:build !windows

package main

// Device status reads Windows' raw input and print spooler; see
// devices_windows.go. Elsewhere every device shows as unknown.

func startInputDeviceListener() {}

// presentInputDevices returns nil: presence can't be told here.
func presentInputDevices() (keyboards, mice map[string]bool) { return nil, nil }

func printerStatus(string) string { return deviceUnknown }
