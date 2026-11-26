package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math/rand"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	db "kashi-pos/db/sqlc"
)

// The balance-swap gates need real shift data to tell the roles apart, so they
// are checked through checkNegativeTransferAllowed (see TestNegativeTransferGate).
var gatedBalanceMethods = []string{"ReplaceShiftBalance", "PreviewShiftBalanceReplacement"}

// The gated methods: each reads who is signed in (cashier or the separate
// screen) through a caller. Keep in step with virtualTwins.
var gatedMethods = []string{
	"VerifyManagerPassword", "GetConversionReport", "SetExpenseFromCentral", "CancelInvoice",
	"SetQuickAccessColor",
	"CancelExpense", "CancelLoan", "PullBranchHistory", "SetQuickAccessMode",
}

func zeroArgs(t reflect.Type, skip int) []json.RawMessage {
	var args []json.RawMessage
	for i := skip; i < t.NumIn(); i++ {
		b, _ := json.Marshal(reflect.Zero(t.In(i)).Interface())
		args = append(args, b)
	}
	return args
}

// outcome of calling a gated method directly (the cashier's screen) with
// zero arguments, as text, so two outcomes can be compared.
func directOutcome(app *App, name string) (out string) {
	defer func() {
		if r := recover(); r != nil {
			out = fmt.Sprint("panic: ", r)
		}
	}()
	m := reflect.ValueOf(app).MethodByName(name)
	in := make([]reflect.Value, m.Type().NumIn())
	for i := range in {
		in[i] = reflect.Zero(m.Type().In(i))
	}
	return outcomeText(m.Call(in))
}

func outcomeText(res []reflect.Value) string {
	var parts []string
	for _, r := range res {
		if r.Type() == errorType {
			if r.IsNil() {
				parts = append(parts, "nil")
			} else {
				parts = append(parts, "err:"+r.Interface().(error).Error())
			}
			continue
		}
		parts = append(parts, "val")
	}
	return strings.Join(parts, ",")
}

func virtualOutcome(app *App, name string) string {
	entry, ok := app.virtualTable()[name]
	if !ok {
		return "missing"
	}
	skip := 0
	if entry.caller {
		skip = 1
	}
	_, err := app.virtualDispatch(name, zeroArgs(entry.fn.Type(), skip))
	// Same shape as directOutcome: only the error decides, except a value.
	if err != nil {
		return "err:" + err.Error()
	}
	return "ok"
}

// errPart is the error text of an outcome ("nil"/"val" parts are not errors).
func errPart(outcome string) string {
	for _, p := range strings.Split(outcome, ",") {
		if strings.HasPrefix(p, "err:") || strings.HasPrefix(p, "panic") {
			return p
		}
	}
	return "none"
}

func gateErr(app *App, name string) string { return errPart(directOutcome(app, name)) }

func virtualGateErr(app *App, name string) string {
	r := virtualOutcome(app, name)
	if r == "ok" {
		return "none"
	}
	return r
}

// newTestApp is an App on a migrated test database, so a gate answers with
// its own refusal instead of "database not initialized".
func newTestApp(t *testing.T) *App {
	t.Helper()
	ctx := context.Background()
	database := openTestDatabase(t)
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	return &App{ctx: ctx, store: db.NewStore(database), conn: database}
}

func newAppWithRole(t *testing.T, role string) *App {
	t.Helper()
	app := newTestApp(t)
	switch role {
	case "admin":
		if _, err := app.VerifyAdminPassword("1963"); err != nil {
			t.Fatal(err)
		}
	case "cashier":
		if _, err := app.VerifyPIN("1478"); err != nil {
			t.Fatal(err)
		}
	}
	return app
}

func virtualLogin(t *testing.T, app *App, role string) {
	t.Helper()
	var err error
	if role == "admin" {
		_, err = app.virtualDispatch("VerifyAdminPassword", []json.RawMessage{json.RawMessage(`"1963"`)})
	} else {
		_, err = app.virtualDispatch("VerifyPIN", []json.RawMessage{json.RawMessage(`"1478"`)})
	}
	if err != nil {
		t.Fatalf("virtual login as %s: %v", role, err)
	}
}

