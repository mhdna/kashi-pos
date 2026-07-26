//go:build windows

package main

import "os"

func configureWebView() error {
	return os.Setenv(
		"WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS",
		"--disable-smooth-scrolling --disable-gpu-rasterization --disable-gpu-vsync",
	)
}
