//go:build windows

package main

import (
	"log"
	"os"
	"path/filepath"
)

// configureWebView sets the browser arguments WebView2 starts with: the
// defaults, or GB_CLOUD_WEBVIEW_ARGS for comparing flags on one till (see
// webview_args.go and docs/dev/windows-till.md).
func configureWebView() error {
	args, set := chooseWebViewArgs(os.LookupEnv)
	if !set {
		log.Printf("webview2: using the preset WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS")
		return nil
	}
	log.Printf("webview2: browser arguments %q", args)
	return os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", args)
}

// webviewUserDataPath picks WebView2's profile folder instead of leaving it to
// the default, %APPDATA%\GB-Cloud.exe. That default is the folder behind
// "Microsoft Edge can't read and write to its data directory": once an
// elevated run (e.g. the installer's "Run GB Cloud" checkbox, which inherited
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
		candidates = append(candidates, filepath.Join(dir, "GB Cloud", "WebView2"))
	}
	candidates = append(candidates, filepath.Join(os.TempDir(), "GB Cloud WebView2"))

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
