package main

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"kashi-pos/internal/license"
)

// Tests open plain databases with no activation; licensing has its own tests
// below, which turn it on where they need it.
func init() { licenseRequired = false }

var licenseTestSeed = []byte("kashi-license-golden-test-seed!!")

// fakeLicenseServer behaves like kashi-license for one product key.
type fakeLicenseServer struct {
	mu     sync.Mutex
	priv   ed25519.PrivateKey
	now    time.Time
	status string // on / off
	days   int
	down   bool
	forget bool // answer unknown_install
	key    string
	fp     string
	secret string
	dbKey  string
	checks int
	srv    *httptest.Server
}

func newFakeLicenseServer(t *testing.T) *fakeLicenseServer {
	f := &fakeLicenseServer{priv: ed25519.NewKeyFromSeed(licenseTestSeed), now: t0License, status: "on", days: 4, key: "KASHI-GOOD", dbKey: strings.Repeat("ab", 32)}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

var t0License = time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)

func (f *fakeLicenseServer) lease(fp string) license.Signed {
	l := license.Lease{V: license.Version, InstallID: "11112222-3333-4444-5555-666677778888", Fingerprint: fp, Status: f.status, LicensedTo: "Grand", IssuedAt: f.now, CheckEverySeconds: 60}
	if f.status == "on" {
		l.ValidUntil = f.now.Add(time.Duration(f.days) * 24 * time.Hour)
	} else {
		l.ValidUntil, l.CheckEverySeconds = f.now, 30
	}
	s, _ := license.Sign(f.priv, l)
	return s
}

func (f *fakeLicenseServer) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	var req map[string]string
	_ = json.NewDecoder(r.Body).Decode(&req)
	fail := func(status int, code string) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
	}
	switch r.URL.Path {
	case "/v1/activate":
		if req["productKey"] != f.key {
			fail(404, "invalid_key")
			return
		}
		if f.status != "on" {
			fail(403, "off")
			return
		}
		f.fp, f.secret = req["fingerprint"], "secret-"+req["fingerprint"]
		_ = json.NewEncoder(w).Encode(license.ActivateResponse{InstallID: "11112222-3333-4444-5555-666677778888", InstallSecret: f.secret, DBKey: f.dbKey, Lease: f.lease(f.fp)})
	case "/v1/checkin":
		f.checks++
		if f.forget || req["installSecret"] != f.secret {
			fail(401, "unknown_install")
			return
		}
		if req["fingerprint"] != f.fp {
			fail(403, "wrong_machine")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"lease": f.lease(f.fp)})
	}
}

func (f *fakeLicenseServer) set(fn func(*fakeLicenseServer)) {
	f.mu.Lock()
	fn(f)
	f.mu.Unlock()
}

type licenseRig struct {
	t       *testing.T
	srv     *fakeLicenseServer
	m       *licenseManager
	now     time.Time
	fp      string
	changes []string
}

func newLicenseRig(t *testing.T) *licenseRig {
	t.Helper()
	r := &licenseRig{t: t, srv: newFakeLicenseServer(t), now: t0License, fp: "fp-till-1"}
	r.m = r.manager(t.TempDir())
	return r
}

func (r *licenseRig) manager(dir string) *licenseManager {
	m, err := newLicenseManager(dir)
	if err != nil {
		r.t.Fatal(err)
	}
	m.pub = ed25519.NewKeyFromSeed(licenseTestSeed).Public().(ed25519.PublicKey)
	m.client = license.NewClient(r.srv.srv.URL)
	m.now = func() time.Time { return r.now }
	m.fingerprint = func() (string, error) { return r.fp, nil }
	m.onChange = func(s LicenseStatus) { r.changes = append(r.changes, s.State) }
	return m
}

func (r *licenseRig) checkIn() string {
	r.m.checkIn(context.Background())
	return r.m.Status().State
}