// The gates have to tell the cashier and the admin apart for this test to mean
// anything: same zero arguments, different answer.
func TestGatesTellCashierAndAdminApart(t *testing.T) {
	cashier, admin := newAppWithRole(t, "cashier"), newAppWithRole(t, "admin")
	for _, name := range gatedMethods {
		if gateErr(cashier, name) == gateErr(admin, name) {
			t.Errorf("%s answers the same for a cashier and an admin (%s): the test cannot tell them apart", name, gateErr(cashier, name))
		}
	}
}

// The reported case: an admin signs in on the separate screen while a cashier
// is signed in at the till. Every gate still refuses the cashier, and the
// separate screen's calls are allowed.
func TestAdminOnSeparateScreenDoesNotMakeCashierAdmin(t *testing.T) {
	asCashier, asAdmin := newAppWithRole(t, "cashier"), newAppWithRole(t, "admin")
	app := newAppWithRole(t, "cashier")
	app.setVirtualSession(true)
	virtualLogin(t, app, "admin")

	for _, name := range gatedMethods {
		if got, want := gateErr(app, name), gateErr(asCashier, name); got != want {
			t.Errorf("%s: the cashier now gets %q, want %q", name, got, want)
		}
		if got, want := virtualGateErr(app, name), gateErr(asAdmin, name); got != want {
			t.Errorf("%s: the separate screen's admin gets %q, want %q", name, got, want)
		}
	}
	// The cashier's own sign-in did not change.
	if c := app.cashierCaller(); c.admin || !c.signedIn {
		t.Fatalf("cashier caller changed: %+v", c)
	}
}

// The other way round: the cashier signs in again (or someone is admin at the
// till) while the separate screen has its own sign-in.
func TestCashierSigningInDoesNotDemoteSeparateScreen(t *testing.T) {
	asCashier, asAdmin := newAppWithRole(t, "cashier"), newAppWithRole(t, "admin")
	app := newAppWithRole(t, "admin")
	app.setVirtualSession(true)
	virtualLogin(t, app, "cashier")
	for _, name := range gatedMethods {
		if got, want := gateErr(app, name), gateErr(asAdmin, name); got != want {
			t.Errorf("%s: the till admin gets %q, want %q", name, got, want)
		}
		if got, want := virtualGateErr(app, name), gateErr(asCashier, name); got != want {
			t.Errorf("%s: the separate screen's cashier gets %q, want %q", name, got, want)
		}
	}

	// Admin on the separate screen, the cashier signs in at the till again.
	virtualLogin(t, app, "admin")
	if _, err := app.VerifyPIN("1478"); err != nil {
		t.Fatal(err)
	}
	if !app.virtualCaller().admin {
		t.Fatal("the cashier signing in demoted the separate screen's admin")
	}
	if app.cashierCaller().admin {
		t.Fatal("the till became admin")
	}
}

func TestSeparateScreenStartsSignedOutAndEndsSignedOut(t *testing.T) {
	app := newAppWithRole(t, "admin")
	app.setVirtualSession(true)
	if c := app.virtualCaller(); c.admin || c.signedIn {
		t.Fatalf("a new separate screen must start signed out, got %+v", c)
	}
	virtualLogin(t, app, "admin")
	app.setVirtualSession(false)
	if c := app.virtualCaller(); c.admin || c.signedIn {
		t.Fatalf("closing the separate screen must sign it out, got %+v", c)
	}
	app.setVirtualSession(true)
	if app.virtualCaller().signedIn {
		t.Fatal("a reopened separate screen kept the old sign-in")
	}
}

