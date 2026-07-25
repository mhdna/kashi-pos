//go:build !windows

package main

import (
	"fmt"
	"os/exec"
)

func printPDFToPrinter(pdfPath, printerName string) error {
	cmd := exec.Command("lp", "-d", printerName, pdfPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("print failed: %v — %s", err, string(out))
	}
	return nil
}
