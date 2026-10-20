package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kashi-pos/internal/updatekey"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Remote updates. kashi tells each till which release to run (its App
// Updates page); the till downloads it in the background, checks it was
// signed with the release key (internal/updatekey), and installs it at a
// safe moment: when the cashier taps "Install now", or by itself while no
// shift is open. Installing hands off to a copy of this program running as a
// helper (runUpdateHelper): it waits for the app to exit, swaps the
// executable, starts the new version and waits for it to confirm it started
// (ConfirmAppStarted). If it doesn't, the helper puts the old executable
// back and starts that -- a bad build can't leave a shop without a till.
// Rolling back by hand (RollBackAppUpdate) is the same hand-off with the
// previous executable, which every install keeps next to the current one.
//
// The database is never touched: an older build runs fine on a database a
// newer one migrated (applyMigrations treats a newer schema as current), as
// long as migrations stay additive.

const (
	updateCheckInterval   = 30 * time.Minute
	updateIdleCheckPeriod = 30 * time.Second
	// How long the new version gets to open its database and show its first
	// screen before the helper rolls it back.
	updateVerifyTimeout = 2 * time.Minute
	updateExitTimeout   = time.Minute
)

// Update states. The same strings go to kashi (appUpdateStates there).
const (
	updateUpToDate    = "up_to_date"
	updateDownloading = "downloading"
	updateReady       = "ready"
	updateInstalling  = "installing"
	updateUpdated     = "updated"
	updateFailed      = "failed"
	updateRolledBack  = "rolled_back"
	updateBlocked     = "blocked"
)

// updatePublicKey is a var so tests can sign with a key of their own.
var updatePublicKey = updatekey.PublicKey

// updaterState is what the updater knows, kept in updates/state.json so a
// download survives a restart and a rollback isn't undone by the next check.
type updaterState struct {
	State     string `json:"state"`
	Detail    string `json:"detail"`
	Progress  int    `json:"progress"`
	ReleaseID int64  `json:"release_id"`
	// Version and Notes describe the release being worked on (the target).
	Version string `json:"version"`
	Notes   string `json:"notes"`
	// StagedPath is the verified download of ReleaseID, ready to install.
	StagedPath string `json:"staged_path"`
	// PreviousVersion is what <exe>.previous runs -- what "Roll back" goes to.
	PreviousVersion string `json:"previous_version"`
	// Force: kashi wants it installed now, shift or not (App Updates' force).
	Force bool `json:"force"`
	// SkipVersion is a version this till rolled back from (by hand or
	// because it didn't start); it isn't installed again until kashi points
	// the till somewhere else.
	SkipVersion string `json:"skip_version"`
}

// updateHandoff is an install in flight, shared through updates/handoff.json
// between the app that starts it, the helper and the version it starts.
type updateHandoff struct {
	Mode        string `json:"mode"` // "update" or "rollback"
	ReleaseID   int64  `json:"release_id"`
	FromVersion string `json:"from_version"`
	ToVersion   string `json:"to_version"`
	Notes       string `json:"notes"`
	ExePath     string `json:"exe_path"`
	ParentPID   int    `json:"parent_pid"`
	// Phase: "waiting" for the app to exit, "verifying" the new version,
	// "healthy" once it confirmed, "done" after the helper finalized, or
	// "rolled_back" when the old version had to be put back.
	Phase     string    `json:"phase"`
	Detail    string    `json:"detail"`
	StartedAt time.Time `json:"started_at"`
}

// The files next to the executable during and after an install.
func newExePath(exe string) string      { return exe + ".new" }
func previousExePath(exe string) string { return exe + ".previous" }
func pendingOldExePath(exe string) string {
	return exe + ".old"
}

type appUpdater struct {
	mu       sync.Mutex
	state    updaterState
	loaded   bool
	kick     chan struct{}
	stop     chan struct{}
	prompted bool
	// started is the result of this launch's hand-off, if it was one --
	// what ConfirmAppStarted tells the frontend.
	started   map[string]any
	confirmed bool
}

