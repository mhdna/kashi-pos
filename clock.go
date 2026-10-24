package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	_ "time/tzdata" // Windows has no zoneinfo of its own for LoadLocation

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// The till keeps time the way kashi does: instants are stored in UTC, and
// days and hours are read in the branch's timezone (kashi's
// branches.timezone), not whatever zone the PC happens to be set to.
// Setting time.Local covers SQLite's 'localtime' too: modernc's libc
// answers localtime() from Go's time.Local.
//
// What the till can't fix itself is a wrong PC clock, or the webview (which
// formats with the PC's own zone), so those raise a warning instead.

const (
	// clockCheckInterval is how often runPullTick asks kashi for its clock.
	clockCheckInterval = 10 * time.Minute
	// clockOffAfter is how far the PC's clock may drift from kashi's before
	// the cashier is told.
	clockOffAfter = 2 * time.Minute
)

// osZone is the PC's own zone, kept before useZone replaces time.Local.
var osZone = time.Local

// ClockStatus is what the clock warning shows.
type ClockStatus struct {
	// Checked is false until kashi has answered once.
	Checked bool `json:"checked"`
	// Zone is the branch's zone, the one the till keeps its days in.
	Zone string `json:"zone"`
	// PCZone is the PC's own UTC offset, e.g. "UTC+02:00".
	PCZone       string `json:"pcZone"`
	ZoneMismatch bool   `json:"zoneMismatch"`
	// DriftSeconds is the PC's clock minus kashi's: positive when ahead.
	DriftSeconds int64 `json:"driftSeconds"`
	ClockOff     bool  `json:"clockOff"`
}

// useZone makes name the till's local zone. It runs once at startup and
// again only if kashi reports a different zone, which is about never.
func useZone(name string) bool {
	loc, err := time.LoadLocation(name)
	if err != nil {
		log.Printf("clock: unknown timezone %q: %v", name, err)
		return false
	}
	time.Local = loc
	return true
}

// applySavedZone switches to the zone kashi last reported, before anything
// reads the clock. Until kashi has answered once, the PC's zone stays.
func (a *App) applySavedZone() {
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		log.Printf("clock: load settings: %v", err)
		return
	}
	if settings.Timezone.Valid && settings.Timezone.String != "" {
		useZone(settings.Timezone.String)
	}
}

// branchClock is the clock half of kashi's /branch/health answer. Both
// fields are empty from a kashi older than the clock check.
type branchClock struct {
	Timezone   string    `json:"timezone"`
	ServerTime time.Time `json:"server_time"`
}

// Clock reads kashi's clock and the branch's zone from /branch/health.
func (c *centralClient) Clock(ctx context.Context) (branchClock, error) {
	var out branchClock
	if !c.enabled() {
		return out, errCentralSyncDisabled
	}
	req, err := c.newRequest(ctx, http.MethodGet, "/branch/health", nil)
	if err != nil {
		return out, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, responseError(http.MethodGet, "/branch/health", resp)
	}
	err = json.NewDecoder(resp.Body).Decode(&out)
	return out, err
}

// checkClock compares the PC's clock with kashi's, adopts the branch's zone
// if it changed, and tells the UI.
func (a *App) checkClock(ctx context.Context, client *centralClient) {
	sent := time.Now()
	clock, err := client.Clock(ctx)
	received := time.Now()
	if err != nil {
		log.Printf("clock: %v", err)
		return
	}
	if clock.ServerTime.IsZero() {
		return // kashi predates the clock check
	}
	// kashi read its clock somewhere in the round trip; the middle is the
	// best guess, and good to well under clockOffAfter.
	drift := sent.Add(received.Sub(sent) / 2).Sub(clock.ServerTime)

	if clock.Timezone != "" {
		settings, err := a.store.GetSettings(ctx)
		if err == nil && settings.Timezone.String != clock.Timezone && useZone(clock.Timezone) {
			if err := a.store.SetSettingsTimezone(ctx, sql.NullString{String: clock.Timezone, Valid: true}); err != nil {
				log.Printf("clock: save timezone: %v", err)
			}
		}
	}

	status := clockStatus(clock.Timezone, drift, received)
	a.clockMu.Lock()
	a.clock = status
	a.clockMu.Unlock()
	runtime.EventsEmit(a.ctx, "clock_status_changed", status)
}

func clockStatus(zone string, drift time.Duration, now time.Time) ClockStatus {
	_, pcOffset := now.In(osZone).Zone()
	status := ClockStatus{
		Checked:      true,
		Zone:         zone,
		PCZone:       utcOffset(pcOffset),
		DriftSeconds: int64(drift.Round(time.Second) / time.Second),
		ClockOff:     drift > clockOffAfter || drift < -clockOffAfter,
	}
	if loc, err := time.LoadLocation(zone); zone != "" && err == nil {
		_, branchOffset := now.In(loc).Zone()
		status.ZoneMismatch = pcOffset != branchOffset
	}
	return status
}

// utcOffset spells a zone offset in seconds as "UTC+03:00".
func utcOffset(seconds int) string {
	sign := '+'
	if seconds < 0 {
		sign, seconds = '-', -seconds
	}
	return fmt.Sprintf("UTC%c%02d:%02d", sign, seconds/3600, seconds%3600/60)
}

// GetClockStatus is the Wails binding behind the clock warning.
func (a *App) GetClockStatus() ClockStatus {
	a.clockMu.Lock()
	defer a.clockMu.Unlock()
	return a.clock
}
