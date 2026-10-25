//go:build windows

package main

// Cash machine mode on Windows; what it does and why is in cash_machine.go.

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const cashMachineSupported = true

// cashMachineSweepEvery is how often other programs are closed again: a
// program the user opens, or one that restarts itself, goes within this.
const cashMachineSweepEvery = 30 * time.Second

var cashMachineStop chan struct{}

// startCashMachine runs with kiosk mode: the user's startup entries off,
// then other programs closed now and every half minute. Off on a till with
// KASHI_CASH_MACHINE=false.
func startCashMachine() {
	if !envBool("KASHI_CASH_MACHINE", true) {
		return
	}
	cashMachine.mu.Lock()
	if cashMachine.on {
		cashMachine.mu.Unlock()
		return
	}
	cashMachine.on = true
	stop := make(chan struct{})
	cashMachineStop = stop
	cashMachine.mu.Unlock()

	off, err := switchOffStartup(userStartupPlaces())
	if err != nil {
		log.Printf("cash machine: startup entries: %v", err)
	}
	cashMachine.mu.Lock()
	cashMachine.startupOff = off
	cashMachine.mu.Unlock()

	go func() {
		ticker := time.NewTicker(cashMachineSweepEvery)
		defer ticker.Stop()
		for {
			closeOtherPrograms()
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
	}()
}

func stopCashMachine() {
	cashMachine.mu.Lock()
	defer cashMachine.mu.Unlock()
	if cashMachine.on {
		close(cashMachineStop)
		cashMachine.on = false
	}
}

// closeOtherPrograms closes every program in the till's session that it
// doesn't need (keepProgram). Services and other users' programs run in
// other sessions and aren't looked at.
func closeOtherPrograms() {
	self := uint32(os.Getpid())
	var session uint32
	if err := windows.ProcessIdToSessionId(self, &session); err != nil {
		log.Printf("cash machine: own session: %v", err)
		return
	}
	all, err := listPrograms()
	if err != nil {
		log.Printf("cash machine: list programs: %v", err)
		return
	}
	own := ownProcesses(self, all)
	selfPath, _ := os.Executable()
	windowsDir, _ := windows.GetWindowsDirectory()
	for _, p := range all {
		var s uint32
		if windows.ProcessIdToSessionId(p.pid, &s) != nil || s != session {
			continue
		}
		p.path = programPath(p.pid)
		if keepProgram(p, selfPath, windowsDir, own) {
			continue
		}
		name := filepath.Base(p.path)
		if err := terminateProgram(p.pid); err != nil {
			log.Printf("cash machine: close %s (%d): %v", name, p.pid, err)
			continue
		}
		log.Printf("cash machine: closed %s (%d)", name, p.pid)
		noteClosedProgram(name)
	}
}

func listPrograms() ([]runningProgram, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	var programs []runningProgram
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		programs = append(programs, runningProgram{pid: e.ProcessID, parent: e.ParentProcessID})
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, err
	}
	return programs, nil
}

// programPath is the process's executable, "" when it can't be read.
func programPath(pid uint32) string {
	h, err := windows.OpenProcess(processQueryLimitedInformation, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:size])
}

func terminateProgram(pid uint32) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.TerminateProcess(h, 1)
}

// startupPlace is one place Windows starts programs from at sign-in, and the
// StartupApproved key Task Manager's Startup tab keeps its on/off switch
// for that place in.
type startupPlace struct {
	root registry.Key
	// run is a Run key under root; folder a Startup folder (when run is "").
	run, folder string
	approved    string
}

const startupApprovedKey = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\`

func userStartupPlaces() []startupPlace {
	return []startupPlace{
		{root: registry.CURRENT_USER, run: autostartKeyPath, approved: startupApprovedKey + "Run"},
		{root: registry.CURRENT_USER, folder: filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs\Startup`), approved: startupApprovedKey + "StartupFolder"},
	}
}