func updatesDir() (string, error) {
	dir, err := branchRegistryDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "updates"), nil
}

// updatesEnabled is false for `wails dev` builds and builds without a
// release key: they never install anything.
func updatesEnabled() bool {
	return appVersion != "dev" && updatePublicKey != ""
}

func (a *App) loadUpdaterState() {
	u := &a.updater
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.loaded {
		return
	}
	u.loaded = true
	dir, err := updatesDir()
	if err != nil {
		return
	}
	if raw, err := os.ReadFile(filepath.Join(dir, "state.json")); err == nil {
		_ = json.Unmarshal(raw, &u.state)
	}
}

// setUpdateState changes the state, saves it, and tells the frontend.
// Callers report to kashi themselves (reportUpdateStatus), since that needs
// the network.
func (a *App) setUpdateState(change func(s *updaterState)) updaterState {
	u := &a.updater
	u.mu.Lock()
	change(&u.state)
	snapshot := u.state
	u.mu.Unlock()
	if dir, err := updatesDir(); err == nil {
		if err := os.MkdirAll(dir, 0o755); err == nil {
			if raw, err := json.MarshalIndent(snapshot, "", "  "); err == nil {
				_ = writeFileAtomic(filepath.Join(dir, "state.json"), raw)
			}
		}
	}
	a.emitUpdateState()
	return snapshot
}

func (a *App) updateStateSnapshot() updaterState {
	a.updater.mu.Lock()
	defer a.updater.mu.Unlock()
	return a.updater.state
}

func (a *App) emitUpdateState() {
	select {
	case <-a.ready:
		wailsruntime.EventsEmit(a.ctx, "app_update_state", a.GetAppUpdateStatus())
	default:
	}
}

// GetAppUpdateStatus is the Settings card's and the update banner's view.
func (a *App) GetAppUpdateStatus() map[string]any {
	s := a.updateStateSnapshot()
	canRollBack := false
	if exe, err := currentExecutable(); err == nil && s.PreviousVersion != "" {
		_, statErr := os.Stat(previousExePath(exe))
		canRollBack = statErr == nil
	}
	return map[string]any{
		"enabled":         updatesEnabled(),
		"supported":       updateInstallSupported,
		"currentVersion":  appVersion,
		"flavor":          appFlavor,
		"state":           s.State,
		"detail":          s.Detail,
		"progress":        s.Progress,
		"version":         s.Version,
		"notes":           s.Notes,
		"previousVersion": s.PreviousVersion,
		"canRollBack":     canRollBack,
		"force":           s.Force,
	}
}

func (a *App) startUpdater() {
	a.loadUpdaterState()
	u := &a.updater
	kick := make(chan struct{}, 1)
	stop := make(chan struct{})
	u.mu.Lock()
	u.kick, u.stop = kick, stop
	u.mu.Unlock()
	a.backgroundWG.Add(1)
	go func() {
		defer a.backgroundWG.Done()
		select {
		case <-a.ready:
		case <-stop:
			return
		}
		a.finishHandoff()
		cleanupUpdateLeftovers()
		if !updatesEnabled() {
			return
		}
		check := time.NewTicker(updateCheckInterval)
		defer check.Stop()
		idle := time.NewTicker(updateIdleCheckPeriod)
		defer idle.Stop()
		a.runUpdateCheck()
		for {
			select {
			case <-stop:
				return
			case <-check.C:
				a.runUpdateCheck()
			case <-kick:
				a.runUpdateCheck()
			case <-idle.C:
				a.maybePromptIdleInstall()
			}
		}
	}()
}

// stopUpdater may run twice (a branch switch, then shutdown); the second
// call finds nothing to stop.
func (a *App) stopUpdater() {
	u := &a.updater
	u.mu.Lock()
	stop := u.stop
	u.stop, u.kick = nil, nil
	u.mu.Unlock()
	if stop != nil {
		close(stop)
	}
}

