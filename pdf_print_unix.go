//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func printReceiptToPrinter(data printReceiptData) error {
	payload, err := buildESCPOSRasterReceipt(data)
	if err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp("", "kashi-receipt-*.escpos")
	if err != nil {
		return fmt.Errorf("create receipt payload: %w", err)
	}
	tmpPath := filepath.Clean(tmpFile.Name())
	defer os.Remove(tmpPath)
	if _, err := tmpFile.Write(payload); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write receipt payload: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close receipt payload: %w", err)
	}

	cmd := exec.Command("lp", "-d", data.PrinterName, "-o", "raw", tmpPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("print receipt to %q: %v: %s", data.PrinterName, err, output)
	}
	return nil
}
