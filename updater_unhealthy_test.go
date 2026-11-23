package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// procsBrokenOnStart: starting the named version "launches" it and, like a
// version whose migration failed, it marks the hand-off unhealthy at once and
// then stays alive (so only the unhealthy signal can end the wait).
func procsBrokenOnStart(t *testing.T, handoff, broken string, restoreDB, dbPath string) (updateProcesses, *[]string) {
	launched := &[]string{}
	launch := func(path string) (int, error) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return 0, err
		}
		*launched = append(*launched, string(raw))
		if string(raw) == broken {
			if !markUnhealthyAt(handoff, broken, "migrate database: migration 000300 failed", restoreDB, dbPath) {
				t.Fatal("could not mark the trial unhealthy")
			}
		}
		return len(*launched), nil
	}
	return updateProcesses{
		waitExit: func(int, time.Duration) bool { return true },
		start:    func(p string) error { _, err := launch(p); return err },
		startPID: launch,
		alive:    func(int) bool { return true },
		kill:     func(int) {},
	}, launched
}

func TestUpdateHelper_UnhealthyRollsBackAtOnceAndRestoresDatabase(t *testing.T) {
	exe, handoff := setupInstall(t, "1.0.8", "1.0.9-bad")
	dir := filepath.Dir(exe)
	dbPath := filepath.Join(dir, "database.db")
	backup := dbPath + ".backup-20261123-120000"
	for path, body := range map[string]string{dbPath: "HALF-MIGRATED", backup: "ORIGINAL", dbPath + "-wal": "w", dbPath + "-shm": "s"} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	procs, launched := procsBrokenOnStart(t, handoff, "1.0.9-bad", backup, dbPath)

	start := time.Now()
	if code := runUpdateHelper(handoff, procs); code == 0 {
		t.Fatal("helper reported success for a version that failed to start")
	}
	if time.Since(start) > 10*time.Second {
		t.Fatalf("took %v: it waited out the verify clock", time.Since(start))
	}
	if got := readFileString(t, exe); got != "1.0.8" {
		t.Fatalf("exe = %s, want the old 1.0.8 back", got)
	}
	if got := readFileString(t, dbPath); got != "ORIGINAL" {
		t.Fatalf("database = %q, want the backup put back", got)
	}
	for _, side := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(dbPath + side); !os.IsNotExist(err) {
			t.Fatalf("%s left behind", side)
		}
	}
	if got := readFileString(t, backup); got != "ORIGINAL" {
		t.Fatal("the backup itself must stay")
	}
	h, _ := loadHandoff(handoff)
	if h.Phase != "rolled_back" || !strings.Contains(h.Detail, "migration 000300 failed") {
		t.Fatalf("handoff = %+v, want rolled_back naming the migration", h)
	}
	if got := strings.Join(*launched, ","); got != "1.0.9-bad,1.0.8" {
		t.Fatalf("launched %s, want the new version then the old", got)
	}
}

func TestUpdateHelper_UnhealthyWithoutBackupLeavesDatabaseAlone(t *testing.T) {
	exe, handoff := setupInstall(t, "1.0.8", "1.0.9-bad")
	dbPath := filepath.Join(filepath.Dir(exe), "database.db")
	if err := os.WriteFile(dbPath, []byte("CURRENT"), 0o644); err != nil {
		t.Fatal(err)
	}
	procs, _ := procsBrokenOnStart(t, handoff, "1.0.9-bad", "", "")
	runUpdateHelper(handoff, procs)
	if got := readFileString(t, dbPath); got != "CURRENT" {
		t.Fatalf("database = %q, want it untouched", got)
	}
	if got := readFileString(t, exe); got != "1.0.8" {
		t.Fatalf("exe = %s", got)
	}
}

// A version that never confirms (no unhealthy signal) is rolled back, but its
// database is never touched: it may have been selling.
func TestUpdateHelper_PlainRollbackNeverRestoresDatabase(t *testing.T) {
	exe, handoff := setupInstall(t, "1.0.8", "1.0.9-broken")
	dbPath := filepath.Join(filepath.Dir(exe), "database.db")
	backup := dbPath + ".backup-x"
	for path, body := range map[string]string{dbPath: "CURRENT", backup: "OLD"} {
		_ = os.WriteFile(path, []byte(body), 0o644)
	}
	// A stale hand-off that happens to carry a backup must not be trusted.
	h, _ := loadHandoff(handoff)
	h.RestoreDB, h.DBPath = backup, dbPath
	_ = saveHandoff(handoff, h)
	till := &fakeTill{t: t, handoff: handoff, healthy: map[string]bool{"1.0.8": true}}
	runUpdateHelper(handoff, till.procs())
	if got := readFileString(t, dbPath); got != "CURRENT" {
		t.Fatalf("database = %q, want it untouched", got)
	}
}

func TestMarkUnhealthyAt(t *testing.T) {
	write := func(phase, to string) string {
		path := filepath.Join(t.TempDir(), "handoff.json")
		_ = saveHandoff(path, updateHandoff{Phase: phase, ToVersion: to})
		return path
	}
	path := write("verifying", "2.0")
	if !markUnhealthyAt(path, "2.0", "boom", "/b", "/d") {
		t.Fatal("a trial under verification must be marked")
	}
	h, _ := loadHandoff(path)
	if h.Phase != "unhealthy" || h.Detail != "boom" || h.RestoreDB != "/b" || h.DBPath != "/d" {
		t.Fatalf("handoff = %+v", h)
	}
	for _, c := range []struct{ phase, to string }{
		{"verifying", "1.0"}, {"waiting", "2.0"}, {"healthy", "2.0"}, {"done", "2.0"}, {"rolled_back", "2.0"}, {"unhealthy", "2.0"},
	} {
		p := write(c.phase, c.to)
		if markUnhealthyAt(p, "2.0", "x", "", "") {
			t.Fatalf("marked %+v", c)
		}
		if got, _ := loadHandoff(p); got.Phase != c.phase {
			t.Fatalf("phase changed to %s for %+v", got.Phase, c)
		}
	}
	if markUnhealthyAt(filepath.Join(t.TempDir(), "none.json"), "2.0", "x", "", "") {
		t.Fatal("marked a missing file")
	}
}

func TestRestoreDatabaseBackupErrors(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.db")
	_ = os.WriteFile(dbPath, []byte("CURRENT"), 0o644)
	for _, c := range [][2]string{{"", dbPath}, {filepath.Join(dir, "missing"), dbPath}, {filepath.Join(dir, "missing"), ""}} {
		if err := restoreDatabaseBackup(c[0], c[1]); err == nil {
			t.Fatalf("restore %v succeeded", c)
		}
		if got := readFileString(t, dbPath); got != "CURRENT" {
			t.Fatalf("database = %q after failed restore", got)
		}
	}
}
