package main

import "time"

// Stuck modifier keys. AnyDesk from a Mac sends ⌘ as the Windows key, and
// when the Mac keeps a key's release to itself (⌘+Tab away from the AnyDesk
// window, say) the till never hears it come back up. Windows goes on holding
// Win, so every key typed or scanned on the till becomes a Win shortcut (1
// opens the first taskbar program) until someone presses Win on the till,
// even after AnyDesk disconnects. The device listener (devices_windows.go)
// lets go of a modifier Windows holds that no keyboard on the till holds.

type modifierKey int

const (
	modWin modifierKey = iota
	modCtrl
	modAlt
	modShift
	modifierCount
)

// stuckModifierAfter is how long a modifier is left held, by nothing
// plugged into the till, before it's let go. Long enough for a remote
// Ctrl+C; no one holds a key that long at the till.
const stuckModifierAfter = 3 * time.Second

// modifierWatch is what the device listener knows about the modifiers.
type modifierWatch struct {
	physical [modifierCount]bool      // held on a keyboard or scanner on the till
	since    [modifierCount]time.Time // when it was first seen held by nothing on the till
}

// pressed records a modifier going down or up on a device on the till.
func (w *modifierWatch) pressed(m modifierKey, down bool) {
	w.physical[m] = down
}

// check is called every second with whether Windows holds m, and reports
// whether to let go of it.
func (w *modifierWatch) check(m modifierKey, held bool, now time.Time) bool {
	if !held {
		// Also forgets a key-up the listener missed.
		w.physical[m] = false
	}
	if !held || w.physical[m] {
		w.since[m] = time.Time{}
		return false
	}
	if w.since[m].IsZero() {
		w.since[m] = now
	}
	return now.Sub(w.since[m]) >= stuckModifierAfter
}
