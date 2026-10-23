//go:build !windows

package main

// Starting at sign-in is set up through the Windows Run key; see
// autostart_windows.go. Nothing to do on other platforms.

func registerAutostart() {}