// kickUpdater runs a check now -- kashi's "app_update" push, or the
// Settings "Check for updates" button.
func (a *App) kickUpdater() {
	a.updater.mu.Lock()
	kick := a.updater.kick
	a.updater.mu.Unlock()
	select {
	case kick <- struct{}{}:
	default:
	}
}

// CheckForAppUpdate asks kashi now instead of waiting for the next check.
func (a *App) CheckForAppUpdate() map[string]any {
	// A manual check is also how a till retries a version it rolled back
	// from.
	a.setUpdateState(func(s *updaterState) { s.SkipVersion = "" })
	a.kickUpdater()
	return a.GetAppUpdateStatus()
}

func (a *App) runUpdateCheck() {
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Minute)
	defer cancel()
	client := a.syncClient(ctx, "update")
	if client == nil {
		return
	}
	if err := a.checkAndDownload(ctx, client); err != nil && !errors.Is(err, errNoAppUpdateRoute) {
		log.Printf("app update: %v", err)
	}
}

// checkAndDownload brings the till's view in line with its target: up to
// date, or the target downloaded, verified and ready to install.
func (a *App) checkAndDownload(ctx context.Context, client *centralClient) error {
	manifest, err := client.GetAppUpdate(ctx)
	if err != nil {
		return err
	}
	s := a.updateStateSnapshot()

	if manifest == nil || manifest.Version == appVersion {
		keep := s.State == updateUpdated || s.State == updateRolledBack || s.State == updateBlocked
		next := a.setUpdateState(func(st *updaterState) {
			if !keep || manifest == nil {
				st.State, st.Detail = updateUpToDate, ""
			}
			st.Progress, st.StagedPath = 0, ""
			if manifest != nil {
				st.ReleaseID, st.Version, st.Notes = manifest.ID, manifest.Version, manifest.Notes
			}
			// A new target clears a rollback's "don't reinstall".
			if manifest == nil || manifest.Version != st.SkipVersion {
				st.SkipVersion = ""
			}
		})
		removeStagedDownloads("")
		a.reportUpdateStatus(ctx, client, next)
		return nil
	}

	if manifest.Flavor != appFlavor {
		next := a.setUpdateState(func(st *updaterState) {
			st.State, st.ReleaseID, st.Version = updateFailed, manifest.ID, manifest.Version
			st.Detail = fmt.Sprintf("kashi offered the %s build; this till runs the %s build", manifest.Flavor, appFlavor)
		})
		a.reportUpdateStatus(ctx, client, next)
		return nil
	}
	if manifest.Version == s.SkipVersion {
		a.reportUpdateStatus(ctx, client, s)
		return nil
	}
	if s.State == updateReady && s.ReleaseID == manifest.ID && s.StagedPath != "" {
		if _, err := os.Stat(s.StagedPath); err == nil {
			if s.Force != manifest.Force { // forced (or not) after it downloaded
				s = a.setUpdateState(func(st *updaterState) { st.Force = manifest.Force })
			}
			a.reportUpdateStatus(ctx, client, s)
			return nil
		}
	}

	staged, err := a.downloadRelease(ctx, client, manifest)
	if err != nil {
		next := a.setUpdateState(func(st *updaterState) {
			st.State, st.ReleaseID, st.Version, st.Notes = updateFailed, manifest.ID, manifest.Version, manifest.Notes
			st.Detail, st.Progress = "Download failed: "+err.Error(), 0
		})
		a.reportUpdateStatus(ctx, client, next)
		return err
	}
	a.updater.mu.Lock()
	a.updater.prompted = false
	a.updater.mu.Unlock()
	next := a.setUpdateState(func(st *updaterState) {
		st.State, st.Detail, st.Progress = updateReady, "", 100
		st.ReleaseID, st.Version, st.Notes, st.StagedPath = manifest.ID, manifest.Version, manifest.Notes, staged
		st.Force = manifest.Force
	})
	a.reportUpdateStatus(ctx, client, next)
	return nil
}

