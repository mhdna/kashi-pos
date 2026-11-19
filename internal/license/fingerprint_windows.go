//go:build windows

package license

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	goruntime "runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// createNoWindow prevents the console flash that a visible cmd.exe/powershell
// window would otherwise cause every time this process shells out.
const createNoWindow = 0x08000000

// wmiTimeout bounds the PowerShell/WMI shell-out in wmiValues. A few
// seconds is generous for what is normally a sub-second query, while still
// turning a hung WMI repository into a fast, recoverable failure (see
// checkLicense's grace path for an already-activated machine) instead of an
// indefinite hang of the whole app startup.
const wmiTimeout = 5 * time.Second

var (
	fingerprintMu     sync.Mutex
	fingerprintCached string
	fingerprintReady  bool
)

// Fingerprint derives a stable machine identity from motherboard/BIOS/CPU
// identifiers via WMI, deliberately avoiding MachineGuid (regenerated on
// every Windows reinstall) and disk serials (change on drive replacement).
// A full OS reinstall or drive swap on the SAME physical machine keeps
// producing the same fingerprint; only an actual motherboard swap changes
// it, which is handled manually by the vendor via AOM Ledger.
//
// A successful result is cached for the rest of the process's lifetime:
// callers such as GetLicenseStatus are invoked on every frontend route
// change, and each underlying call previously shelled out to three separate
// PowerShell processes, which is both slow and (pre-CREATE_NO_WINDOW)
// visibly flashed console windows. A *failure* is deliberately not cached -
// a transient WMI hiccup (service restarting after a Windows update, a
// momentary PowerShell/AV interference) would otherwise report an invalid
// license for the rest of the process's life instead of recovering on the
// next call.
func Fingerprint() (string, error) {
	fingerprintMu.Lock()
	defer fingerprintMu.Unlock()
	if fingerprintReady {
		return fingerprintCached, nil
	}
	fp, err := computeFingerprintFn()
	if err != nil {
		return "", err
	}
	fingerprintCached, fingerprintReady = fp, true
	return fingerprintCached, nil
}

// computeFingerprintFn is a test seam: fingerprint_windows_test.go swaps this
// out to simulate WMI failures/recoveries without actually shelling out.
var computeFingerprintFn = computeFingerprint

// fingerprintFallbacks names, for each primary identifier, a second WMI
// property to fall back to when the primary comes back blank (common on
// OEM boards - e.g. many Lenovo systems never stamp Win32_BaseBoard's
// SerialNumber). The fallback is only ever consulted when the primary is
// empty, so a machine where all three primaries already resolve keeps
// producing the exact same hash it always has - no already-activated
// machine can be invalidated by this change.
var fingerprintFallbacks = map[string]string{
	"board": "board_fallback",
	"bios":  "bios_fallback",
	"cpu":   "cpu_fallback",
}

func computeFingerprint() (string, error) {
	out, err := wmiValues(map[string]string{
		"board":          "Win32_BaseBoard.SerialNumber",
		"board_fallback": "Win32_BaseBoard.Product",
		"bios":           "Win32_BIOS.SerialNumber",
		"bios_fallback":  "Win32_BIOS.SMBIOSBIOSVersion",
		"cpu":            "Win32_Processor.ProcessorId",
		"cpu_fallback":   "Win32_Processor.Name",
	})
	if err != nil {
		return "", fmt.Errorf("read machine identifiers: %w", err)
	}
	return fingerprintFromValues(out)
}

// fingerprintFromValues hashes the three resolved identifiers into the final
// fingerprint. Split out from computeFingerprint so the empty-value/fallback
// resolution can be unit tested without shelling out to real WMI.
func fingerprintFromValues(out map[string]string) (string, error) {
	resolved := make(map[string]string, 3)
	// A query that exits 0 but returns a blank value for one of these
	// properties (seen in practice right after certain driver/firmware
	// updates, or permanently on OEM boards that never set it) is a read
	// failure, not a genuinely different machine - if treated as data, it
	// hashes into a validly-different fingerprint that is indistinguishable
	// from real tampering to the user. Fall back to a secondary property for
	// that same slot before giving up.
	for _, key := range []string{"board", "bios", "cpu"} {
		value := strings.TrimSpace(out[key])
		if value == "" {
			value = strings.TrimSpace(out[fingerprintFallbacks[key]])
		}
		if value == "" {
			return "", fmt.Errorf("read machine identifiers: %s value is empty (fallback also unavailable)", key)
		}
		resolved[key] = value
	}

	raw := strings.Join([]string{resolved["board"], resolved["bios"], resolved["cpu"]}, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:]), nil
}

// wmiValues reads several WMI Class.Property values, querying WMI in-process
// over COM first. Starting powershell.exe (a cold .NET load, often scanned by
// antivirus) and running a Get-CimInstance per value used to take several
// seconds on some Windows machines, all of it behind the boot overlay on
// every launch. Both paths read the same WMI provider, so they return the
// same values and therefore the same fingerprint; PowerShell is kept only as
// a fallback for a machine where the COM query itself fails.
func wmiValues(queries map[string]string) (map[string]string, error) {
	out, err := wmiValuesCOM(queries)
	if err == nil {
		return out, nil
	}
	if strings.Contains(err.Error(), "timed out") {
		// A hung WMI repository would hang PowerShell's query too; don't
		// spend a second timeout on it.
		return nil, err
	}
	out, psErr := wmiValuesPowerShell(queries)
	if psErr != nil {
		return nil, fmt.Errorf("%w (in-process WMI query also failed: %v)", psErr, err)
	}
	return out, nil
}

