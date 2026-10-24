package main

// Cash machine mode: a till runs GB Cloud and nothing else.
//
// Tills came with dozens of programs starting with Windows (OneDrive, Teams,
// updaters, game launchers, vendor trays) that took the CPU and memory the
// till needs. Two parts, both Windows only (cash_machine_windows.go):
//
//   - Every launch, while kiosk mode is on: the user's own startup entries
//     are switched off (the same switch as Task Manager's Startup tab, so
//     they can be switched back on there), and every other program the user
//     runs is closed, again every half minute while the till runs.
//     Windows' own programs and services are never touched.
//   - Once, from Settings -> General -> "Make this a cash machine", with a
//     Windows administrator's approval: the computer's startup entries and
//     third-party scheduled tasks off, the apps a till never uses removed,
//     background services off, no sleep, Windows Update outside opening
//     hours. Removed apps don't come back by themselves; the rest can be
//     turned on again by hand.
//
// What a till needs stays: GB Cloud itself (and its WebView2 and update
// helper), AnyDesk for remote support, touch screen drivers, the on-screen
// keyboard and Task Manager.

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// cashMachineKeepWords keep any program or startup entry whose path or name
// contains one of them.
var cashMachineKeepWords = []string{
	"anydesk",
	// Touch screen drivers, which some tills need for touch to work at all.
	"egalax", "touchkit", "elotouch", "ilitek", "eeti", "xtouch",
}

// cashMachineKeepPrograms are kept by file name.
var cashMachineKeepPrograms = map[string]bool{
	"msedgewebview2.exe": true, // the till's own window runs in it
	"tabtip.exe":         true, // the touch keyboard
	"taskmgr.exe":        true,
	"consent.exe":        true,
}

// runningProgram is one process, as cash machine mode sees it.
type runningProgram struct {
	pid, parent uint32
	path        string // full path of the executable, "" when it can't be read
}

