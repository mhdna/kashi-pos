//go:build !windows

package main

import (
	"os/exec"
	"strings"
)

func listPrinters() ([]string, error) {
	out, err := exec.Command("lpstat", "-e").Output()
	if err != nil {
		return []string{}, nil
	}
	var result []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	if result == nil {
		return []string{}, nil
	}
	return result, nil
}
