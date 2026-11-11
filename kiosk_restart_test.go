package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRelaunchScriptWaitsThenStartsTheExe(t *testing.T) {
	system32 := filepath.Join(`C:\Windows`, "System32")
	script := relaunchScript(system32, `C:\Program Files\GB Cloud\GB-Cloud.exe`)

	// Wait first (three pings), then start; the start uses cmd's `start ""` so
	// it doesn't wait for the app, and a path with spaces stays in quotes.
	ping := strings.Index(script, "PING.EXE")
	start := strings.Index(script, `start ""`)
	if ping < 0 || start < 0 || ping > start {
		t.Fatalf("want the wait before the start, got %q", script)
	}
	if !strings.Contains(script, "-n 3 127.0.0.1 >nul") {
		t.Errorf("no three-ping wait in %q", script)
	}
	if !strings.HasSuffix(script, `start "" "C:\Program Files\GB Cloud\GB-Cloud.exe"`) {
		t.Errorf("the exe path must be quoted and last: %q", script)
	}
	if !strings.Contains(script, "&") {
		t.Errorf("the two commands must be chained: %q", script)
	}
}

func TestRelaunchScriptKeepsOddPathsWhole(t *testing.T) {
	for _, exe := range []string{`C:\gb.exe`, `D:\Tills\Centro Mall\gb cloud.exe`, `C:\Users\Cashier (1)\GB-Cloud.exe`} {
		script := relaunchScript(`C:\Windows\System32`, exe)
		if !strings.HasSuffix(script, `start "" "`+exe+`"`) {
			t.Errorf("exe %q mangled in %q", exe, script)
		}
	}
}

// A development build must never relaunch itself: it would start a second
// `wails dev` binary against the live database.
func TestRestartAppRefusesInDevBuilds(t *testing.T) {
	if !isDevMode {
		t.Skip("production build")
	}
	app := &App{}
	if err := app.RestartApp(); err == nil {
		t.Fatal("RestartApp must refuse in a development build")
	}
}
