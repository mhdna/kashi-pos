//go:build !windows

package main

import "errors"

// Kiosk mode keeps Windows out of sight while the till runs; see
// kiosk_windows.go. There is nothing to hide on other platforms.

func enterKiosk() {}

func leaveKiosk() {}

func startPowerCommand(bool) error {
	return errors.New("shutting the computer down from the app only works on Windows")
}

func startRelaunch(string) error {
	return errors.New("restarting the app from itself only works on Windows")
}
