package main

import (
	"errors"
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
