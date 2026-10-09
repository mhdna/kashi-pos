package main

import "fmt"

// soundsEnabledFlag gates every sound the screen plays -- scan/checkout
// chimes, error and warning tones, keypad clicks (frontend/src/lib/sound.ts
// is the one place they go through). Off until someone turns it on, here or
// from kashi's branch settings.
const soundsEnabledFlag = "sounds_enabled"

// soundsEnabled reports whether the till plays sounds, defaulting to false
// when the flag has never been set.
func (a *App) soundsEnabled() bool {
	if v, err := a.store.GetFeatureFlag(a.ctx, soundsEnabledFlag); err == nil {
		return v != 0
	}
	return false
}

// GetSoundsEnabled is soundsEnabled for the screen, which re-reads it on
// every settings_changed.
func (a *App) GetSoundsEnabled() bool {
	if a.store == nil {
		return false
	}
	return a.soundsEnabled()
}

// SetSoundsEnabled turns the till's sounds on or off. Called both by the
// local Settings page and by an update_settings command from kashi; emits
// "settings_changed" so a running screen picks it up at once.
func (a *App) SetSoundsEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := a.setFeatureFlagBool(a.ctx, soundsEnabledFlag, enabled); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}