func TestLockoutsAreSeparate(t *testing.T) {
	app := newTestApp(t)
	app.setVirtualSession(true)
	wrong := []json.RawMessage{json.RawMessage(`"0000"`)}
	var last error
	for i := 0; i < maxPINAttempts; i++ {
		_, last = app.virtualDispatch("VerifyPIN", wrong)
	}
	if last == nil || !strings.HasPrefix(last.Error(), "locked:") {
		t.Fatalf("separate screen should be locked, got %v", last)
	}
	// The till is not locked out by the separate screen's wrong PINs.
	if _, err := app.VerifyPIN("1478"); err != nil {
		t.Fatalf("till locked by the separate screen: %v", err)
	}
	if st := app.GetPINLockoutStatus(); st["locked"] != false {
		t.Fatalf("till shows locked: %v", st)
	}
	// And the other way round.
	app2 := NewApp()
	app2.setVirtualSession(true)
	for i := 0; i < maxPINAttempts; i++ {
		_, _ = app2.VerifyPIN("0000")
	}
	virtualLogin(t, app2, "cashier")
	res, err := app2.virtualDispatch("GetPINLockoutStatus", nil)
	if err != nil || res.(map[string]interface{})["locked"] != false {
		t.Fatalf("separate screen shows the till's lockout: %v %v", res, err)
	}
}

func TestVirtualLoginsDoNotPersistOrTouchTheTill(t *testing.T) {
	app := newTestApp(t)
	app.setVirtualSession(true)
	_, _ = app.virtualDispatch("VerifyPIN", []json.RawMessage{json.RawMessage(`"0000"`)})
	app.pinMu.Lock()
	failed, role := app.failedPINAttempts, app.currentRole
	app.pinMu.Unlock()
	if failed != 0 || role != "" {
		t.Fatalf("separate screen's attempt changed the till: failed %d role %q", failed, role)
	}
}

func TestDispatcherErrors(t *testing.T) {
	app := newTestApp(t)
	raw := func(s string) json.RawMessage { return json.RawMessage(s) }

	if _, err := app.virtualDispatch("GetPINLockoutStatus", nil); err == nil {
		t.Fatal("a call with no separate screen open must be refused")
	}
	app.setVirtualSession(true)
	if _, err := app.virtualDispatch("NoSuchMethod", nil); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("unknown method: %v", err)
	}
	if _, err := app.virtualDispatch("VerifyPIN", nil); err == nil {
		t.Fatal("missing argument accepted")
	}
	if _, err := app.virtualDispatch("VerifyPIN", []json.RawMessage{raw(`"a"`), raw(`"b"`)}); err == nil {
		t.Fatal("extra argument accepted")
	}
	if _, err := app.virtualDispatch("VerifyPIN", []json.RawMessage{raw(`5`)}); err == nil {
		t.Fatal("wrong argument type accepted")
	}
	if _, err := app.virtualDispatch("CancelExpense", []json.RawMessage{raw(`"x"`)}); err == nil {
		t.Fatal("wrong argument type accepted for a gated method")
	}
	// The binding wraps the same thing.
	if _, err := app.LiveVirtualCall("VerifyPIN", `not json`); err == nil {
		t.Fatal("bad argument JSON accepted")
	}
	out, err := app.LiveVirtualCall("VerifyPIN", `["1478"]`)
	if err != nil || !strings.Contains(out, `"role":"cashier"`) {
		t.Fatalf("binding result %q %v", out, err)
	}
	if _, err := app.LiveVirtualCall("VerifyPIN", `["0"]`); err == nil || err.Error() != "invalid PIN" {
		t.Fatalf("a failed call must come back as an error: %v", err)
	}
}

func TestDeniedOnSeparateScreen(t *testing.T) {
	app := newTestApp(t)
	app.setVirtualSession(true)
	typ := reflect.TypeOf(app)
	for _, name := range deniedOnVirtual {
		if _, ok := typ.MethodByName(name); !ok {
			t.Errorf("deny list names %s, which is not a method of App (renamed?)", name)
		}
		if _, err := app.virtualDispatch(name, nil); err == nil || err.Error() != errNotOnVirtual.Error() {
			t.Errorf("%s was not refused on the separate screen: %v", name, err)
		}
	}
	var live int
	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name
		if strings.HasPrefix(name, "LiveView") || strings.HasPrefix(name, "LiveVirtual") {
			live++
			if _, err := app.virtualDispatch(name, nil); err == nil || err.Error() != errNotOnVirtual.Error() {
				t.Errorf("%s was not refused on the separate screen: %v", name, err)
			}
		}
	}
	if live == 0 {
		t.Fatal("no live view bindings found: the prefix list is stale")
	}
	table := app.virtualTable()
	for _, name := range append([]string{"Startup", "Shutdown", "DomReady", "BeforeClose"}, deniedOnVirtual...) {
		if _, ok := table[name]; ok {
			t.Errorf("%s must not be in the separate screen's table", name)
		}
	}
}

