package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Picking up where the cashier was after a forced update restarts the till.
//
// Before it quits, the till writes updates/resume.json: who was signed in
// (the role only, never a PIN or password), which screen they were on, and
// which versions the restart is between. The first launch reads it once,
// deletes it, and, if it is fresh and between those versions, signs the same
// role back in and returns the screen to go to. The open sale needs nothing
// here: the sale screen already keeps its draft and restores it when it
// opens (home.vue restoreDraft).

const (
	// A restart takes seconds; a file older than this is somebody else's day.
	resumeMaxAge = 10 * time.Minute
	// How long the till waits for the screen to say where it is.
	resumeRouteWait = time.Second
)

type resumeFile struct {
	Role        string    `json:"role"`
	Route       string    `json:"route"`
	FromVersion string    `json:"from_version"`
	ToVersion   string    `json:"to_version"`
	SavedAt     time.Time `json:"saved_at"`
}

func resumeFilePath() (string, error) {
	dir, err := updatesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "resume.json"), nil
}

// resumeUsable says whether f may sign someone back in on this launch: a
// known role, saved recently, by one of the two versions this launch is
// between (the new one, or the old one after a rollback).
func resumeUsable(f resumeFile, version string, now time.Time) bool {
	if f.Role != "cashier" && f.Role != "admin" {
		return false
	}
	if version == "" || (version != f.ToVersion && version != f.FromVersion) {
		return false
	}
	age := now.Sub(f.SavedAt)
	return age >= -time.Minute && age <= resumeMaxAge
}

func writeResumeFile(path string, f resumeFile) error {
	raw, err := json.Marshal(f)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(path, raw)
}

// takeResumeFile reads and deletes the file: it works once.
func takeResumeFile(path string) (resumeFile, bool) {
	var f resumeFile
	raw, err := os.ReadFile(path)
	if err != nil {
		return f, false
	}
	_ = os.Remove(path)
	if json.Unmarshal(raw, &f) != nil {
		return resumeFile{}, false
	}
	return f, true
}

// SaveResumeRoute is the screen's answer to "app_update_save_state": the
// route it is on.
func (a *App) SaveResumeRoute(route string) {
	u := &a.updater
	u.mu.Lock()
	ch := u.routeCh
	u.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- route:
	default:
	}
}

// saveResumeBeforeInstall asks the screen where it is, waits a moment for the
// answer and writes the resume file. A till nobody is signed in on writes
// nothing.
func (a *App) saveResumeBeforeInstall(from, to string) {
	role := a.currentRoleNow()
	if role == "" {
		return
	}
	u := &a.updater
	ch := make(chan string, 1)
	u.mu.Lock()
	u.routeCh = ch
	u.mu.Unlock()
	defer func() {
		u.mu.Lock()
		u.routeCh = nil
		u.mu.Unlock()
	}()
	wailsruntime.EventsEmit(a.ctx, "app_update_save_state")
	route := ""
	select {
	case route = <-ch:
	case <-time.After(resumeRouteWait):
	}
	path, err := resumeFilePath()
	if err != nil {
		return
	}
	_ = writeResumeFile(path, resumeFile{Role: role, Route: route, FromVersion: from, ToVersion: to, SavedAt: time.Now()})
}

func (a *App) currentRoleNow() string {
	a.pinMu.Lock()
	defer a.pinMu.Unlock()
	return a.currentRole
}

// TakeResumeSession runs once when the screen starts: after a forced update
// it signs the same role back in and says which screen to open. Anything
// else (no file, stale, wrong version) returns nothing and the cashier signs
// in as usual.
func (a *App) TakeResumeSession() map[string]any {
	path, err := resumeFilePath()
	if err != nil {
		return map[string]any{}
	}
	f, ok := takeResumeFile(path)
	if !ok || !resumeUsable(f, appVersion, time.Now()) {
		return map[string]any{}
	}
	a.pinMu.Lock()
	a.currentRole = f.Role
	a.pinMu.Unlock()
	return map[string]any{"id": int64(0), "username": f.Role, "role": f.Role, "route": f.Route}
}

// discardResume removes a resume file for a restart that didn't happen.
func discardResume() {
	if path, err := resumeFilePath(); err == nil {
		_ = os.Remove(path)
	}
}
