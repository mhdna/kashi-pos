//go:build !windows

package main

// Taskbar auto-hide is a Windows shell setting; see taskbar_windows.go. There
// is nothing to change on other platforms.

func autoHideTaskbar() {}

func restoreTaskbar() {}
