//go:build windows

package main

import "syscall"

// hidePath sets the Hidden attribute, so the photo cache folder doesn't
// show in Explorer by default. Best effort.
func hidePath(path string) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	attrs, err := syscall.GetFileAttributes(p)
	if err != nil {
		return
	}
	_ = syscall.SetFileAttributes(p, attrs|syscall.FILE_ATTRIBUTE_HIDDEN)
}
