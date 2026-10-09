//go:build windows

package main

import (
	"os"
	"path/filepath"
)

func configureWebView() error {
	return os.Setenv(
		"WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS",
		"--disable-smooth-scrolling --disable-gpu-rasterization --disable-gpu-vsync",
	)
}

// webviewUserDataPath picks WebView2's profile folder instead of leaving it to
// the default, %APPDATA%\Kashi-Cloud.exe. That default is the folder behind
// "Microsoft Edge can't read and write to its data directory": once an
// elevated run (e.g. the installer's "Run Kashi Cloud" checkbox, which inherited
// the installer's admin token) has created it, a normal run can be refused
// access, and the app never gets past the error box.
//
// %LOCALAPPDATA% rather than roaming because a browser profile is per-machine
// cache, not something to roam. The folder is probed with a real write before
// it is handed over; if that fails, a per-user folder under the temp dir is
// used so the till still starts -- losing the WebView2 cache is harmless, the
// app's own data lives elsewhere.
func webviewUserDataPath() string {
	var candidates []string
	if dir, err := os.UserCacheDir(); err == nil { // %LOCALAPPDATA%
		candidates = append(candidates, filepath.Join(dir, "Kashi Cloud", "WebView2"))
	}
	candidates = append(candidates, filepath.Join(os.TempDir(), "Kashi Cloud WebView2"))

	for _, dir := range candidates {
		if dirWritable(dir) {
			return dir
		}
	}
	return "" // WebView2's own default, as before
}

func dirWritable(dir string) bool {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	f, err := os.CreateTemp(dir, ".probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}
