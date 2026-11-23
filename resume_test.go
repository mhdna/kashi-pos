package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestResumeUsable(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	ok := resumeFile{Role: "cashier", Route: "/home", FromVersion: "1.1.188", ToVersion: "1.1.189", SavedAt: now.Add(-20 * time.Second)}
	cases := []struct {
		name    string
		mod     func(*resumeFile)
		version string
		want    bool
	}{
		{"fresh, new version", func(*resumeFile) {}, "1.1.189", true},
		{"fresh, rolled back to old version", func(*resumeFile) {}, "1.1.188", true},
		{"admin", func(f *resumeFile) { f.Role = "admin" }, "1.1.189", true},
		{"some third version", func(*resumeFile) {}, "1.1.150", false},
		{"dev build", func(*resumeFile) {}, "", false},
		{"no role", func(f *resumeFile) { f.Role = "" }, "1.1.189", false},
		{"unknown role", func(f *resumeFile) { f.Role = "root" }, "1.1.189", false},
		{"too old", func(f *resumeFile) { f.SavedAt = now.Add(-resumeMaxAge - time.Second) }, "1.1.189", false},
		{"just inside the age", func(f *resumeFile) { f.SavedAt = now.Add(-resumeMaxAge) }, "1.1.189", true},
		{"saved in the future", func(f *resumeFile) { f.SavedAt = now.Add(time.Hour) }, "1.1.189", false},
		{"zero time", func(f *resumeFile) { f.SavedAt = time.Time{} }, "1.1.189", false},
	}
	for _, c := range cases {
		f := ok
		c.mod(&f)
		if got := resumeUsable(f, c.version, now); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// The file works once: reading it deletes it, so a second restart, or a
// manual one, signs nobody in.
func TestTakeResumeFileWorksOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "updates", "resume.json")
	want := resumeFile{Role: "admin", Route: "/shifts?x=1", FromVersion: "1", ToVersion: "2", SavedAt: time.Now().Truncate(time.Second)}
	if err := writeResumeFile(path, want); err != nil {
		t.Fatal(err)
	}
	got, ok := takeResumeFile(path)
	if !ok || got.Role != want.Role || got.Route != want.Route || !got.SavedAt.Equal(want.SavedAt) {
		t.Fatalf("got %+v ok=%v, want %+v", got, ok, want)
	}
	if _, ok := takeResumeFile(path); ok {
		t.Fatal("the file was readable twice")
	}
}

func TestTakeResumeFileGarbage(t *testing.T) {
	dir := t.TempDir()
	if _, ok := takeResumeFile(filepath.Join(dir, "missing.json")); ok {
		t.Fatal("a missing file resumed")
	}
	path := filepath.Join(dir, "resume.json")
	if err := writeFileAtomic(path, []byte("{not json")); err != nil {
		t.Fatal(err)
	}
	if _, ok := takeResumeFile(path); ok {
		t.Fatal("garbage resumed")
	}
}

// What the screen answers is what is saved; a screen that never answers
// doesn't hold the restart past the wait.
func TestSaveRouteDelivery(t *testing.T) {
	a := &App{}
	ch := make(chan string, 1)
	a.updater.routeCh = ch
	a.SaveResumeRoute("/home")
	a.SaveResumeRoute("/ignored-second-answer")
	if got := <-ch; got != "/home" {
		t.Fatalf("got %q", got)
	}
	a.updater.routeCh = nil
	a.SaveResumeRoute("/nobody-listening") // must not block or panic
}
