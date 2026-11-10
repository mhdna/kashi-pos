//go:build !windows

package main

// hidePath is a no-op off Windows: the leading dot already hides the folder.
func hidePath(string) {}