func (r *licenseRig) activate() {
	r.t.Helper()
	if err := r.m.load(); !errors.Is(err, errLicenseNotActivated) {
		r.t.Fatalf("fresh PC load = %v", err)
	}
	if s := r.m.Status(); s.State != licenseActivate {
		r.t.Fatalf("fresh PC state = %q", s.State)
	}
	if err := r.m.activate(context.Background(), "KASHI-GOOD", "TILL-1"); err != nil {
		r.t.Fatal(err)
	}
}

func TestLicenseActivationThenRuns(t *testing.T) {
	r := newLicenseRig(t)
	r.activate()
	s := r.m.Status()
	if s.State != licenseOK || s.Code != "11112222" || s.LicensedTo != "Grand" || s.WarnHours != 0 {
		t.Fatalf("status %+v", s)
	}
	if r.m.DBKey() != r.srv.dbKey {
		t.Fatal("database key not kept")
	}
	// Next launch on the same PC: no key asked, runs offline from the file.
	r.srv.set(func(f *fakeLicenseServer) { f.down = true })
	r.now = r.now.Add(time.Hour)
	again := r.manager(r.m.path[:len(r.m.path)-len("/license.dat")])
	if err := again.load(); err != nil {
		t.Fatalf("relaunch load = %v", err)
	}
	if again.Status().State != licenseOK || again.DBKey() != r.srv.dbKey {
		t.Fatalf("relaunch %+v", again.Status())
	}
}

func TestLicenseActivationErrors(t *testing.T) {
	r := newLicenseRig(t)
	_ = r.m.load()
	ctx := context.Background()
	if err := r.m.activate(ctx, "KASHI-TYPO", "h"); err == nil || err.Error() != "license:invalid_key" {
		t.Fatalf("typo = %v", err)
	}
	r.srv.set(func(f *fakeLicenseServer) { f.status = "off" })
	if err := r.m.activate(ctx, "KASHI-GOOD", "h"); err == nil || err.Error() != "license:off" {
		t.Fatalf("customer off = %v", err)
	}
	r.srv.set(func(f *fakeLicenseServer) { f.status = "on"; f.down = true })
	if err := r.m.activate(ctx, "KASHI-GOOD", "h"); err == nil || err.Error() != "license:offline" {
		t.Fatalf("server down = %v", err)
	}
	if r.m.Status().State != licenseActivate {
		t.Fatal("a failed activation left the till in another state")
	}
	// A server (or a man in the middle) answering with a lease not signed by
	// the vendor key: refused, nothing saved.
	r.srv.set(func(f *fakeLicenseServer) { f.down = false; _, f.priv, _ = ed25519.GenerateKey(nil) })
	if err := r.m.activate(ctx, "KASHI-GOOD", "h"); err == nil || err.Error() != "license:server_error" {
		t.Fatalf("forged lease = %v", err)
	}
	if r.m.Status().State != licenseActivate || r.m.DBKey() != "" {
		t.Fatal("a forged activation was kept")
	}
}

func TestLicenseTurnOffLocksAndTurnOnUnlocks(t *testing.T) {
	r := newLicenseRig(t)
	r.activate()
	r.srv.set(func(f *fakeLicenseServer) { f.status = "off" })
	if got := r.checkIn(); got != licenseLocked {
		t.Fatalf("after turn off = %q", got)
	}
	if r.m.interval() != 30*time.Second {
		t.Fatalf("a locked till retries every %v", r.m.interval())
	}
	// Even with the server unreachable now, the off lease keeps it locked.
	r.srv.set(func(f *fakeLicenseServer) { f.down = true })
	if got := r.checkIn(); got != licenseLocked {
		t.Fatalf("locked, then offline = %q", got)
	}
	r.srv.set(func(f *fakeLicenseServer) { f.down = false; f.status = "on" })
	if got := r.checkIn(); got != licenseOK {
		t.Fatalf("after turn on = %q", got)
	}
	if strings.Join(r.changes, ",") != "locked,ok" {
		t.Fatalf("screen told %v", r.changes)
	}
}