// downloadRelease fetches the release into updates/<id>/ (resuming a
// partial download) and returns its path once size, checksum and signature
// all check out.
func (a *App) downloadRelease(ctx context.Context, client *centralClient, m *appUpdateManifest) (string, error) {
	dir, err := updatesDir()
	if err != nil {
		return "", err
	}
	releaseDir := filepath.Join(dir, fmt.Sprint(m.ID))
	removeStagedDownloads(releaseDir)
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		return "", err
	}
	final := filepath.Join(releaseDir, "GB-Cloud.exe")
	if verifyRelease(final, m) == nil {
		return final, nil
	}
	part := final + ".part"

	lastPercent := -1
	progress := func(have int64) {
		percent := 0
		if m.SizeBytes > 0 {
			percent = int(min(have*100/m.SizeBytes, 99))
		}
		if percent == lastPercent {
			return
		}
		lastPercent = percent
		next := a.setUpdateState(func(st *updaterState) {
			st.State, st.Detail, st.Progress = updateDownloading, "", percent
			st.ReleaseID, st.Version, st.Notes = m.ID, m.Version, m.Notes
		})
		a.reportUpdateStatus(ctx, client, next)
	}
	progress(0)
	if err := client.DownloadAppUpdate(ctx, m.ID, part, progress); err != nil {
		return "", err
	}
	if err := verifyRelease(part, m); err != nil {
		// A corrupt partial must not be resumed next time.
		os.Remove(part)
		return "", err
	}
	if err := os.Rename(part, final); err != nil {
		return "", err
	}
	return final, nil
}

// verifyRelease checks a downloaded build against its manifest: size, then
// SHA-256, then the release-key signature over that digest.
func verifyRelease(path string, m *appUpdateManifest) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hash := sha256.New()
	n, err := io.Copy(hash, f)
	if err != nil {
		return err
	}
	if m.SizeBytes > 0 && n != m.SizeBytes {
		return fmt.Errorf("downloaded %d bytes, expected %d", n, m.SizeBytes)
	}
	digest := hash.Sum(nil)
	if hex.EncodeToString(digest) != strings.ToLower(m.Sha256) {
		return errors.New("the download is corrupted (checksum mismatch)")
	}
	pub, err := base64.StdEncoding.DecodeString(updatePublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return errors.New("this build has no valid release key")
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil || !ed25519.Verify(pub, digest, sig) {
		return errors.New("the update isn't signed with the release key; refusing to install it")
	}
	return nil
}

// removeStagedDownloads deletes every download but keep (a release dir,
// or "" for all).
func removeStagedDownloads(keep string) {
	dir, err := updatesDir()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() && path != keep {
			os.RemoveAll(path)
		}
	}
}

func (a *App) reportUpdateStatus(ctx context.Context, client *centralClient, s updaterState) {
	if client == nil || s.State == "" {
		return
	}
	reportCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := client.PostAppUpdateStatus(reportCtx, appUpdateStatusReport{
		AppVersion: appVersion,
		AppFlavor:  appFlavor,
		State:      s.State,
		ReleaseID:  s.ReleaseID,
		Detail:     s.Detail,
		Progress:   s.Progress,
	})
	if err != nil && !isNetworkError(err) {
		log.Printf("app update: report status: %v", err)
	}
}

// reportUpdateStatusNow is reportUpdateStatus for callers without a client.
func (a *App) reportUpdateStatusNow(s updaterState) {
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()
	a.reportUpdateStatus(ctx, a.syncClient(ctx, "update"), s)
}