// A twin must take what the real method takes and give what it gives, or the
// separate screen would behave differently from the till in more than who is
// signed in.
func TestTwinsMatchTheirMethods(t *testing.T) {
	app := newTestApp(t)
	typ := reflect.TypeOf(app)
	for name, twin := range app.virtualTwins() {
		real, ok := typ.MethodByName(name)
		if !ok {
			t.Errorf("twin %s has no method on App (renamed?)", name)
			continue
		}
		tt := reflect.TypeOf(twin)
		skip := 0
		if tt.NumIn() > 0 && tt.In(0) == callerType {
			skip = 1
		}
		// real.Type has the receiver first.
		if tt.NumIn()-skip != real.Type.NumIn()-1 {
			t.Errorf("twin %s takes %d arguments, App.%s takes %d", name, tt.NumIn()-skip, name, real.Type.NumIn()-1)
			continue
		}
		for i := 0; i < tt.NumIn()-skip; i++ {
			if tt.In(i+skip) != real.Type.In(i+1) {
				t.Errorf("twin %s argument %d is %v, App.%s has %v", name, i+1, tt.In(i+skip), name, real.Type.In(i+1))
			}
		}
		if tt.NumOut() != real.Type.NumOut() {
			t.Errorf("twin %s returns %d values, App.%s returns %d", name, tt.NumOut(), name, real.Type.NumOut())
			continue
		}
		for i := 0; i < tt.NumOut(); i++ {
			if tt.Out(i) != real.Type.Out(i) {
				t.Errorf("twin %s result %d is %v, App.%s has %v", name, i+1, tt.Out(i), name, real.Type.Out(i))
			}
		}
	}
	// Every gated method has its twin, and no twin is missing from the list.
	twins := app.virtualTwins()
	for _, name := range append(append([]string{}, gatedMethods...), gatedBalanceMethods...) {
		if _, ok := twins[name]; !ok {
			t.Errorf("gated method %s has no twin", name)
		}
	}
}

// A new gate added later must come with a twin: whoever reads currentRole or
// calls currentUserIsAdmin / anyoneSignedInThisSession outside the places
// that are meant to fails here, and so does a call to cashierCaller from
// anything but a method that has a twin.
func TestGuardNoGateSkipsItsTwin(t *testing.T) {
	fset := token.NewFileSet()
	files, err := parseNonTestFiles(fset)
	if err != nil {
		t.Fatal(err)
	}
	// Where the cashier's own sign-in may be read directly.
	readsSignIn := map[string]bool{
		"currentUserIsAdmin": true, "anyoneSignedInThisSession": true, "cashierCaller": true,
		"VerifyPIN": true, "VerifyAdminPassword": true, "currentRoleNow": true, "TakeResumeSession": true,
		// The branch management methods are denied on the separate screen.
		"CreateBranch": true, "SwitchBranch": true, "SetDefaultBranch": true,
	}
	twinned := map[string]bool{}
	for _, name := range append(append([]string{}, gatedMethods...), gatedBalanceMethods...) {
		twinned[name] = true
	}
	denied := map[string]bool{}
	for _, name := range deniedOnVirtual {
		denied[name] = true
	}
	var problems []string
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := fn.Name.Name
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.SelectorExpr:
					switch x.Sel.Name {
					case "currentUserIsAdmin", "anyoneSignedInThisSession", "currentRole":
						if !readsSignIn[name] {
							problems = append(problems, fmt.Sprintf("%s reads the cashier's sign-in directly (%s): take a caller and add a twin", name, x.Sel.Name))
						}
					case "cashierCaller":
						if name == "cashierCaller" {
							break
						}
						// Only the exported method that has a twin may read it.
						if !twinned[name] {
							problems = append(problems, fmt.Sprintf("%s calls cashierCaller but has no twin in gatedMethods", name))
						}
					}
				}
				return true
			})
		}
	}
	// The denied methods that read the sign-in must really be denied.
	for _, name := range []string{"CreateBranch", "SwitchBranch", "SetDefaultBranch"} {
		if !denied[name] {
			problems = append(problems, name+" reads the sign-in directly but is not on the deny list")
		}
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		t.Fatal(strings.Join(problems, "\n"))
	}
}