func TestLicenseOfflineDaysThenLocks(t *testing.T) {
	r := newLicenseRig(t)
	r.activate()
	r.srv.set(func(f *fakeLicenseServer) { f.down = true })
	r.now = r.now.Add(47 * time.Hour)
	if got := r.checkIn(); got != licenseOK || r.m.Status().WarnHours != 0 {
		t.Fatalf("2 days offline = %q %+v", got, r.m.Status())
	}
	r.now = r.now.Add(25 * time.Hour) // 72 h
	if got := r.checkIn(); got != licenseOK || r.m.Status().WarnHours != 24 {
		t.Fatalf("3 days offline = %q warn %d", got, r.m.Status().WarnHours)
	}
	r.now = t0License.Add(96 * time.Hour)
	if got := r.checkIn(); got != licenseLocked {
		t.Fatalf("4 days offline = %q", got)
	}
	// Back online: unlocks at the next check-in, with a fresh 4 days.
	r.srv.set(func(f *fakeLicenseServer) { f.down = false; f.now = r.now })
	if got := r.checkIn(); got != licenseOK || r.m.Status().WarnHours != 0 {
		t.Fatalf("back online = %q", got)
	}
}

func TestLicenseCustomOfflineDays(t *testing.T) {
	r := newLicenseRig(t)
	r.srv.set(func(f *fakeLicenseServer) { f.days = 10 })
	r.activate()
	r.srv.set(func(f *fakeLicenseServer) { f.down = true })
	r.now = r.now.Add(9 * 24 * time.Hour)
	if got := r.checkIn(); got != licenseOK {
		t.Fatalf("9 of 10 days = %q", got)
	}
	r.now = r.now.Add(24 * time.Hour)
	if got := r.checkIn(); got != licenseLocked {
		t.Fatalf("10 of 10 days = %q", got)
	}
}

func TestLicenseClockSetBackOfflineLocks(t *testing.T) {
	r := newLicenseRig(t)
	r.activate()
	r.now = r.now.Add(3 * 24 * time.Hour)
	r.checkIn() // a lease issued at the server's t0, LastSeen = day 3
	r.m.mu.Lock()
	r.m.noteTimeLocked(true)
	r.m.mu.Unlock()
	r.srv.set(func(f *fakeLicenseServer) { f.down = true })
	// Offline, clock wound back to day 1 to stretch the lease: locked.
	r.now = t0License.Add(24 * time.Hour)
	if got := r.checkIn(); got != licenseLocked || r.m.reason != license.ReasonClock {
		t.Fatalf("clock set back = %q (%s)", got, r.m.reason)
	}
	// Online again, even with the wrong clock: the fresh lease counts.
	r.srv.set(func(f *fakeLicenseServer) { f.down = false; f.now = t0License.Add(3 * 24 * time.Hour) })
	if got := r.checkIn(); got != licenseOK {
		t.Fatalf("online with a wrong clock = %q (%s)", got, r.m.reason)
	}
}

func TestLicenseUnknownInstallLocks(t *testing.T) {
	r := newLicenseRig(t)
	r.activate()
	r.srv.set(func(f *fakeLicenseServer) { f.forget = true })
	if got := r.checkIn(); got != licenseLocked {
		t.Fatalf("forgotten install = %q", got)
	}
}

func TestLicenseFileFromAnotherPCAsksForTheKey(t *testing.T) {
	r := newLicenseRig(t)
	r.activate()
	dir := r.m.path[:len(r.m.path)-len("/license.dat")]
	r.fp = "fp-other-pc"
	other := r.manager(dir)
	if err := other.load(); !errors.Is(err, errLicenseNotActivated) {
		t.Fatalf("copied license file = %v", err)
	}
	if other.DBKey() != "" {
		t.Fatal("a copied license file gave out the database key")
	}
}