// maybePromptIdleInstall offers to install a ready update once no shift is
// open -- after close shift, or overnight once a shift auto-closes. The
// frontend shows a countdown the cashier can cancel; it prompts again the
// next time the till is idle. A forced update (App Updates' force) is
// prompted for right away, shift or not, with a countdown that can't be
// cancelled.
func (a *App) maybePromptIdleInstall() {
	s := a.updateStateSnapshot()
	u := &a.updater
	_, err := a.store.GetOpenShift(a.ctx)
	idle := errors.Is(err, sql.ErrNoRows)
	u.mu.Lock()
	if !idle && !s.Force {
		u.prompted = false
	}
	prompt := (idle || s.Force) && s.State == updateReady && !u.prompted && updateInstallSupported
	if prompt {
		u.prompted = true
	}
	u.mu.Unlock()
	if prompt {
		wailsruntime.EventsEmit(a.ctx, "app_update_prompt", a.GetAppUpdateStatus())
	}
}

// InstallAppUpdate installs the downloaded update now: the app closes, and
// the new version opens a few seconds later.
func (a *App) InstallAppUpdate() error {
	s := a.updateStateSnapshot()
	if s.State != updateReady || s.StagedPath == "" {
		return errors.New("no update is ready to install")
	}
	return a.beginHandoff(updateHandoff{
		Mode: "update", ReleaseID: s.ReleaseID, FromVersion: appVersion, ToVersion: s.Version, Notes: s.Notes,
	}, s.StagedPath)
}

// RollBackAppUpdate goes back to the version this till ran before its last
// update (or forward again, after a rollback: the two swap places).
func (a *App) RollBackAppUpdate() error {
	s := a.updateStateSnapshot()
	exe, err := currentExecutable()
	if err != nil {
		return err
	}
	previous := previousExePath(exe)
	if _, err := os.Stat(previous); err != nil || s.PreviousVersion == "" {
		return errors.New("there is no previous version on this till")
	}
	return a.beginHandoff(updateHandoff{
		Mode: "rollback", FromVersion: appVersion, ToVersion: s.PreviousVersion,
	}, previous)
}

// beginHandoff stages source as <exe>.new, starts the helper and closes the
// app; the helper does the rest (runUpdateHelper).
func (a *App) beginHandoff(h updateHandoff, source string) error {
	if !updateInstallSupported {
		return errors.New("updates install only on Windows")
	}
	exe, err := currentExecutable()
	if err != nil {
		return err
	}
	if err := copyFile(source, newExePath(exe)); err != nil {
		next := a.setUpdateState(func(st *updaterState) {
			st.State = updateBlocked
			st.Detail = "GB Cloud can't write to its install folder. Reinstall it once with the latest installer; updates install by themselves after that. (" + err.Error() + ")"
		})
		go a.reportUpdateStatusNow(next)
		return errors.New(next.Detail)
	}
	dir, err := updatesDir()
	if err != nil {
		return err
	}
	helper := filepath.Join(os.TempDir(), fmt.Sprintf("kashicloud-update-helper-%d.exe", os.Getpid()))
	if err := copyFile(exe, helper); err != nil {
		os.Remove(newExePath(exe))
		return fmt.Errorf("prepare update helper: %w", err)
	}
	h.ExePath, h.ParentPID, h.Phase, h.StartedAt = exe, os.Getpid(), "waiting", time.Now()
	handoffPath := filepath.Join(dir, "handoff.json")
	if err := saveHandoff(handoffPath, h); err != nil {
		return err
	}
	if err := startDetached(helper, "--apply-update", handoffPath); err != nil {
		os.Remove(newExePath(exe))
		os.Remove(handoffPath)
		return fmt.Errorf("start update helper: %w", err)
	}

	next := a.setUpdateState(func(st *updaterState) {
		st.State, st.Detail = updateInstalling, fmt.Sprintf("Installing %s", h.ToVersion)
	})
	a.reportUpdateStatusNow(next)
	wailsruntime.EventsEmit(a.ctx, "app_update_installing", a.GetAppUpdateStatus())
	go func() {
		// Let the frontend paint its "Updating…" screen first.
		time.Sleep(1500 * time.Millisecond)
		wailsruntime.Quit(a.ctx)
	}()
	return nil
}

