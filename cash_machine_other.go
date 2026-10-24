//go:build !windows

package main

import "errors"

// Cash machine mode trims Windows; see cash_machine.go. Nothing to do on
// other platforms.

const cashMachineSupported = false

func startCashMachine() {}

func stopCashMachine() {}

func setUpCashMachineForUser() []CashMachineStep { return nil }

func setUpCashMachineAsAdmin() ([]CashMachineStep, error) {
	return nil, errors.New("only on Windows")
}

func runCashMachineSetup(string) int { return 1 }
