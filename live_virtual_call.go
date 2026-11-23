package main

// The separate screen's calls to Go. The hidden copy of the till app runs in
// an iframe of the same window, so Go cannot tell which copy is calling a
// binding. Everything the hidden copy does therefore goes through one
// binding, LiveVirtualCall, which dispatches to the same methods with:
//   - the hidden copy's own sign-in (virtualAuth), so an admin signing in
//     there never makes the cashier an admin, and the cashier signing in never
//     demotes the admin;
//   - a deny list for what acts on the whole till (switching branch, restart,
//     update, cash machine, the live view itself).
//
// A gated method X is split in two: X reads the cashier's sign-in and calls
// xAs(caller, ...); the table below runs xAs with the separate screen's
// sign-in. live_virtual_call_test.go fails if a gate is added without a twin.

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// virtualAuth is the separate screen's sign-in and lockout. It lives only in
// memory: a till restart ends the session and the sign-in with it.
type virtualAuth struct {
	on          bool
	role        string
	failed      int
	lockedUntil time.Time
	level       int
}

// errNotOnVirtual is what a denied call answers.
var errNotOnVirtual = errors.New("Not available on the separate screen")

// deniedOnVirtual are the bindings that act on the whole till, not on the
// screen the admin is using. Prefixes below cover the live view itself.
var deniedOnVirtual = []string{
	"SwitchBranch", "CreateBranch", "SetDefaultBranch",
	"RestartApp", "RestartComputer",
	"StartCashMachineSetup", "InstallAppUpdate",
}

var deniedVirtualPrefixes = []string{"LiveView", "LiveVirtual"}