func TestLicenseFingerprintFailureLocksWithoutAskingForAKey(t *testing.T) {
	r := newLicenseRig(t)
	r.m.fingerprint = func() (string, error) { return "", errors.New("WMI down") }
	if err := r.m.load(); !errors.Is(err, errLicenseLocked) {
		t.Fatalf("load = %v", err)
	}
	if r.m.Status().State != licenseLocked {
		t.Fatalf("state %q", r.m.Status().State)
	}
}

func TestCheckoutRefusedWhileLocked(t *testing.T) {
	r := newLicenseRig(t)
	r.activate()
	app := &App{license: r.m}
	licenseRequired = true
	defer func() { licenseRequired = false }()
	if err := app.licenseBlocked(); err != nil {
		t.Fatalf("running till blocked: %v", err)
	}
	r.srv.set(func(f *fakeLicenseServer) { f.status = "off" })
	r.checkIn()
	if _, err := app.CheckoutInvoice("[]", "[]", 0, "", "", "", "", "", ""); err == nil || !strings.Contains(err.Error(), "can't access central") {
		t.Fatalf("checkout while locked = %v", err)
	}
	if _, err := app.CheckoutExchangeInvoice("[]", "[]", 0, 0, 0, "", false); err == nil || !strings.Contains(err.Error(), "can't access central") {
		t.Fatalf("exchange while locked = %v", err)
	}
	if got := app.vacuumTarget("/x/b.db"); !strings.Contains(got, "vfs=adiantum&hexkey="+r.srv.dbKey) {
		t.Fatalf("backup target %q is not encrypted", got)
	}
}

func TestLicenseCheckInLoopRunsAndStops(t *testing.T) {
	r := newLicenseRig(t)
	r.activate()
	r.m.start()
	r.m.start() // twice is harmless
	r.m.kick <- struct{}{}
	deadline := time.Now().Add(5 * time.Second)
	for {
		r.srv.mu.Lock()
		n := r.srv.checks
		r.srv.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("loop never checked in")
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.m.close()
	r.m.close()
}

const licenseStatusGoldenPath = "frontend/src/lib/testdata/license_status.json"

// Real statuses from the manager, for frontend/src/lib/license.test.ts: a
// change to LicenseStatus on this side that the screen can't read fails there.
func TestLicenseStatusGolden(t *testing.T) {
	out := map[string]LicenseStatus{}
	r := newLicenseRig(t)
	_ = r.m.load()
	out["activate"] = r.m.Status()
	r.activate()
	out["ok"] = r.m.Status()
	r.srv.set(func(f *fakeLicenseServer) { f.down = true })
	r.now = r.now.Add(72 * time.Hour)
	r.checkIn()
	out["okOfflineWarning"] = r.m.Status()
	r.srv.set(func(f *fakeLicenseServer) { f.down = false; f.status = "off"; f.now = r.now })
	r.checkIn()
	out["lockedOff"] = r.m.Status()
	r.srv.set(func(f *fakeLicenseServer) { f.down = true })
	r.now = r.now.Add(30 * 24 * time.Hour)
	out["lockedOffline"] = r.m.Status()
	r.checkIn()
	out["lockedOffline"] = r.m.Status()
	out["licensingOff"] = (&App{}).GetLicenseStatus()

	got, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if os.Getenv("KASHI_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(licenseStatusGoldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(licenseStatusGoldenPath)
	if err != nil {
		t.Fatalf("%v (run with KASHI_UPDATE_GOLDEN=1)", err)
	}
	if string(want) != string(got) {
		t.Fatalf("%s is stale; run with KASHI_UPDATE_GOLDEN=1\n%s", licenseStatusGoldenPath, got)
	}
	for name, s := range out {
		if strings.HasPrefix(name, "locked") && (s.LicensedTo != "" || s.WarnHours != 0) {
			t.Fatalf("%s tells the cashier more than the lock message: %+v", name, s)
		}
	}
}
