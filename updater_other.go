//go:build !windows

package main

import (
	"errors"
	"time"
)

// Installing swaps a Windows executable; other platforms (development
// machines) only ever check.
const updateInstallSupported = false

func startDetached(string, ...string) error {
	return errors.New("updates install only on Windows")
}

func defaultUpdateProcesses() updateProcesses {
	unsupported := errors.New("updates install only on Windows")
	return updateProcesses{
		waitExit: func(int, time.Duration) bool { return true },
		start:    func(string) error { return unsupported },
		startPID: func(string) (int, error) { return 0, unsupported },
		alive:    func(int) bool { return false },
		kill:     func(int) {},
	}
}