func deniedOnVirtualName(name string) bool {
	for _, d := range deniedOnVirtual {
		if d == name {
			return true
		}
	}
	for _, p := range deniedVirtualPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// setVirtualSession is called when the separate screen opens or closes. Either
// way it starts from nobody signed in.
func (a *App) setVirtualSession(on bool) {
	a.pinMu.Lock()
	defer a.pinMu.Unlock()
	a.virtual = virtualAuth{on: on}
}

func (a *App) virtualSessionOn() bool {
	a.pinMu.Lock()
	defer a.pinMu.Unlock()
	return a.virtual.on
}

// virtualCaller is the separate screen's own sign-in, read in one step.
func (a *App) virtualCaller() caller {
	a.pinMu.Lock()
	defer a.pinMu.Unlock()
	return caller{admin: a.virtual.role == "admin", signedIn: a.virtual.role != ""}
}

func (a *App) virtualLoginLocked(now time.Time) error {
	if now.Before(a.virtual.lockedUntil) {
		seconds := int((a.virtual.lockedUntil.Sub(now) + time.Second - 1) / time.Second)
		return fmt.Errorf("locked:%d", seconds)
	}
	return nil
}

func (a *App) virtualFailedLogin(now time.Time, failMsg string) error {
	a.virtual.failed++
	if a.virtual.failed >= maxPINAttempts {
		a.virtual.failed = 0
		a.virtual.level++
		duration := pinLockoutDuration(a.virtual.level)
		a.virtual.lockedUntil = now.Add(duration)
		return fmt.Errorf("locked:%d", int(duration/time.Second))
	}
	return errors.New(failMsg)
}

func (a *App) virtualSignedIn(role string) {
	a.virtual.failed = 0
	a.virtual.lockedUntil = time.Time{}
	a.virtual.level = 0
	a.virtual.role = role
}

// virtualVerifyPIN is VerifyPIN for the separate screen: its own lockout, its
// own role, nothing persisted and no shift closed (the cashier's shift is not
// the admin's business here).
func (a *App) virtualVerifyPIN(pin string) (map[string]interface{}, error) {
	a.pinMu.Lock()
	defer a.pinMu.Unlock()
	now := time.Now()
	if err := a.virtualLoginLocked(now); err != nil {
		return nil, err
	}
	if pin != a.cashierPIN() {
		return nil, a.virtualFailedLogin(now, "invalid PIN")
	}
	a.virtualSignedIn("cashier")
	return map[string]interface{}{"id": int64(0), "username": "cashier", "role": "cashier"}, nil
}

// virtualVerifyAdminPassword is VerifyAdminPassword for the separate screen.
func (a *App) virtualVerifyAdminPassword(password string) (map[string]interface{}, error) {
	a.pinMu.Lock()
	defer a.pinMu.Unlock()
	now := time.Now()
	if err := a.virtualLoginLocked(now); err != nil {
		return nil, err
	}
	if hashPassword(password) != a.adminPasswordHash() {
		return nil, a.virtualFailedLogin(now, "wrong password")
	}
	a.virtualSignedIn("admin")
	return map[string]interface{}{"id": int64(0), "username": "admin", "role": "admin"}, nil
}

// virtualPINLockoutStatus is GetPINLockoutStatus for the separate screen's own
// lockout, so its login screen shows its own countdown.
func (a *App) virtualPINLockoutStatus() map[string]interface{} {
	a.pinMu.Lock()
	defer a.pinMu.Unlock()
	remaining := time.Until(a.virtual.lockedUntil)
	if remaining <= 0 {
		return map[string]interface{}{"locked": false, "retryAfter": 0}
	}
	return map[string]interface{}{"locked": true, "retryAfter": int((remaining + time.Second - 1) / time.Second)}
}

// virtualTwins maps a method to what the separate screen runs instead. A twin
// whose first parameter is a caller gets the separate screen's caller put
// there; the rest of its parameters must match the real method (the test
// checks that).
func (a *App) virtualTwins() map[string]any {
	return map[string]any{
		// Its own sign-in.
		"VerifyPIN":           a.virtualVerifyPIN,
		"VerifyAdminPassword": a.virtualVerifyAdminPassword,
		"GetPINLockoutStatus": a.virtualPINLockoutStatus,
		// The cashier's resume-after-update is not the admin's.
		"SaveResumeRoute":   func(string) {},
		"TakeResumeSession": func() map[string]any { return map[string]any{} },
		// The gates, run with the separate screen's sign-in.
		"VerifyManagerPassword":          a.verifyManagerPasswordAs,
		"GetConversionReport":            a.getConversionReportAs,
		"SetExpenseFromCentral":          a.setExpenseFromCentralAs,
		"CancelInvoice":                  a.cancelInvoiceAs,
		"ReplaceShiftBalance":            a.replaceShiftBalanceAs,
		"PreviewShiftBalanceReplacement": a.previewShiftBalanceReplacementAs,
		"SetQuickAccessColor":            a.setQuickAccessColorAs,
		"CancelExpense":                  a.cancelExpenseAs,
		"CancelLoan":                     a.cancelLoanAs,
		"PullBranchHistory":              a.pullBranchHistoryAs,
		"SetQuickAccessMode":             a.setQuickAccessModeAs,
	}
}

var callerType = reflect.TypeOf(caller{})

type virtualEntry struct {
	fn     reflect.Value
	caller bool // the first parameter is the caller, filled in by the dispatcher
}

// virtualTable is every exported method of *App the separate screen may call.
func (a *App) virtualTable() map[string]virtualEntry {
	twins := a.virtualTwins()
	table := map[string]virtualEntry{}
	v := reflect.ValueOf(a)
	t := v.Type()
	for i := 0; i < t.NumMethod(); i++ {
		name := t.Method(i).Name
		if deniedOnVirtualName(name) || isLifecycleMethod(name) {
			continue
		}
		fn := v.Method(i)
		if twin, ok := twins[name]; ok {
			fn = reflect.ValueOf(twin)
		}
		ft := fn.Type()
		table[name] = virtualEntry{fn: fn, caller: ft.NumIn() > 0 && ft.In(0) == callerType}
	}
	return table
}

// isLifecycleMethod is for exported methods that are Wails hooks, not bindings.
func isLifecycleMethod(name string) bool {
	switch name {
	case "Startup", "Shutdown", "DomReady", "BeforeClose":
		return true
	}
	return false
}

// virtualDispatch runs one call of the separate screen. Every failure is an
// error, never a panic.
func (a *App) virtualDispatch(method string, args []json.RawMessage) (result any, err error) {
	if !a.virtualSessionOn() {
		return nil, errors.New("the separate screen is not open")
	}
	if deniedOnVirtualName(method) {
		return nil, errNotOnVirtual
	}
	entry, ok := a.virtualTableOnce()[method]
	if !ok {
		return nil, fmt.Errorf("unknown method %q", method)
	}
	ft := entry.fn.Type()
	if ft.IsVariadic() {
		return nil, fmt.Errorf("%s: not available on the separate screen", method)
	}
	first := 0
	if entry.caller {
		first = 1
	}
	if len(args) != ft.NumIn()-first {
		return nil, fmt.Errorf("%s: want %d arguments, got %d", method, ft.NumIn()-first, len(args))
	}
	in := make([]reflect.Value, 0, ft.NumIn())
	if entry.caller {
		in = append(in, reflect.ValueOf(a.virtualCaller()))
	}
	for i, raw := range args {
		p := reflect.New(ft.In(first + i))
		if err := json.Unmarshal(raw, p.Interface()); err != nil {
			return nil, fmt.Errorf("%s: argument %d: %w", method, i+1, err)
		}
		in = append(in, p.Elem())
	}
	defer func() {
		if r := recover(); r != nil {
			result, err = nil, fmt.Errorf("%s failed: %v", method, r)
		}
	}()
	out := entry.fn.Call(in)
	return splitResults(out)
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// splitResults turns a method's results into (value, error).
func splitResults(out []reflect.Value) (any, error) {
	var value any
	for _, o := range out {
		if o.Type() == errorType {
			if !o.IsNil() {
				return nil, o.Interface().(error)
			}
			continue
		}
		value = o.Interface()
	}
	return value, nil
}

func (a *App) virtualTableOnce() map[string]virtualEntry {
	a.virtualTableMu.Lock()
	defer a.virtualTableMu.Unlock()
	if a.virtualTableCache == nil {
		a.virtualTableCache = a.virtualTable()
	}
	return a.virtualTableCache
}

// LiveVirtualCall is the one binding the separate screen's copy of the app
// uses for everything: the method's name, its arguments as a JSON array, and
// the result back as JSON. Only the hidden copy's bridge calls it.
func (a *App) LiveVirtualCall(method string, argsJSON string) (string, error) {
	var args []json.RawMessage
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("bad arguments: %w", err)
		}
	}
	value, err := a.virtualDispatch(method, args)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("%s: %w", method, err)
	}
	return string(out), nil
}
