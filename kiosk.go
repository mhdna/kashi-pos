package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// kioskKeepShellOff is set when the app closes for a shutdown, a restart or
// an update, so leaveKiosk doesn't bring Windows' taskbar back for the few
// seconds before the computer goes down or the new version starts.
var kioskKeepShellOff atomic.Bool

// ShutDownComputer turns the till off, from the Quit dialog.
func (a *App) ShutDownComputer() error { return a.powerOff(false) }

// RestartComputer restarts the till, from the Quit dialog.
func (a *App) RestartComputer() error { return a.powerOff(true) }

func (a *App) powerOff(restart bool) error {
	if isDevMode {
		return errors.New("not from a development build")
	}
	if err := startPowerCommand(restart); err != nil {
		return err
	}
	kioskKeepShellOff.Store(true)
	go func() {
		// Let the frontend paint before the window goes.
		time.Sleep(300 * time.Millisecond)
		wailsruntime.Quit(a.ctx)
	}()
	return nil
}

// RestartApp closes this app and starts it again, from the Quit dialog. The
// new copy is started by a hidden helper after a short wait, not by this
// process: the new one must not begin while this one still holds its ports
// and the single-instance lock.
func (a *App) RestartApp() error {
	if isDevMode {
		return errors.New("not from a development build")
	}
	exe, err := currentExecutable()
	if err != nil {
		return err
	}
	if err := startRelaunch(exe); err != nil {
		return err
	}
	kioskKeepShellOff.Store(true)
	go func() {
		// Let the frontend paint before the window goes.
		time.Sleep(300 * time.Millisecond)
		wailsruntime.Quit(a.ctx)
	}()
	return nil
}

// relaunchScript is the cmd.exe line that waits about two seconds (three
// pings to localhost, like the power helper: a timer command would need a
// console) and then starts exe. The quotes keep a path with spaces whole;
// `start "" ...` is cmd's way to start a program without waiting for it.
func relaunchScript(system32, exe string) string {
	return fmt.Sprintf(`"%s" -n 3 127.0.0.1 >nul & start "" "%s"`,
		filepath.Join(system32, "PING.EXE"), exe)
}