// machineStartupPlaces start programs for every user; switching them off
// needs an administrator.
func machineStartupPlaces() []startupPlace {
	return []startupPlace{
		{root: registry.LOCAL_MACHINE, run: autostartKeyPath, approved: startupApprovedKey + "Run"},
		{root: registry.LOCAL_MACHINE, run: `Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Run`, approved: startupApprovedKey + "Run32"},
		{root: registry.LOCAL_MACHINE, folder: filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs\StartUp`), approved: startupApprovedKey + "StartupFolder"},
	}
}

// switchOffStartup switches off every entry in places that a till doesn't
// need and that isn't off already, and returns their names.
func switchOffStartup(places []startupPlace) ([]string, error) {
	var off []string
	var errs []error
	for _, place := range places {
		entries, err := startupEntries(place)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if len(entries) == 0 {
			continue
		}
		approved, _, err := registry.CreateKey(place.root, place.approved, registry.QUERY_VALUE|registry.SET_VALUE)
		if err != nil {
			errs = append(errs, fmt.Errorf("open %s: %w", place.approved, err))
			continue
		}
		for name, command := range entries {
			if keepStartupEntry(name, command, autostartValueName) {
				continue
			}
			if v, _, err := approved.GetBinaryValue(name); err == nil && startupApprovedOff(v) {
				continue
			}
			if err := approved.SetBinaryValue(name, startupApprovedValue(false, time.Now())); err != nil {
				errs = append(errs, fmt.Errorf("switch off %s: %w", name, err))
				continue
			}
			off = append(off, name)
		}
		approved.Close()
	}
	return off, errors.Join(errs...)
}

// startupEntries maps each entry's name to its command (a Startup folder's
// shortcuts by file name, which is also what StartupApproved keys them by).
func startupEntries(place startupPlace) (map[string]string, error) {
	entries := map[string]string{}
	if place.run == "" {
		files, err := os.ReadDir(place.folder)
		if err != nil {
			if os.IsNotExist(err) {
				return entries, nil
			}
			return nil, err
		}
		for _, f := range files {
			if !f.IsDir() && !strings.EqualFold(f.Name(), "desktop.ini") {
				entries[f.Name()] = f.Name()
			}
		}
		return entries, nil
	}
	key, err := registry.OpenKey(place.root, place.run, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return entries, nil
		}
		return nil, err
	}
	defer key.Close()
	names, err := key.ReadValueNames(0)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		command, _, _ := key.GetStringValue(name)
		entries[name] = command
	}
	return entries, nil
}

// registryValues sets DWORD values under root, creating keys as needed.
func registryValues(root registry.Key, values map[string]map[string]uint32) error {
	var errs []error
	for path, names := range values {
		key, _, err := registry.CreateKey(root, path, registry.SET_VALUE)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		for name, v := range names {
			if err := key.SetDWordValue(name, v); err != nil {
				errs = append(errs, fmt.Errorf(`%s\%s: %w`, path, name, err))
			}
		}
		key.Close()
	}
	return errors.Join(errs...)
}

func step(name string, detail string, err error) CashMachineStep {
	if err != nil {
		if detail != "" {
			detail += "; "
		}
		return CashMachineStep{Name: name, Detail: detail + err.Error()}
	}
	return CashMachineStep{Name: name, OK: true, Detail: detail}
}

func listOrNone(names []string) string {
	if len(names) == 0 {
		return "nothing to change"
	}
	return strings.Join(names, ", ")
}

// setUpCashMachineForUser is the part of "Make this a cash machine" that is
// the till user's own; it runs as that user, since an administrator who
// approves the rest may be someone else.
func setUpCashMachineForUser() []CashMachineStep {
	var steps []CashMachineStep

	off, err := switchOffStartup(userStartupPlaces())
	steps = append(steps, step("Startup apps (this user) off", listOrNone(off), err))

	const cv = `Software\Microsoft\Windows\CurrentVersion\`
	steps = append(steps, step("Notifications off", "", registryValues(registry.CURRENT_USER, map[string]map[string]uint32{
		cv + "PushNotifications": {"ToastEnabled": 0},
	})))
	steps = append(steps, step("Tips, ads and suggested apps off", "", registryValues(registry.CURRENT_USER, map[string]map[string]uint32{
		cv + "ContentDeliveryManager": {
			"ContentDeliveryAllowed": 0, "SilentInstalledAppsEnabled": 0, "SystemPaneSuggestionsEnabled": 0,
			"SoftLandingEnabled": 0, "OemPreInstalledAppsEnabled": 0, "PreInstalledAppsEnabled": 0,
			"SubscribedContent-338388Enabled": 0, "SubscribedContent-338389Enabled": 0,
			"SubscribedContent-310093Enabled": 0, "SubscribedContent-353698Enabled": 0,
		},
	})))
	steps = append(steps, step("Background apps off", "", registryValues(registry.CURRENT_USER, map[string]map[string]uint32{
		cv + "BackgroundAccessApplications": {"GlobalUserDisabled": 1},
		cv + "Search":                       {"BackgroundAppGlobalToggle": 0},
	})))
	steps = append(steps, step("Visual effects set for performance", "", registryValues(registry.CURRENT_USER, map[string]map[string]uint32{
		cv + `Explorer\VisualEffects`: {"VisualFXSetting": 2},
	})))
	steps = append(steps, removeOneDrive())
	return steps
}

// removeOneDrive uninstalls the user's OneDrive with its own uninstaller.
func removeOneDrive() CashMachineStep {
	const name = "OneDrive removed"
	if _, err := os.Stat(filepath.Join(os.Getenv("LOCALAPPDATA"), `Microsoft\OneDrive\OneDrive.exe`)); err != nil {
		return step(name, "not installed", nil)
	}
	windowsDir, _ := windows.GetWindowsDirectory()
	for _, dir := range []string{"SysWOW64", "System32"} {
		setup := filepath.Join(windowsDir, dir, "OneDriveSetup.exe")
		if _, err := os.Stat(setup); err != nil {
			continue
		}
		_, err := runHidden(5*time.Minute, setup, "/uninstall")
		return step(name, "", err)
	}
	return step(name, "", errors.New("OneDrive's uninstaller wasn't found"))
}

// runHidden runs a program with no window and returns what it printed.
func runHidden(timeout time.Duration, path string, args ...string) (string, error) {
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	var out strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		return "", err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return out.String(), fmt.Errorf("%w: %s", err, strings.TrimSpace(out.String()))
		}
		return out.String(), nil
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return out.String(), errors.New("took too long")
	}
}

func powershell(timeout time.Duration, script string) ([]string, error) {
	windowsDir, _ := windows.GetWindowsDirectory()
	exe := filepath.Join(windowsDir, `System32\WindowsPowerShell\v1.0\powershell.exe`)
	out, err := runHidden(timeout, exe, "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, err
}

// SHELLEXECUTEINFOW, for running GB Cloud as administrator.
type shellExecuteInfo struct {
	size         uint32
	mask         uint32
	hwnd         uintptr
	verb         *uint16
	file         *uint16
	parameters   *uint16
	directory    *uint16
	show         int32
	instApp      uintptr
	idList       uintptr
	class        *uint16
	keyClass     uintptr
	hotKey       uint32
	iconOrMonitr uintptr
	process      windows.Handle
}

var pShellExecuteExW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

const seeMaskNoCloseProcess = 0x00000040

// setUpCashMachineAsAdmin runs the computer's part (runCashMachineSetup) in
// a copy of GB Cloud started as administrator, which Windows asks approval
// for, and waits for what it did.
func setUpCashMachineAsAdmin() ([]CashMachineStep, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	result := filepath.Join(os.TempDir(), fmt.Sprintf("gb-cloud-cash-machine-%d.json", os.Getpid()))
	os.Remove(result)
	defer os.Remove(result)

	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	params, _ := windows.UTF16PtrFromString(`--cash-machine-setup "` + result + `"`)
	owner, approved := beginApprovalPrompt()
	info := shellExecuteInfo{mask: seeMaskNoCloseProcess, hwnd: owner, verb: verb, file: file, parameters: params}
	info.size = uint32(unsafe.Sizeof(info))
	// Returns once the approval prompt has been answered.
	r, _, err := pShellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	approved()
	if r == 0 {
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return nil, errors.New("Windows' administrator approval was declined")
		}
		return nil, err
	}
	defer windows.CloseHandle(info.process)
	// Removing apps is the slow part; a few minutes on an old till.
	if ev, err := windows.WaitForSingleObject(info.process, uint32((20 * time.Minute).Milliseconds())); err != nil || ev != windows.WAIT_OBJECT_0 {
		return nil, errors.New("the administrator part didn't finish in 20 minutes")
	}
	raw, err := os.ReadFile(result)
	if err != nil {
		return nil, fmt.Errorf("the administrator part left no result: %w", err)
	}
	var steps []CashMachineStep
	if err := json.Unmarshal(raw, &steps); err != nil {
		return nil, err
	}
	return steps, nil
}

// runCashMachineSetup is the administrator copy (`GB-Cloud.exe
// --cash-machine-setup <result>`): it changes the computer's settings and
// writes what it did to result. No window.
func runCashMachineSetup(result string) int {
	steps := []CashMachineStep{}

	off, err := switchOffStartup(machineStartupPlaces())
	steps = append(steps, step("Startup apps (computer) off", listOrNone(off), err))
	steps = append(steps, disableScheduledTasks())
	steps = append(steps, disableServices())
	steps = append(steps, step("Windows extras off (widgets, Cortana, Copilot, game bar, telemetry, background apps)", "",
		registryValues(registry.LOCAL_MACHINE, cashMachinePolicies)))
	// Active hours, when Windows won't restart for updates, cover the shops'
	// working hours (8:00 or 10:00 to 22:00) with an hour either side for
	// opening and closing. Windows allows at most 18 hours.
	steps = append(steps, step("Windows Update restarts only outside 7:00-23:00", "", registryValues(registry.LOCAL_MACHINE, map[string]map[string]uint32{
		`SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate`:    {"SetActiveHours": 1, "ActiveHoursStart": 7, "ActiveHoursEnd": 23},
		`SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU`: {"NoAutoRebootWithLoggedOnUsers": 1},
	})))
	steps = append(steps, setPower())
	steps = append(steps, removeApps())

	raw, _ := json.Marshal(steps)
	if err := os.WriteFile(result, raw, 0o644); err != nil {
		return 1
	}
	return 0
}

var cashMachinePolicies = map[string]map[string]uint32{
	`SOFTWARE\Policies\Microsoft\Windows\CloudContent`:         {"DisableWindowsConsumerFeatures": 1, "DisableSoftLanding": 1, "DisableCloudOptimizedContent": 1},
	`SOFTWARE\Policies\Microsoft\Windows\DataCollection`:       {"AllowTelemetry": 0},
	`SOFTWARE\Policies\Microsoft\Windows\AppPrivacy`:           {"LetAppsRunInBackground": 2},
	`SOFTWARE\Policies\Microsoft\Windows\Windows Feeds`:        {"EnableFeeds": 0},
	`SOFTWARE\Policies\Microsoft\Dsh`:                          {"AllowNewsAndInterests": 0},
	`SOFTWARE\Policies\Microsoft\Windows\Windows Search`:       {"AllowCortana": 0},
	`SOFTWARE\Policies\Microsoft\Windows\GameDVR`:              {"AllowGameDVR": 0},
	`SOFTWARE\Policies\Microsoft\Windows\OneDrive`:             {"DisableFileSyncNGSC": 1},
	`SOFTWARE\Policies\Microsoft\Windows\WindowsCopilot`:       {"TurnOffWindowsCopilot": 1},
	`SOFTWARE\Policies\Microsoft\Windows\DeliveryOptimization`: {"DODownloadMode": 0},
	// Edge keeps itself running in the background; WebView2, which the till
	// runs in, doesn't read Edge's policies.
	`SOFTWARE\Policies\Microsoft\Edge`: {"StartupBoostEnabled": 0, "BackgroundModeEnabled": 0},
}

// cashMachineServices are background services a till never needs. The
// print spooler, Windows Update and AnyDesk's service stay.
var cashMachineServices = []string{
	"SysMain",          // preloads apps into memory
	"WSearch",          // search indexing
	"DiagTrack",        // telemetry
	"dmwappushservice", // telemetry
	"MapsBroker", "lfsvc", "RetailDemo", "Fax", "WMPNetworkSvc", "WerSvc", "wisvc",
	"XblAuthManager", "XblGameSave", "XboxNetApiSvc", "XboxGipSvc",
}

func disableServices() CashMachineStep {
	const name = "Background services off"
	m, err := mgr.Connect()
	if err != nil {
		return step(name, "", err)
	}
	defer m.Disconnect()
	var done []string
	var errs []error
	for _, n := range cashMachineServices {
		s, err := m.OpenService(n)
		if err != nil {
			continue // not on this edition of Windows
		}
		cfg, err := s.Config()
		if err == nil && cfg.StartType != mgr.StartDisabled {
			cfg.StartType = mgr.StartDisabled
			if err = s.UpdateConfig(cfg); err == nil {
				done = append(done, n)
			}
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", n, err))
		}
		_, _ = s.Control(svc.Stop)
		s.Close()
	}
	return step(name, listOrNone(done), errors.Join(errs...))
}

// disableScheduledTasks switches off third-party scheduled tasks (anything
// outside Windows' own \Microsoft\ folder: updaters, OneDrive, vendor
// tools) and Windows' telemetry tasks. Edge's updater stays: it also keeps
// WebView2, which the till runs in, up to date.
func disableScheduledTasks() CashMachineStep {
	keep := strings.Join(append([]string{"edgeupdate", "gb cloud", "kashi"}, cashMachineKeepWords...), "|")
	script := `$keep = '` + keep + `'
$telemetry = @('\Microsoft\Windows\Application Experience\', '\Microsoft\Windows\Customer Experience Improvement Program\', '\Microsoft\Windows\Feedback\Siuf\', '\Microsoft\XblGameSave\', '\Microsoft\Windows\Maps\')
Get-ScheduledTask | Where-Object {
  $_.State -ne 'Disabled' -and (
    ($_.TaskPath -notlike '\Microsoft\*' -and ($_.TaskPath + $_.TaskName) -notmatch $keep) -or
    ($telemetry -contains $_.TaskPath))
} | ForEach-Object {
  try { Disable-ScheduledTask -TaskName $_.TaskName -TaskPath $_.TaskPath -ErrorAction Stop | Out-Null; $_.TaskName } catch {}
}`
	names, err := powershell(5*time.Minute, script)
	return step("Scheduled tasks off", listOrNone(names), err)
}

// cashMachineApps are the Windows apps removed, for every user and for
// users added later. The Store, Calculator and Photos stay.
var cashMachineApps = []string{
	"Microsoft.BingNews", "Microsoft.BingWeather", "Microsoft.BingSearch",
	"Microsoft.GamingApp", "Microsoft.XboxApp", "Microsoft.Xbox.TCUI", "Microsoft.XboxGameOverlay",
	"Microsoft.XboxGamingOverlay", "Microsoft.XboxIdentityProvider", "Microsoft.XboxSpeechToTextOverlay",
	"Microsoft.GetHelp", "Microsoft.Getstarted", "Microsoft.MicrosoftOfficeHub", "Microsoft.MicrosoftSolitaireCollection",
	"Microsoft.People", "Microsoft.SkypeApp", "Microsoft.YourPhone", "Microsoft.ZuneMusic", "Microsoft.ZuneVideo",
	"Microsoft.WindowsMaps", "Microsoft.WindowsFeedbackHub", "Microsoft.MixedReality.Portal", "Microsoft.Microsoft3DViewer",
	"Microsoft.Print3D", "Microsoft.Office.OneNote", "Microsoft.OneConnect", "Microsoft.Messaging", "Microsoft.Wallet",
	"Microsoft.Todos", "Microsoft.PowerAutomateDesktop", "Microsoft.549981C3F5F10", "Microsoft.WindowsAlarms",
	"microsoft.windowscommunicationsapps", "Microsoft.OutlookForWindows", "Microsoft.Windows.DevHome",
	"Microsoft.Copilot", "Microsoft.Windows.Ai.Copilot.Provider", "MicrosoftWindows.Client.WebExperience",
	"MicrosoftTeams", "MSTeams", "Clipchamp.Clipchamp",
	// What PC makers and Windows put on as "suggested" apps.
	"*SpotifyMusic*", "*Disney*", "*CandyCrush*", "*king.com*", "*Netflix*", "*TikTok*", "*Instagram*",
	"*Facebook*", "*PrimeVideo*", "*Twitter*", "*LinkedIn*", "*McAfee*", "*Norton*",
}

func removeApps() CashMachineStep {
	quoted := make([]string, len(cashMachineApps))
	for i, n := range cashMachineApps {
		quoted[i] = "'" + n + "'"
	}
	script := `$names = @(` + strings.Join(quoted, ",") + `)
foreach ($n in $names) {
  Get-AppxPackage -AllUsers -Name $n -ErrorAction SilentlyContinue | ForEach-Object {
    try { Remove-AppxPackage -Package $_.PackageFullName -AllUsers -ErrorAction Stop; $_.Name } catch {}
  }
  Get-AppxProvisionedPackage -Online -ErrorAction SilentlyContinue | Where-Object DisplayName -like $n | ForEach-Object {
    Remove-AppxProvisionedPackage -Online -PackageName $_.PackageName -ErrorAction SilentlyContinue | Out-Null
  }
}`
	names, err := powershell(15*time.Minute, script)
	return step("Apps a till doesn't use removed", listOrNone(names), err)
}

// setPower keeps the till awake and fast: high performance, no sleep, no
// screen off, no hibernation (which also turns off fast startup, so a
// restart starts Windows fresh), and USB ports that never power down, which
// otherwise drops a barcode scanner or a receipt printer.
func setPower() CashMachineStep {
	windowsDir, _ := windows.GetWindowsDirectory()
	powercfg := filepath.Join(windowsDir, `System32\powercfg.exe`)
	const highPerformance = "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c"
	const usbGroup, usbSuspend = "2a737441-1930-4402-8d77-b2bebba308a3", "48e6b7a6-50f5-4782-a5d4-53bb8f07e226"
	var errs []error
	if _, err := runHidden(time.Minute, powercfg, "/setactive", highPerformance); err != nil {
		// Some makers' Windows ship without it; the current plan is kept.
		errs = append(errs, fmt.Errorf("high performance plan: %w", err))
	}
	for _, args := range [][]string{
		{"/change", "standby-timeout-ac", "0"},
		{"/change", "monitor-timeout-ac", "0"},
		{"/change", "hibernate-timeout-ac", "0"},
		{"/change", "disk-timeout-ac", "0"},
		{"/setacvalueindex", "SCHEME_CURRENT", usbGroup, usbSuspend, "0"},
		{"/setactive", "SCHEME_CURRENT"},
		{"/hibernate", "off"},
	} {
		if _, err := runHidden(time.Minute, powercfg, args...); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", strings.Join(args, " "), err))
		}
	}
	return step("Power: high performance, never sleeps", "", errors.Join(errs...))
}
