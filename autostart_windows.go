//go:build windows

package main

// Start GB Cloud when the till's user signs in to Windows.
//
// The app writes its own entry under the current user's
// Software\Microsoft\Windows\CurrentVersion\Run key on every launch, rather
// than the installer doing it once, so tills that only ever take updates
// from kashi (updater.go swaps the exe, the installer never runs) get it too,
// and a moved install fixes itself. Turning it off in Task Manager's Startup
// tab still works: Windows keeps that choice in a separate StartupApproved
// key, which this never touches. The uninstaller deletes the entry.

import (
	"log"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	autostartKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	// autostartValueName matches INFO_PRODUCTNAME, which the uninstaller
	// (build/windows/installer/project.nsi) deletes the entry by.
	autostartValueName = "GB Cloud"
)

// registerAutostart points the Run entry at the running executable.
func registerAutostart() {
	if isDevMode {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		log.Printf("autostart: find executable: %v", err)
		return
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	command := `"` + exe + `"`

	key, _, err := registry.CreateKey(registry.CURRENT_USER, autostartKeyPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		log.Printf("autostart: open Run key: %v", err)
		return
	}
	defer key.Close()
	if current, _, err := key.GetStringValue(autostartValueName); err == nil && current == command {
		return
	}
	if err := key.SetStringValue(autostartValueName, command); err != nil {
		log.Printf("autostart: write Run entry: %v", err)
	}
}