// The rule behind the two balance-swap gates: only an admin takes a balance
// below zero. Each side is judged by its own sign-in.
func TestNegativeTransferGate(t *testing.T) {
	app := newAppWithRole(t, "cashier")
	app.setVirtualSession(true)
	virtualLogin(t, app, "admin")
	below := shiftBalanceReplacementCheck{SourceRemainingCents: -1}
	ok := shiftBalanceReplacementCheck{SourceRemainingCents: 0}

	if err := app.checkNegativeTransferAllowed(app.cashierCaller(), below); err == nil {
		t.Fatal("the cashier took a balance below zero because an admin is on the separate screen")
	}
	if err := app.checkNegativeTransferAllowed(app.virtualCaller(), below); err != nil {
		t.Fatalf("the separate screen's admin was refused: %v", err)
	}
	for _, c := range []caller{app.cashierCaller(), app.virtualCaller()} {
		if err := app.checkNegativeTransferAllowed(c, ok); err != nil {
			t.Fatalf("a transfer that stays at zero or above must pass: %v", err)
		}
	}
}

// Randomised: cashier and separate-screen logins and gate reads interleaved
// from many goroutines (run with -race). Each side must only ever see its own
// role. No database: logins and gates here only touch the in-memory sign-in.
func TestInterleavedSignInsKeepRolesApart(t *testing.T) {
	below := shiftBalanceReplacementCheck{SourceRemainingCents: -1}
	for seed := int64(1); seed <= 40; seed++ {
		app := NewApp()
		app.setVirtualSession(true)
		r0 := rand.New(rand.NewSource(seed))
		tillAdmin, screenAdmin := r0.Intn(2) == 0, r0.Intn(2) == 0
		login := func(side string) {
			if side == "till" {
				if tillAdmin {
					_, _ = app.VerifyAdminPassword("1963")
				} else {
					_, _ = app.VerifyPIN("1478")
				}
				return
			}
			if screenAdmin {
				_, _ = app.virtualDispatch("VerifyAdminPassword", []json.RawMessage{json.RawMessage(`"1963"`)})
			} else {
				_, _ = app.virtualDispatch("VerifyPIN", []json.RawMessage{json.RawMessage(`"1478"`)})
			}
		}
		login("till")
		login("screen")
		var wg sync.WaitGroup
		errs := make(chan string, 1000)
		for g := 0; g < 8; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				r := rand.New(rand.NewSource(seed*100 + int64(g)))
				for i := 0; i < 60; i++ {
					switch r.Intn(4) {
					case 0:
						login("till")
					case 1:
						login("screen")
					case 2:
						c := app.cashierCaller()
						if c.admin != tillAdmin || !c.signedIn {
							errs <- fmt.Sprintf("till: got %+v, admin should be %v", c, tillAdmin)
						}
						if err := app.checkNegativeTransferAllowed(c, below); (err == nil) != tillAdmin {
							errs <- fmt.Sprintf("till gate: err %v, admin should be %v", err, tillAdmin)
						}
					case 3:
						c := app.virtualCaller()
						if c.admin != screenAdmin || !c.signedIn {
							errs <- fmt.Sprintf("screen: got %+v, admin should be %v", c, screenAdmin)
						}
						if err := app.checkNegativeTransferAllowed(c, below); (err == nil) != screenAdmin {
							errs <- fmt.Sprintf("screen gate: err %v, admin should be %v", err, screenAdmin)
						}
					}
				}
			}(g)
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			t.Error(e)
		}
	}
}

func parseNonTestFiles(fset *token.FileSet) ([]*ast.File, error) {
	entries, err := filepath.Glob("*.go")
	if err != nil {
		return nil, err
	}
	var files []*ast.File
	for _, path := range entries {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}
