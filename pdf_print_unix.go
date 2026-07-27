//go:build !windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "lp", "-d", data.PrinterName, "-o", "raw", tmpPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("print receipt to %q timed out after 15 seconds", data.PrinterName)
		}
		return fmt.Errorf("print receipt to %q: %v: %s", data.PrinterName, err, output)
	}
	return nil
}
