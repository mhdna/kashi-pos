//go:build !windows

package main

// The instant-launch splash is a native Win32 window shown before the WebView2
// runtime has booted; there is no equivalent on other platforms, so these are
// no-ops. See splash_windows.go for the real implementation.

func showSplash() {}

func hideSplash() {}
