//go:build windows

package main

import (
	"os/exec"
	"strings"
	"syscall"
)

func listPrinters() ([]string, error) {
	// HideWindow prevents Windows from creating a visible console for the
	// short-lived PowerShell child process used to query the print spooler.
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", "Get-Printer | Select-Object -ExpandProperty Name")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return []string{}, nil
	}
	return parsePrinterList(string(out)), nil
}

func parsePrinterList(output string) []string {
	var result []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	if result == nil {
		return []string{}
	}
	return result
}
