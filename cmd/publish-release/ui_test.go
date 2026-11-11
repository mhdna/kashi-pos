package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadFailureSaysWhatToDo(t *testing.T) {
	cases := []struct {
		status int
		answer string
		want   []string
		not    []string
	}{
		// The reported case: central being deployed answers 502 from its proxy.
		{502, "error code: 502", []string{"most likely being updated", "Nothing was published", "run the same command again"}, []string{"upload failed"}},
		{503, "", []string{"being updated"}, nil},
		{504, "", []string{"being updated"}, nil},
		{521, "", []string{"being updated"}, nil},
		{522, "", []string{"being updated"}, nil},
		{523, "", []string{"being updated"}, nil},
		{409, "standard 1.1.114 is already published; bump the version", []string{"already published", "make bump", "1.1.116"}, nil},
		{401, "no token", []string{"refused the upload", "no token"}, nil},
		{403, "", []string{"may not be allowed to publish"}, nil},
		{413, "", []string{"too big", "limit"}, nil},
		{400, "checksum mismatch", []string{"didn't accept", "checksum mismatch"}, nil},
		{500, "boom", []string{"status 500", "boom"}, []string{"being updated"}},
	}
	for _, c := range cases {
		got := uploadFailure(c.status, c.answer, "1.1.116", "standard")
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("status %d: %q should mention %q", c.status, got, w)
			}
		}
		for _, n := range c.not {
			if strings.Contains(got, n) {
				t.Errorf("status %d: %q should not mention %q", c.status, got, n)
			}
		}
	}
}

func TestPaintOnlyWhenColourIsOn(t *testing.T) {
	if got := paint(false, cRed, "x"); got != "x" {
		t.Fatalf("no colour: %q", got)
	}
	if got := paint(true, cRed, "x"); got != cRed+"x"+cReset {
		t.Fatalf("colour: %q", got)
	}
}

func TestColourFollowsNO_COLOR_andFORCE_COLOR(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "out") // a file, not a terminal
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	if colorOn(f) {
		t.Fatal("a file is not a terminal: no colour")
	}
	t.Setenv("FORCE_COLOR", "1")
	if !colorOn(f) {
		t.Fatal("FORCE_COLOR turns colour on")
	}
	t.Setenv("NO_COLOR", "1")
	if colorOn(f) {
		t.Fatal("NO_COLOR wins over FORCE_COLOR")
	}
}

// The build script: the noisy output goes to a log, one line shows, a failure
// shows the end of the log and the way out.
func runQuietBuild(t *testing.T, label string, cmd ...string) (string, int, string) {
	t.Helper()
	log := filepath.Join(t.TempDir(), "build.log")
	args := append([]string{"../../scripts/quiet-build.sh", label, log, "--"}, cmd...)
	c := exec.Command("sh", args...)
	c.Env = append(os.Environ(), "NO_COLOR=1")
	out, err := c.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(log)
	return string(out), code, string(raw)
}

func TestQuietBuildHidesTheNoise(t *testing.T) {
	out, code, log := runQuietBuild(t, "GB Cloud 9.9.9 (standard)", "sh", "-c", "echo noisy-line-1; echo noisy-line-2; echo to-stderr >&2")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if strings.Contains(out, "noisy-line") || strings.Contains(out, "to-stderr") {
		t.Fatalf("the build output must not reach the terminal:\n%s", out)
	}
	if !strings.Contains(out, "Building GB Cloud 9.9.9 (standard)") || !strings.Contains(out, "Built GB Cloud 9.9.9 (standard)") {
		t.Fatalf("expected a building line and a built line:\n%s", out)
	}
	if !strings.Contains(log, "noisy-line-1") || !strings.Contains(log, "to-stderr") {
		t.Fatalf("the log should hold stdout and stderr:\n%s", log)
	}
}

func TestQuietBuildFailureShowsTheEndOfTheLogAndKeepsTheStatus(t *testing.T) {
	out, code, _ := runQuietBuild(t, "GB Cloud 9.9.9 (standard)", "sh", "-c", "for i in $(seq 1 60); do echo line-$i; done; echo the-real-error >&2; exit 3")
	if code != 3 {
		t.Fatalf("the command's own exit status must come through, got %d", code)
	}
	for _, want := range []string{"Build failed: GB Cloud 9.9.9 (standard)", "the-real-error", "line-60", "Full build log:"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "line-1\n") || strings.Contains(out, "line-20\n") {
		t.Errorf("only the end of the log is shown:\n%s", out)
	}
}

func TestSayScriptPrintsEveryKind(t *testing.T) {
	for kind, mark := range map[string]string{"step": "▸", "ok": "✔", "warn": "!", "fail": "✘", "dim": ""} {
		c := exec.Command("sh", "../../scripts/say.sh", kind, "hello world")
		c.Env = append(os.Environ(), "NO_COLOR=1")
		out, err := c.CombinedOutput()
		if err != nil || !strings.Contains(string(out), mark+" hello world") && kind != "dim" || kind == "dim" && !strings.Contains(string(out), "hello world") {
			t.Errorf("%s: %q %v", kind, out, err)
		}
	}
	c := exec.Command("sh", "../../scripts/say.sh", "nope", "x")
	if err := c.Run(); err == nil {
		t.Error("an unknown kind must fail")
	}
}

// `make release…` calls make again: that must not print "Entering/Leaving
// directory" or echo the inner command line.
func TestMakefileReleaseIsQuiet(t *testing.T) {
	raw, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	mk := string(raw)
	if !strings.Contains(mk, "MAKEFLAGS += --no-print-directory") {
		t.Error("the Makefile must set --no-print-directory")
	}
	if !strings.Contains(mk, "\t@$(MAKE) --no-print-directory $(patsubst release%,publish%,$@)") {
		t.Error("the inner publish call must be silent (@) and not print directories")
	}
}

func TestMakeReleaseRunsWithoutDirectoryNoise(t *testing.T) {
	out, err := exec.Command("make", "-C", "../..", "-n", "release-all-force").CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if strings.Contains(string(out), "Entering directory") {
		t.Errorf("directory noise:\n%s", out)
	}
}