// wmiValuesCOM queries WMI through SWbemLocator in-process, one query per
// class. Like the PowerShell path's Select-Object -First 1, only the first
// instance of each class is read. COM is initialized on a dedicated,
// locked OS thread so it never conflicts with how Wails initialized COM on
// its own threads.
func wmiValuesCOM(queries map[string]string) (map[string]string, error) {
	type result struct {
		values map[string]string
		err    error
	}
	done := make(chan result, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				done <- result{err: fmt.Errorf("in-process WMI query panicked: %v", recovered)}
			}
		}()
		goruntime.LockOSThread()
		defer goruntime.UnlockOSThread()
		values, err := queryWMI(queries)
		done <- result{values: values, err: err}
	}()
	select {
	case r := <-done:
		return r.values, r.err
	case <-time.After(wmiTimeout):
		return nil, fmt.Errorf("timed out after %s waiting for WMI", wmiTimeout)
	}
}

func queryWMI(queries map[string]string) (map[string]string, error) {
	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		// S_FALSE means COM was already initialized on this thread, which
		// still needs the matching CoUninitialize below.
		var oleErr *ole.OleError
		if !errors.As(err, &oleErr) || oleErr.Code() != comSFalse {
			return nil, fmt.Errorf("initialize COM: %w", err)
		}
	}
	defer ole.CoUninitialize()

	locator, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return nil, fmt.Errorf("create WMI locator: %w", err)
	}
	defer locator.Release()
	locatorDispatch, err := locator.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("query WMI locator: %w", err)
	}
	defer locatorDispatch.Release()
	serviceVariant, err := oleutil.CallMethod(locatorDispatch, "ConnectServer")
	if err != nil {
		return nil, fmt.Errorf("connect to WMI: %w", err)
	}
	defer serviceVariant.Clear()
	service := serviceVariant.ToIDispatch()

	// Group keys by class so each class is queried once.
	byClass := make(map[string]map[string]string)
	for key, classProperty := range queries {
		class, property, _ := strings.Cut(classProperty, ".")
		if byClass[class] == nil {
			byClass[class] = make(map[string]string)
		}
		byClass[class][key] = property
	}

	result := make(map[string]string, len(queries))
	for class, properties := range byClass {
		if err := queryWMIClass(service, class, properties, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// comSFalse is S_FALSE, which CoInitializeEx returns when COM was already
// initialized on the calling thread.
const comSFalse = 1

func queryWMIClass(service *ole.IDispatch, class string, properties map[string]string, result map[string]string) error {
	names := make([]string, 0, len(properties))
	for _, property := range properties {
		names = append(names, property)
	}
	setVariant, err := oleutil.CallMethod(service, "ExecQuery", "SELECT "+strings.Join(names, ", ")+" FROM "+class)
	if err != nil {
		return fmt.Errorf("query %s: %w", class, err)
	}
	defer setVariant.Clear()
	set := setVariant.ToIDispatch()

	countVariant, err := oleutil.GetProperty(set, "Count")
	if err != nil {
		return fmt.Errorf("count %s: %w", class, err)
	}
	count := countVariant.Val
	_ = countVariant.Clear()
	if count == 0 {
		// No instance: every value is blank, as PowerShell would have
		// printed it; fingerprintFromValues decides whether that's fatal.
		for key := range properties {
			result[key] = ""
		}
		return nil
	}

	itemVariant, err := oleutil.CallMethod(set, "ItemIndex", 0)
	if err != nil {
		return fmt.Errorf("read %s: %w", class, err)
	}
	defer itemVariant.Clear()
	item := itemVariant.ToIDispatch()
	for key, property := range properties {
		valueVariant, err := oleutil.GetProperty(item, property)
		if err != nil {
			return fmt.Errorf("read %s.%s: %w", class, property, err)
		}
		switch value := valueVariant.Value().(type) {
		case nil:
			result[key] = ""
		case string:
			result[key] = value
		default:
			result[key] = fmt.Sprint(value)
		}
		_ = valueVariant.Clear()
	}
	return nil
}

// wmiValuesPowerShell fetches several WMI Class.Property values in a single
// PowerShell invocation rather than shelling out once per value, and hides
// the console window that would otherwise flash on screen.
func wmiValuesPowerShell(queries map[string]string) (map[string]string, error) {
	var script strings.Builder
	for key, classProperty := range queries {
		parts := strings.SplitN(classProperty, ".", 2)
		fmt.Fprintf(&script, "Write-Output ('%s=' + (Get-CimInstance -ClassName %s | Select-Object -First 1 -ExpandProperty %s));",
			key, parts[0], parts[1])
	}

	// A hung or corrupted WMI repository is a known Windows failure mode,
	// and Fingerprint() holds fingerprintMu for the duration of this call -
	// with no deadline, a hang here used to hang the app's entire startup
	// (the router awaits the license check behind the boot overlay) rather
	// than just this one fingerprint read failing.
	ctx, cancel := context.WithTimeout(context.Background(), wmiTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script.String())
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("timed out after %s waiting for WMI", wmiTimeout)
		}
		// exec.Output populates ExitError.Stderr (up to 512 bytes) whenever
		// Stderr isn't otherwise redirected. Surfacing it turns "exit status
		// 1" into an actionable cause (blocked by AV/policy, WMI repository
		// corrupted, access denied) instead of a dead end in support logs.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}

	result := make(map[string]string, len(queries))
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		result[key] = value
	}
	return result, nil
}
