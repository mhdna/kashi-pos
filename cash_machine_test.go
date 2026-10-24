package main

import (
	"testing"
	"time"
)

func TestKeepProgram(t *testing.T) {
	const self = `C:\Program Files\GB Cloud\GB-Cloud.exe`
	const windowsDir = `C:\Windows`
	own := ownProcesses(100, []runningProgram{
		{pid: 100, parent: 4},
		{pid: 200, parent: 100}, // GB Cloud's WebView2
		{pid: 300, parent: 200}, // and its renderer
		{pid: 400, parent: 50},
	})
	cases := []struct {
		name string
		p    runningProgram
		keep bool
	}{
		{"GB Cloud", runningProgram{pid: 100, path: self}, true},
		{"under GB Cloud", runningProgram{pid: 300, path: `C:\Users\till\AppData\Local\Other\x.exe`}, true},
		{"Windows' own", runningProgram{pid: 400, path: `C:\Windows\System32\ctfmon.exe`}, true},
		{"Explorer, case aside", runningProgram{pid: 401, path: `c:\windows\explorer.exe`}, true},
		{"a second GB Cloud", runningProgram{pid: 402, path: `c:\program files\gb cloud\gb-cloud.exe`}, true},
		{"update helper", runningProgram{pid: 403, path: `C:\Users\till\AppData\Local\Temp\kashicloud-update-helper-100.exe`}, true},
		{"AnyDesk", runningProgram{pid: 404, path: `C:\Program Files (x86)\AnyDesk\AnyDesk.exe`}, true},
		{"touch driver", runningProgram{pid: 405, path: `C:\Program Files\eGalaxTouch\xTouch.exe`}, true},
		{"touch keyboard", runningProgram{pid: 406, path: `C:\Program Files\Common Files\microsoft shared\ink\TabTip.exe`}, true},
		{"unreadable", runningProgram{pid: 407}, true},
		{"OneDrive", runningProgram{pid: 500, path: `C:\Users\till\AppData\Local\Microsoft\OneDrive\OneDrive.exe`}, false},
		{"Teams", runningProgram{pid: 501, path: `C:\Program Files\WindowsApps\MSTeams_1.0\ms-teams.exe`}, false},
		{"Chrome", runningProgram{pid: 502, path: `C:\Program Files\Google\Chrome\Application\chrome.exe`}, false},
		{"a folder named like Windows", runningProgram{pid: 503, path: `C:\WindowsApps2\junk.exe`}, false},
	}
	for _, c := range cases {
		if got := keepProgram(c.p, self, windowsDir, own); got != c.keep {
			t.Errorf("%s: keep = %v, want %v", c.name, got, c.keep)
		}
	}
}

func TestKeepStartupEntry(t *testing.T) {
	for _, c := range []struct {
		name, command string
		keep          bool
	}{
		{"GB Cloud", `"C:\Program Files\GB Cloud\GB-Cloud.exe"`, true},
		{"AnyDesk", `"C:\Program Files (x86)\AnyDesk\AnyDesk.exe" --tray`, true},
		{"Touch", `C:\Program Files\eGalaxTouch\xTouch.exe`, true},
		{"OneDrive", `"C:\Users\till\AppData\Local\Microsoft\OneDrive\OneDrive.exe" /background`, false},
		{"Steam", `"C:\Program Files (x86)\Steam\steam.exe" -silent`, false},
	} {
		if got := keepStartupEntry(c.name, c.command, "GB Cloud"); got != c.keep {
			t.Errorf("%s: keep = %v, want %v", c.name, got, c.keep)
		}
	}
}

func TestStartupApprovedValue(t *testing.T) {
	on := startupApprovedValue(true, time.Now())
	if len(on) != 12 || on[0] != 2 || startupApprovedOff(on) {
		t.Fatalf("on = %v", on)
	}
	at := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	off := startupApprovedValue(false, at)
	if len(off) != 12 || off[0] != 3 || !startupApprovedOff(off) {
		t.Fatalf("off = %v", off)
	}
	var ft uint64
	for i := 0; i < 8; i++ {
		ft |= uint64(off[4+i]) << (8 * i)
	}
	if got := time.Unix(0, int64(ft-116444736000000000)*100).UTC(); !got.Equal(at) {
		t.Errorf("switched off at %v, want %v", got, at)
	}
	// Task Manager's own "enabled" value for an entry the app is the first
	// to see: 6.
	if startupApprovedOff([]byte{6, 0, 0, 0}) || startupApprovedOff(nil) {
		t.Error("an enabled or missing value read as off")
	}
}
