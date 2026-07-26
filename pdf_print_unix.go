//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func printReceiptToPrinter(data printReceiptData) error {
	tmpFile, err := os.CreateTemp("", "kashi-receipt-*.bin")
	if err != nil {
		return fmt.Errorf("create receipt payload: %w", err)
	}
	tmpPath := filepath.Clean(tmpFile.Name())
	if _, err := tmpFile.Write(buildESCPosReceipt(data)); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write receipt payload: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close receipt payload: %w", err)
	}
	defer os.Remove(tmpPath)

	cmd := exec.Command("lp", "-d", data.PrinterName, "-o", "raw", tmpPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("print failed: %v — %s", err, string(out))
	}
	return nil
}
