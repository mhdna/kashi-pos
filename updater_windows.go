//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

const updateInstallSupported = true

// startDetached starts a process that outlives this one, with no console
// window.
func startDetached(path string, args ...string) error {
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func defaultUpdateProcesses() updateProcesses {
	startPID := func(path string) (int, error) {
		cmd := exec.Command(path)
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
		if err := cmd.Start(); err != nil {
			return 0, err
		}
		pid := cmd.Process.Pid
		_ = cmd.Process.Release()
		return pid, nil
	}
	return updateProcesses{
		waitExit: func(pid int, timeout time.Duration) bool {
			h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
			if err != nil {
				return true // already gone
			}
			defer windows.CloseHandle(h)
			ev, err := windows.WaitForSingleObject(h, uint32(timeout.Milliseconds()))
			return err == nil && ev == windows.WAIT_OBJECT_0
		},
		start: func(path string) error {
			_, err := startPID(path)
			return err
		},
		startPID: startPID,
		alive: func(pid int) bool {
			h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
			if err != nil {
				return false
			}
			defer windows.CloseHandle(h)
			ev, _ := windows.WaitForSingleObject(h, 0)
			return ev == uint32(windows.WAIT_TIMEOUT)
		},
		kill: func(pid int) {
			if p, err := os.FindProcess(pid); err == nil {
				_ = p.Kill()
			}
		},
	}
}