// finishHandoff runs at startup: if this launch came out of an install,
// record how it went. A "verifying" hand-off means this is the new version
// and the helper waits for ConfirmAppStarted; "rolled_back" means this is
// the old version, put back because the new one didn't start.
func (a *App) finishHandoff() {
	dir, err := updatesDir()
	if err != nil {
		return
	}
	path := filepath.Join(dir, "handoff.json")
	h, err := loadHandoff(path)
	if err != nil {
		return
	}
	u := &a.updater
	switch h.Phase {
	case "verifying", "healthy":
		if appVersion != h.ToVersion {
			// Not the version being verified (a stale file); the helper
			// will time out and roll back on its own if it's still there.
			if time.Since(h.StartedAt) > updateVerifyTimeout+updateExitTimeout {
				os.Remove(path)
			}
			return
		}
		u.mu.Lock()
		u.started = map[string]any{"result": h.Mode, "from": h.FromVersion, "to": h.ToVersion, "notes": h.Notes}
		u.mu.Unlock()
		// The helper died (a reboot mid-install): finish its job ourselves
		// once we've confirmed.
		if h.Phase == "healthy" || time.Since(h.StartedAt) > updateVerifyTimeout+updateExitTimeout {
			finalizeSwap(h.ExePath)
		}
	case "done":
		os.Remove(path)
	case "rolled_back":
		os.Remove(path)
		u.mu.Lock()
		u.started = map[string]any{"result": "failed", "from": h.FromVersion, "to": h.ToVersion, "detail": h.Detail}
		u.mu.Unlock()
		next := a.setUpdateState(func(st *updaterState) {
			st.State, st.SkipVersion, st.StagedPath = updateRolledBack, h.ToVersion, ""
			st.Detail = fmt.Sprintf("%s didn't start, so the till went back to %s. %s", h.ToVersion, appVersion, h.Detail)
		})
		a.reportUpdateStatusNow(next)
	}
}

// ConfirmAppStarted is called by the frontend once it is up. If this launch
// is a new version on trial, that's the signal the helper waits for; the
// result tells the frontend what to say ("Updated to …", "Went back to …").
func (a *App) ConfirmAppStarted() map[string]any {
	u := &a.updater
	u.mu.Lock()
	started := u.started
	already := u.confirmed
	u.confirmed = true
	u.mu.Unlock()
	if started == nil || already {
		return map[string]any{}
	}
	if started["result"] == "failed" {
		return started
	}

	dir, err := updatesDir()
	if err == nil {
		path := filepath.Join(dir, "handoff.json")
		if h, err := loadHandoff(path); err == nil && h.ToVersion == appVersion {
			h.Phase = "healthy"
			_ = saveHandoff(path, h)
		}
	}
	from, _ := started["from"].(string)
	state, detail := updateUpdated, fmt.Sprintf("Updated from %s", from)
	skip := ""
	if started["result"] == "rollback" {
		state, detail, skip = updateRolledBack, fmt.Sprintf("Rolled back from %s on the till", from), from
	}
	next := a.setUpdateState(func(st *updaterState) {
		st.State, st.Detail, st.Progress, st.StagedPath = state, detail, 0, ""
		st.PreviousVersion, st.SkipVersion = from, skip
	})
	go a.reportUpdateStatusNow(next)
	removeStagedDownloads("")
	return started
}

