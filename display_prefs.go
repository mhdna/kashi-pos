package main

import (
	"encoding/json"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Display prefs are how this register's screen looks: quick access rows,
// card text size and lines, animations, the calculator's leading currency,
// the theme. The screen owns them (they apply instantly, from localStorage,
// before any binding call returns), but they're mirrored into
// settings.display_prefs so kashi sees them in the settings report and can
// push new values (see migration 000110).
//
// Two writers, told apart by display_prefs_rev:
//
//	the screen  ReportDisplayPrefs after each local change; rev untouched
//	kashi       applyRemoteDisplayPrefs from update_settings; rev + 1
//
// The screen remembers the last rev it applied and adopts the stored prefs
// whenever this one is newer -- at startup, and on the display_prefs_changed
// event a push emits. Whichever happened last wins.

// displayPrefKeys are the prefs kashi may set, each with a validator. A key
// not listed here is dropped from a push rather than stored, so the screen
// never receives something it doesn't know how to apply.
var displayPrefKeys = map[string]func(json.RawMessage) error{
	"quick_access_rows":         intInRange(0, 8),
	"quick_access_font_scale":   intInRange(70, 200),
	"quick_access_show_code":    isBool,
	"quick_access_show_name":    isBool,
	"quick_access_show_remove":  isBool,
	"animations_enabled":        isBool,
	"product_search_in_place":   isBool,
	"calculator_total_currency": isString,
	"theme":                     oneOf("light", "dark"),
}

func intInRange(min, max int) func(json.RawMessage) error {
	return func(raw json.RawMessage) error {
		var v int
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("must be a whole number")
		}
		if v < min || v > max {
			return fmt.Errorf("must be between %d and %d", min, max)
		}
		return nil
	}
}

func isBool(raw json.RawMessage) error {
	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("must be true or false")
	}
	return nil
}

func isString(raw json.RawMessage) error {
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("must be text")
	}
	return nil
}

func oneOf(values ...string) func(json.RawMessage) error {
	return func(raw json.RawMessage) error {
		var v string
		if err := json.Unmarshal(raw, &v); err == nil {
			for _, allowed := range values {
				if v == allowed {
					return nil
				}
			}
		}
		return fmt.Errorf("must be one of %v", values)
	}
}

// DisplayPrefs is what the screen reads at startup and on each push.
type DisplayPrefs struct {
	Prefs map[string]any `json:"prefs"`
	Rev   int64          `json:"rev"`
}

func (a *App) GetDisplayPrefs() (DisplayPrefs, error) {
	if a.store == nil {
		return DisplayPrefs{}, fmt.Errorf("database not initialized")
	}
	row, err := a.store.GetDisplayPrefs(a.ctx)
	if err != nil {
		return DisplayPrefs{}, err
	}
	prefs := map[string]any{}
	_ = json.Unmarshal([]byte(row.DisplayPrefs), &prefs)
	return DisplayPrefs{Prefs: prefs, Rev: row.DisplayPrefsRev}, nil
}

// ReportDisplayPrefs stores the screen's current prefs after a local change
// so the next settings report carries them. It's a mirror, not a command:
// rev doesn't move, so it never re-applies anything on screen.
func (a *App) ReportDisplayPrefs(prefsJSON string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	var incoming map[string]json.RawMessage
	if err := json.Unmarshal([]byte(prefsJSON), &incoming); err != nil {
		return fmt.Errorf("decode display prefs: %w", err)
	}
	merged, err := a.mergeDisplayPrefs(incoming)
	if err != nil {
		return err
	}
	return a.store.SetLocalDisplayPrefs(a.ctx, merged)
}

// applyRemoteDisplayPrefs is kashi's push (update_settings.display_prefs):
// every key is validated first, so one bad value fails the command instead
// of half-applying it. A theme travels on to settings.theme too, since that
// is where the app reads it at startup.
func (a *App) applyRemoteDisplayPrefs(raw json.RawMessage) error {
	var incoming map[string]json.RawMessage
	if err := json.Unmarshal(raw, &incoming); err != nil {
		return fmt.Errorf("display_prefs must be an object: %w", err)
	}
	for key, value := range incoming {
		validate, ok := displayPrefKeys[key]
		if !ok {
			return fmt.Errorf("unknown display pref %q", key)
		}
		if err := validate(value); err != nil {
			return fmt.Errorf("display pref %s %w", key, err)
		}
	}
	if len(incoming) == 0 {
		return nil
	}
	merged, err := a.mergeDisplayPrefs(incoming)
	if err != nil {
		return err
	}
	if err := a.store.SetRemoteDisplayPrefs(a.ctx, merged); err != nil {
		return err
	}
	if themeRaw, ok := incoming["theme"]; ok {
		var theme string
		_ = json.Unmarshal(themeRaw, &theme)
		if err := a.SetTheme(theme); err != nil {
			return fmt.Errorf("update theme: %w", err)
		}
	}
	a.emitDisplayPrefsChanged()
	return nil
}

// mergeDisplayPrefs overlays incoming keys on the stored prefs, so a push
// naming only quick_access_rows leaves the rest as the screen last set them.
func (a *App) mergeDisplayPrefs(incoming map[string]json.RawMessage) (string, error) {
	row, err := a.store.GetDisplayPrefs(a.ctx)
	if err != nil {
		return "", err
	}
	current := map[string]json.RawMessage{}
	_ = json.Unmarshal([]byte(row.DisplayPrefs), &current)
	for key, value := range incoming {
		current[key] = value
	}
	out, err := json.Marshal(current)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (a *App) emitDisplayPrefsChanged() {
	select {
	case <-a.ready:
		runtime.EventsEmit(a.ctx, "display_prefs_changed")
	default:
	}
}
