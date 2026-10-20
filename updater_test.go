package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// signedManifest signs content with a fresh key installed as the release
// key for the test.
func signedManifest(t *testing.T, content []byte, version string) *appUpdateManifest {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	old := updatePublicKey
	updatePublicKey = base64.StdEncoding.EncodeToString(pub)
	t.Cleanup(func() { updatePublicKey = old })
	digest := sha256.Sum256(content)
	return &appUpdateManifest{
		ID: 7, Version: version, Flavor: appFlavor, FileName: "GB-Cloud.exe",
		SizeBytes: int64(len(content)), Sha256: hex.EncodeToString(digest[:]),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, digest[:])),
	}
}

func TestVerifyRelease(t *testing.T) {
	content := []byte("new build")
	m := signedManifest(t, content, "1.0.9")
	path := filepath.Join(t.TempDir(), "GB-Cloud.exe")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyRelease(path, m); err != nil {
		t.Fatalf("good release: %v", err)
	}

	// Same file, signed by some other key (a compromised kashi).
	_, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
	digest := sha256.Sum256(content)
	forged := *m
	forged.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(otherPriv, digest[:]))
	if err := verifyRelease(path, &forged); err == nil || !strings.Contains(err.Error(), "release key") {
		t.Fatalf("forged signature: err = %v, want a release key error", err)
	}

	if err := os.WriteFile(path, []byte("new buil"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyRelease(path, m); err == nil {
		t.Fatal("truncated file passed verification")
	}
}

func TestCheckAndDownload_DownloadsVerifiesAndReports(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	oldVersion := appVersion
	appVersion = "1.0.8"
	t.Cleanup(func() { appVersion = oldVersion })

	content := []byte(strings.Repeat("x", 200_000))
	m := signedManifest(t, content, "1.0.9")
	var reports []appUpdateStatusReport
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/branch/app_update":
			_ = json.NewEncoder(w).Encode(map[string]any{"release": m})
		case "/branch/app_update/download/7":
			http.ServeContent(w, r, "GB-Cloud.exe", time.Now(), strings.NewReader(string(content)))
		case "/branch/app_update/status":
			var rep appUpdateStatusReport
			_ = json.NewDecoder(r.Body).Decode(&rep)
			reports = append(reports, rep)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := &App{ctx: context.Background(), ready: make(chan struct{})}
	client := newCentralClient(srv.URL, "CN", "key")
	if err := a.checkAndDownload(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	s := a.updateStateSnapshot()
	if s.State != updateReady || s.Version != "1.0.9" {
		t.Fatalf("state = %+v, want ready 1.0.9", s)
	}
	staged, err := os.ReadFile(s.StagedPath)
	if err != nil || string(staged) != string(content) {
		t.Fatalf("staged file wrong: %v", err)
	}
	if len(reports) == 0 || reports[len(reports)-1].State != updateReady || reports[len(reports)-1].AppVersion != "1.0.8" {
		t.Fatalf("reports = %+v, want last one ready from 1.0.8", reports)
	}

	// A till that rolled back from 1.0.9 doesn't fetch it again.
	a.setUpdateState(func(st *updaterState) { st.State, st.SkipVersion, st.StagedPath = updateRolledBack, "1.0.9", "" })
	if err := a.checkAndDownload(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	if got := a.updateStateSnapshot().State; got != updateRolledBack {
		t.Fatalf("after rollback state = %s, want it to stay rolled_back", got)
	}
}

// fakeTill stands in for Windows processes: starting the exe "launches"
// whatever version its file content names, and a healthy version confirms
// by setting the hand-off phase the way ConfirmAppStarted does.
type fakeTill struct {
	t         *testing.T
	handoff   string
	healthy   map[string]bool
	launched  []string
	confirmed bool
}

func (f *fakeTill) procs() updateProcesses {
	launch := func(path string) (int, error) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return 0, err
		}
		version := string(raw)
		f.launched = append(f.launched, version)
		if f.healthy[version] {
			if h, err := loadHandoff(f.handoff); err == nil && h.Phase == "verifying" {
				h.Phase = "healthy"
				_ = saveHandoff(f.handoff, h)
				f.confirmed = true
			}
		}
		return len(f.launched), nil
	}
	return updateProcesses{
		waitExit: func(int, time.Duration) bool { return true },
		start:    func(p string) error { _, err := launch(p); return err },
		startPID: launch,
		alive:    func(int) bool { return false },
		kill:     func(int) {},
	}
}

func setupInstall(t *testing.T, current, next string) (exe, handoff string) {
	t.Helper()
	dir := t.TempDir()
	exe = filepath.Join(dir, "GB-Cloud.exe")
	if err := os.WriteFile(exe, []byte(current), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newExePath(exe), []byte(next), 0o644); err != nil {
		t.Fatal(err)
	}
	handoff = filepath.Join(dir, "handoff.json")
	if err := saveHandoff(handoff, updateHandoff{Mode: "update", FromVersion: current, ToVersion: next, ExePath: exe, Phase: "waiting"}); err != nil {
		t.Fatal(err)
	}
	return exe, handoff
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

func TestUpdateHelper_InstallsAndKeepsPrevious(t *testing.T) {
	exe, handoff := setupInstall(t, "1.0.8", "1.0.9")
	till := &fakeTill{t: t, handoff: handoff, healthy: map[string]bool{"1.0.9": true}}

	if code := runUpdateHelper(handoff, till.procs()); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if got := readFileString(t, exe); got != "1.0.9" {
		t.Fatalf("exe = %s, want 1.0.9", got)
	}
	if got := readFileString(t, previousExePath(exe)); got != "1.0.8" {
		t.Fatalf("previous = %s, want 1.0.8", got)
	}
	if h, _ := loadHandoff(handoff); h.Phase != "done" {
		t.Fatalf("phase = %s, want done", h.Phase)
	}
	if _, err := os.Stat(pendingOldExePath(exe)); !os.IsNotExist(err) {
		t.Fatal("old executable left behind")
	}
}

func TestUpdateHelper_RollsBackWhenNewVersionDoesNotStart(t *testing.T) {
	exe, handoff := setupInstall(t, "1.0.8", "1.0.9-broken")
	till := &fakeTill{t: t, handoff: handoff, healthy: map[string]bool{"1.0.8": true}}

	if code := runUpdateHelper(handoff, till.procs()); code == 0 {
		t.Fatal("helper reported success for a version that never started")
	}
	if got := readFileString(t, exe); got != "1.0.8" {
		t.Fatalf("exe = %s, want the old 1.0.8 back", got)
	}
	h, _ := loadHandoff(handoff)
	if h.Phase != "rolled_back" || !strings.Contains(h.Detail, "rolled back") {
		t.Fatalf("handoff = %+v, want rolled_back with a reason", h)
	}
	if fmt.Sprint(till.launched) != "[1.0.9-broken 1.0.8]" {
		t.Fatalf("launched %v, want the new version then the old one", till.launched)
	}
}

func TestUpdateHelper_ManualRollbackSwapsVersions(t *testing.T) {
	// A till on 1.0.9 with 1.0.8 kept as previous rolls back: the hand-off
	// stages a copy of the previous executable as .new.
	exe, handoff := setupInstall(t, "1.0.9", "1.0.8")
	if err := os.WriteFile(previousExePath(exe), []byte("1.0.8"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, _ := loadHandoff(handoff)
	h.Mode = "rollback"
	_ = saveHandoff(handoff, h)
	till := &fakeTill{t: t, handoff: handoff, healthy: map[string]bool{"1.0.8": true, "1.0.9": true}}

	if code := runUpdateHelper(handoff, till.procs()); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if got := readFileString(t, exe); got != "1.0.8" {
		t.Fatalf("exe = %s, want 1.0.8", got)
	}
	// ...and 1.0.9 becomes the "previous", so the rollback can be undone.
	if got := readFileString(t, previousExePath(exe)); got != "1.0.9" {
		t.Fatalf("previous = %s, want 1.0.9", got)
	}
}
