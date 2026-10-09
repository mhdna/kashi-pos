//go:build !windows

package main

func configureWebView() error {
	return nil
}

func webviewUserDataPath() string {
	return ""
}