// runUpdateHelper is the helper process (`GB-Cloud.exe --apply-update
// <handoff.json>`), started from a temporary copy of the old executable so
// the program it replaces isn't running. It returns the process exit code.
func runUpdateHelper(handoffPath string, procs updateProcesses) int {
	h, err := loadHandoff(handoffPath)
	if err != nil {
		log.Printf("update helper: %v", err)
		return 1
	}
	fail := func(detail string) int {
		h.Phase, h.Detail = "rolled_back", detail
		_ = saveHandoff(handoffPath, h)
		return 1
	}

	if !procs.waitExit(h.ParentPID, updateExitTimeout) {
		os.Remove(newExePath(h.ExePath))
		h.Phase, h.Detail = "done", "GB Cloud didn't close; the update was cancelled"
		_ = saveHandoff(handoffPath, h)
		_ = procs.start(h.ExePath)
		return 1
	}

	// exe -> exe.old, exe.new -> exe. A file handle can linger for a moment
	// after the process exits, so retry briefly.
	if err := retryRename(h.ExePath, pendingOldExePath(h.ExePath)); err != nil {
		os.Remove(newExePath(h.ExePath))
		code := fail("couldn't move the old version aside: " + err.Error())
		_ = procs.start(h.ExePath)
		return code
	}
	if err := retryRename(newExePath(h.ExePath), h.ExePath); err != nil {
		_ = retryRename(pendingOldExePath(h.ExePath), h.ExePath)
		code := fail("couldn't put the new version in place: " + err.Error())
		_ = procs.start(h.ExePath)
		return code
	}

	h.Phase, h.StartedAt = "verifying", time.Now()
	if err := saveHandoff(handoffPath, h); err != nil {
		log.Printf("update helper: %v", err)
	}
	pid, err := procs.startPID(h.ExePath)
	healthy := false
	reason := ""
	if err != nil {
		reason = "it couldn't be started: " + err.Error()
	} else {
		deadline := time.Now().Add(updateVerifyTimeout)
		for time.Now().Before(deadline) {
			if cur, err := loadHandoff(handoffPath); err == nil && cur.Phase == "healthy" {
				healthy = true
				break
			}
			if !procs.alive(pid) {
				reason = "it closed right after starting"
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		if !healthy && reason == "" {
			reason = fmt.Sprintf("it didn't finish starting within %s", updateVerifyTimeout)
		}
	}

	if healthy {
		finalizeSwap(h.ExePath)
		if cur, err := loadHandoff(handoffPath); err == nil {
			h = cur
		}
		h.Phase = "done"
		_ = saveHandoff(handoffPath, h)
		return 0
	}

	if pid > 0 {
		procs.kill(pid)
		procs.waitExit(pid, 10*time.Second)
	}
	os.Remove(h.ExePath)
	if err := retryRename(pendingOldExePath(h.ExePath), h.ExePath); err != nil {
		log.Printf("update helper: restore old version: %v", err)
	}
	code := fail("The new version was rolled back because " + reason + ".")
	_ = procs.start(h.ExePath)
	return code
}

// finalizeSwap makes the old executable the new "previous" once the new
// version has proven itself. Idempotent.
func finalizeSwap(exe string) {
	old := pendingOldExePath(exe)
	if _, err := os.Stat(old); err != nil {
		return
	}
	os.Remove(previousExePath(exe))
	_ = retryRename(old, previousExePath(exe))
}

// cleanupUpdateLeftovers removes helper copies from earlier installs.
func cleanupUpdateLeftovers() {
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "kashicloud-update-helper-*.exe"))
	for _, m := range matches {
		if !strings.Contains(filepath.Base(m), fmt.Sprintf("-%d.exe", os.Getpid())) {
			os.Remove(m)
		}
	}
}

// updateProcesses is what the helper needs from the OS, swappable in tests.
type updateProcesses struct {
	waitExit func(pid int, timeout time.Duration) bool
	start    func(path string) error
	startPID func(path string) (int, error)
	alive    func(pid int) bool
	kill     func(pid int)
}

func loadHandoff(path string) (updateHandoff, error) {
	var h updateHandoff
	raw, err := os.ReadFile(path)
	if err != nil {
		return h, err
	}
	err = json.Unmarshal(raw, &h)
	return h, err
}

func saveHandoff(path string, h updateHandoff) error {
	raw, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(path, raw)
}

func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return retryRename(tmp, path)
}

func retryRename(from, to string) error {
	var err error
	for attempt := 0; attempt < 20; attempt++ {
		if err = os.Rename(from, to); err == nil {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return err
}

func currentExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return exe, nil
}