// keepProgram reports whether a program in the till's own session stays.
// selfPath is GB Cloud's executable, windowsDir Windows' folder; ownPIDs is
// GB Cloud and every process under it.
func keepProgram(p runningProgram, selfPath, windowsDir string, ownPIDs map[uint32]bool) bool {
	if ownPIDs[p.pid] {
		return true
	}
	if p.path == "" {
		// Can't be read, so it can't be judged (an elevated program, say).
		return true
	}
	// Windows paths, compared as Windows does: without regard to case.
	path := strings.ToLower(p.path)
	if windowsDir != "" && strings.HasPrefix(path, strings.TrimRight(strings.ToLower(windowsDir), `\`)+`\`) {
		return true
	}
	if selfPath != "" && path == strings.ToLower(selfPath) {
		return true
	}
	name := path[strings.LastIndexAny(path, `\/`)+1:]
	if cashMachineKeepPrograms[name] || strings.HasPrefix(name, "kashicloud-update-helper-") {
		return true
	}
	return containsKeepWord(path)
}

// keepStartupEntry reports whether a startup entry stays on: GB Cloud's own
// (named own) and the ones a till needs.
func keepStartupEntry(name, command, own string) bool {
	if name == own {
		return true
	}
	return containsKeepWord(strings.ToLower(name + " " + command))
}

func containsKeepWord(s string) bool {
	for _, w := range cashMachineKeepWords {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// ownProcesses is pid and every process started under it, from a snapshot
// of all processes.
func ownProcesses(self uint32, programs []runningProgram) map[uint32]bool {
	own := map[uint32]bool{self: true}
	for grew := true; grew; {
		grew = false
		for _, p := range programs {
			if !own[p.pid] && own[p.parent] && p.pid != p.parent {
				own[p.pid] = true
				grew = true
			}
		}
	}
	return own
}

// startupApprovedValue is a StartupApproved entry, the 12 bytes Task
// Manager writes: a state (2 on, 3 off) and, when off, the time it was
// switched off.
func startupApprovedValue(enabled bool, at time.Time) []byte {
	v := make([]byte, 12)
	if enabled {
		v[0] = 2
		return v
	}
	v[0] = 3
	// FILETIME: 100ns ticks since 1601-01-01, little-endian.
	ft := uint64(at.UnixNano()/100) + 116444736000000000
	for i := 0; i < 8; i++ {
		v[4+i] = byte(ft >> (8 * i))
	}
	return v
}

// startupApprovedOff reports whether a StartupApproved entry says off.
func startupApprovedOff(v []byte) bool {
	return len(v) > 0 && v[0]&1 == 1
}

// CashMachineStep is one thing "Make this a cash machine" did.
type CashMachineStep struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// CashMachineStatus is what Settings shows about cash machine mode.
type CashMachineStatus struct {
	Supported bool `json:"supported"`
	// On: closing other programs is running now (kiosk mode is on).
	On bool `json:"on"`
	// ClosedPrograms were closed since GB Cloud started, by file name.
	ClosedPrograms []CashMachineCount `json:"closedPrograms"`
	// StartupOff are the user's startup entries switched off this launch.
	StartupOff []string `json:"startupOff"`
	// LastSetup is the last "Make this a cash machine", if any.
	LastSetup *CashMachineSetup `json:"lastSetup,omitempty"`
}

type CashMachineCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type CashMachineSetup struct {
	At    time.Time         `json:"at"`
	Steps []CashMachineStep `json:"steps"`
}

var cashMachine struct {
	mu         sync.Mutex
	on         bool
	closed     map[string]int
	startupOff []string
}

func noteClosedProgram(name string) {
	cashMachine.mu.Lock()
	defer cashMachine.mu.Unlock()
	if cashMachine.closed == nil {
		cashMachine.closed = map[string]int{}
	}
	cashMachine.closed[name]++
}

// GetCashMachineStatus is for Settings -> General.
func (a *App) GetCashMachineStatus() CashMachineStatus {
	cashMachine.mu.Lock()
	s := CashMachineStatus{
		Supported:      cashMachineSupported && !isDevMode,
		On:             cashMachine.on,
		ClosedPrograms: []CashMachineCount{},
		StartupOff:     append([]string{}, cashMachine.startupOff...),
	}
	for name, n := range cashMachine.closed {
		s.ClosedPrograms = append(s.ClosedPrograms, CashMachineCount{Name: name, Count: n})
	}
	cashMachine.mu.Unlock()
	sort.Slice(s.ClosedPrograms, func(i, j int) bool { return s.ClosedPrograms[i].Name < s.ClosedPrograms[j].Name })
	s.LastSetup = loadCashMachineSetup()
	return s
}

// SetUpCashMachine is "Make this a cash machine": the user's part here, then
// the computer's part in a copy of GB Cloud run as administrator, which
// Windows asks approval for.
func (a *App) SetUpCashMachine() (CashMachineSetup, error) {
	if !cashMachineSupported || isDevMode {
		return CashMachineSetup{}, errors.New("only on a till running Windows, not from a development build")
	}
	steps := setUpCashMachineForUser()
	machine, err := setUpCashMachineAsAdmin()
	if err != nil {
		steps = append(steps, CashMachineStep{Name: "Computer settings (administrator)", Detail: err.Error()})
	}
	setup := CashMachineSetup{At: time.Now(), Steps: append(steps, machine...)}
	saveCashMachineSetup(setup)
	return setup, nil
}

func cashMachineSetupPath() (string, error) {
	dir, err := branchRegistryDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cash-machine.json"), nil
}

func loadCashMachineSetup() *CashMachineSetup {
	path, err := cashMachineSetupPath()
	if err != nil {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var setup CashMachineSetup
	if json.Unmarshal(raw, &setup) != nil {
		return nil
	}
	return &setup
}

func saveCashMachineSetup(setup CashMachineSetup) {
	path, err := cashMachineSetupPath()
	if err == nil {
		err = os.MkdirAll(filepath.Dir(path), 0o755)
	}
	if err == nil {
		var raw []byte
		if raw, err = json.MarshalIndent(setup, "", "  "); err == nil {
			err = writeFileAtomic(path, raw)
		}
	}
	if err != nil {
		log.Printf("cash machine: save setup: %v", err)
	}
}
