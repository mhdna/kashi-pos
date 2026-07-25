//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func printReceiptToPrinter(data printReceiptData) error {
	pdf := buildReceiptPDF(data.Items, data.StoreName, data.Branch, data.Phone, data.Salesperson, data.ReceiptNo, data.GrandTotalCents, data.Payments, data.ExchangeRate, data.RoundingMode, data.ReceiptWidth, data.ReceiptHeight)
	tmpFile, err := os.CreateTemp("", "kashi-receipt-*.pdf")
	if err != nil {
		return err
	}
	tmpPath := filepath.Clean(tmpFile.Name())
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := pdf.OutputFileAndClose(tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	defer os.Remove(tmpPath)

	cmd := exec.Command("lp", "-d", data.PrinterName, tmpPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("print failed: %v — %s", err, string(out))
	}
	return nil
}
