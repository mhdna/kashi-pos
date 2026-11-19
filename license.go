package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"kashi-pos/internal/license"
)

// Licensing: a till runs only with a lease signed by the vendor's license
// server (kashi-license). See docs/dev/data-and-config.md#licensing.
//
//   - First launch: the activation screen asks for a product key. The server
//     binds it to this PC's hardware fingerprint and returns an install
//     secret, the database key, and a lease. They are kept in one sealed file
//     (license.dat beside branches.json) that opens only on this PC.
//   - While running: a check-in about once a minute renews the lease. A lease
//     is good for the customer's offline days (4 by default), so a till cut
//     off from the internet stops starting when it runs out.
//   - Turned off on the server: the next check-in returns an "off" lease and
//     the screen locks with a vague startup error. The database stays open
//     underneath, so sales already made keep syncing up, and the check-ins
//     continue, so turning the customer back on unlocks the till by itself.

// licenseRequired is on in every release build. `wails dev` runs without it
// (it works against plain branch databases) unless KASHI_LICENSE=1. Tests turn
// it off in license_test.go.
var licenseRequired = !isDevMode || os.Getenv("KASHI_LICENSE") == "1"

// licenseServerURL can be set at build time with
// -ldflags "-X main.licenseServerURL=https://...". KASHI_LICENSE_URL overrides
// it; that is safe to allow, since a server without the private key can't sign
// a lease this build accepts.
var licenseServerURL = "https://license.grandbz.com"

func licenseURL() string {
	if v := strings.TrimSpace(os.Getenv("KASHI_LICENSE_URL")); v != "" {
		return v
	}
	return licenseServerURL
}

// Check-in pacing: the server says how often (60 s while on, 30 s while off);
// these bound it so a bad value can neither flood it nor go quiet for hours.
const (
	licenseMinCheck     = 15 * time.Second
	licenseMaxCheck     = 10 * time.Minute
	licenseDefaultCheck = time.Minute
	// lastSeenSaveEvery is how often the "latest time this till ran at" is
	// written back while nothing else changes.
	lastSeenSaveEvery = 10 * time.Minute
)

// Frontend states.
const (
	licenseOK       = "ok"
	licenseActivate = "activate"
	licenseLocked   = "locked"
)

var (
	errLicenseNotActivated = errors.New("license: not activated")
	errLicenseLocked       = errors.New("license: locked")
)

// LicenseStatus is what the screen needs. It never says why a till is
// locked; Code is a short install id for a support call.
type LicenseStatus struct {
	Required   bool   `json:"required"`
	State      string `json:"state"`
	Code       string `json:"code"`
	LicensedTo string `json:"licensedTo"`
	// WarnHours is set (> 0) when the till hasn't reached the server for a
	// while and its lease runs out within this many hours.
	WarnHours int `json:"warnHours"`
}

type licenseManager struct {
	mu          sync.Mutex
	pub         ed25519.PublicKey
	path        string
	client      *license.Client
	fingerprint func() (string, error)
	now         func() time.Time
	// onChange is called (without mu held) when State flips.
	onChange func(LicenseStatus)

	fp        string
	state     license.State
	activated bool
	decision  license.Decision
	// reason is why the till is locked (logged, never shown).
	reason    string
	lastSaved time.Time

	stop chan struct{}
	kick chan struct{}
	wg   sync.WaitGroup
}

func newLicenseManager(dir string) (*licenseManager, error) {
	pub, err := license.ParsePublicKey(license.ServerPublicKeyPEM)
	if err != nil {
		return nil, err
	}
	return &licenseManager{
		pub:         pub,
		path:        license.StatePath(dir),
		client:      license.NewClient(licenseURL()),
		fingerprint: license.Fingerprint,
		now:         time.Now,
		kick:        make(chan struct{}, 1),
	}, nil
}

// load reads the license file and judges it offline. It returns
// errLicenseNotActivated when there is nothing usable to judge.
func (m *licenseManager) load() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	fp, err := m.fingerprint()
	if err != nil {
		// Without the fingerprint the license file can't be opened. Locked,
		// not "activate": the till was likely activated, and Retry reads the
		// hardware again.
		m.reason = "fingerprint: " + err.Error()
		return errLicenseLocked
	}
	m.fp = fp
	st, err := license.LoadState(m.path, fp)
	if err != nil {
		if !errors.Is(err, license.ErrNoState) {
			log.Printf("license file unusable, asking for the key again: %v", err)
		}
		m.activated = false
		return errLicenseNotActivated
	}
	m.state, m.activated = st, true
	m.judgeLocked()
	return nil
}

func (m *licenseManager) judgeLocked() {
	m.decision = license.Judge(m.pub, m.state, m.fp, m.now())
	if m.decision.OK {
		m.reason = ""
	} else {
		m.reason = m.decision.Reason
	}
}

func (m *licenseManager) statusLocked() LicenseStatus {
	s := LicenseStatus{Required: true}
	switch {
	case !m.activated && m.fp != "":
		s.State = licenseActivate
	case !m.activated || !m.decision.OK:
		s.State = licenseLocked
	default:
		s.State = licenseOK
		s.LicensedTo = m.decision.Lease.LicensedTo
		if m.decision.Warn {
			s.WarnHours = max(1, int(m.decision.Left.Hours()))
		}
	}
	if len(m.state.InstallID) >= 8 {
		s.Code = strings.ToUpper(m.state.InstallID[:8])
	}
	return s
}

func (m *licenseManager) Status() LicenseStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked()
}

func (m *licenseManager) DBKey() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state.DBKey
}

// adoptLocked takes a lease from the server, if it is really for this
// install and machine, and saves it.
func (m *licenseManager) adoptLocked(signed license.Signed) error {
	l, err := license.Verify(m.pub, signed)
	if err != nil {
		return err
	}
	if l.InstallID != m.state.InstallID || l.Fingerprint != m.fp {
		return errors.New("lease is for another install")
	}
	m.state.Lease = signed
	m.noteTimeLocked(true)
	m.judgeLocked()
	return nil
}

// trustFreshLeaseLocked: a lease that just came from the server is good
// even if this PC's clock is wrong (a dead CMOS battery). The clock checks
// exist for a till that is offline; one that checks in gains nothing from a
// wrong clock. LastSeen restarts from this clock so it doesn't keep the till
// locked; offline, a lease issued later than the PC's time still locks it.
func (m *licenseManager) trustFreshLeaseLocked() {
	if m.decision.Reason != license.ReasonClock {
		return
	}
	l := m.decision.Lease
	m.state.LastSeen = m.now()
	m.decision = license.Decision{OK: true, Lease: l, Left: l.ValidUntil.Sub(l.IssuedAt)}
	m.reason = ""
	if err := license.SaveState(m.path, m.fp, m.state); err != nil {
		log.Printf("save license file: %v", err)
	}
}

// noteTimeLocked moves LastSeen forward and saves when needed.
func (m *licenseManager) noteTimeLocked(force bool) {
	now := m.now()
	if now.After(m.state.LastSeen) {
		m.state.LastSeen = now
	}
	if force || now.Sub(m.lastSaved) >= lastSeenSaveEvery {
		if err := license.SaveState(m.path, m.fp, m.state); err != nil {
			log.Printf("save license file: %v", err)
			return
		}
		m.lastSaved = now
	}
}

// checkIn asks the server for a fresh lease. A network failure keeps the
// saved lease and only re-judges it against the clock.
func (m *licenseManager) checkIn(ctx context.Context) {
	m.mu.Lock()
	if !m.activated {
		m.mu.Unlock()
		return
	}
	before := m.statusLocked().State
	req := license.CheckInRequest{InstallID: m.state.InstallID, InstallSecret: m.state.InstallSecret, Fingerprint: m.fp, AppVersion: appVersion}
	m.mu.Unlock()

	signed, err := m.client.CheckIn(ctx, req)

	m.mu.Lock()
	var apiErr *license.APIError
	switch {
	case err == nil:
		if aerr := m.adoptLocked(signed); aerr != nil {
			log.Printf("license check-in: rejected lease: %v", aerr)
			m.judgeLocked()
		} else {
			m.trustFreshLeaseLocked()
		}
	case errors.As(err, &apiErr) && (apiErr.Code == license.CodeUnknownInstall || apiErr.Code == license.CodeWrongMachine):
		// The server no longer accepts this install. Lock, keep the file:
		// the vendor may fix it on their side.
		m.decision = license.Decision{Reason: apiErr.Code}
		m.reason = apiErr.Code
	default:
		m.judgeLocked()
		if m.decision.OK {
			m.noteTimeLocked(false)
		}
	}
	after := m.statusLocked()
	reason := m.reason
	m.mu.Unlock()

	if after.State != before {
		log.Printf("license: %s -> %s (%s)", before, after.State, reason)
		if m.onChange != nil {
			m.onChange(after)
		}
	}
}

// interval is how long to wait until the next check-in.
func (m *licenseManager) interval() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	d := time.Duration(m.decision.Lease.CheckEverySeconds) * time.Second
	if d == 0 {
		d = licenseDefaultCheck
	}
	return min(max(d, licenseMinCheck), licenseMaxCheck)
}

func (m *licenseManager) start() {
	m.mu.Lock()
	if m.stop != nil {
		m.mu.Unlock()
		return
	}
	m.stop = make(chan struct{})
	stop := m.stop
	m.mu.Unlock()
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			timer := time.NewTimer(m.interval())
			select {
			case <-stop:
				timer.Stop()
				return
			case <-m.kick:
			case <-timer.C:
			}
			timer.Stop()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			m.checkIn(ctx)
			cancel()
		}
	}()
}

func (m *licenseManager) close() {
	m.mu.Lock()
	stop := m.stop
	m.stop = nil
	m.mu.Unlock()
	if stop != nil {
		close(stop)
		m.wg.Wait()
	}
}

// activate binds productKey to this PC. It returns a code the screen turns
// into words (see frontend/src/lib/license.ts).
func (m *licenseManager) activate(ctx context.Context, productKey, hostname string) error {
	m.mu.Lock()
	if m.fp == "" {
		fp, err := m.fingerprint()
		if err != nil {
			m.mu.Unlock()
			return fmt.Errorf("license:fingerprint")
		}
		m.fp = fp
	}
	fp := m.fp
	m.mu.Unlock()

	resp, err := m.client.Activate(ctx, license.ActivateRequest{ProductKey: productKey, Fingerprint: fp, Hostname: hostname, AppVersion: appVersion})
	var apiErr *license.APIError
	if errors.As(err, &apiErr) {
		return fmt.Errorf("license:%s", apiErr.Code)
	}
	if err != nil {
		log.Printf("license activate: %v", err)
		return fmt.Errorf("license:offline")
	}
	if len(resp.DBKey) != 64 || resp.InstallID == "" || resp.InstallSecret == "" {
		return fmt.Errorf("license:server_error")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = license.State{InstallID: resp.InstallID, InstallSecret: resp.InstallSecret, DBKey: resp.DBKey}
	if err := m.adoptLocked(resp.Lease); err != nil {
		log.Printf("license activate: rejected lease: %v", err)
		m.state = license.State{}
		return fmt.Errorf("license:server_error")
	}
	m.trustFreshLeaseLocked()
	if err := license.SaveState(m.path, m.fp, m.state); err != nil {
		log.Printf("license activate: save: %v", err)
		m.state = license.State{}
		return fmt.Errorf("license:save_failed")
	}
	m.activated = true
	return nil
}

// --- App wiring ---

// startLicense runs at startup before any database opens. It returns
// errLicenseNotActivated when the activation screen must come first; for a
// locked till it returns nil, so the database still opens underneath the
// lock screen and syncs.
func (a *App) startLicense() error {
	if !licenseRequired {
		return nil
	}
	dir, err := branchRegistryDir()
	if err != nil {
		return err
	}
	m, err := newLicenseManager(dir)
	if err != nil {
		return err
	}
	m.onChange = a.licenseChanged
	a.license = m
	switch err := m.load(); {
	case errors.Is(err, errLicenseNotActivated):
		return errLicenseNotActivated
	case errors.Is(err, errLicenseLocked):
		return errLicenseLocked
	}
	if m.Status().State != licenseOK {
		// Out of date or set back: one check-in at once, so a till that was
		// only offline unlocks as soon as it is back.
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		m.checkIn(ctx)
		cancel()
	}
	m.start()
	return nil
}

func (a *App) licenseChanged(s LicenseStatus) {
	// Wails log.Fatal()s on a context without its frontend (tests, headless).
	if a.ctx != nil && a.ctx.Value("frontend") != nil {
		runtime.EventsEmit(a.ctx, "license:changed", s)
	}
}

// GetLicenseStatus tells the screen whether to show the app, the activation
// screen or the lock screen.
func (a *App) GetLicenseStatus() LicenseStatus {
	if !licenseRequired || a.license == nil {
		return LicenseStatus{State: licenseOK}
	}
	return a.license.Status()
}

// licenseBlocked is the backend's own check, for actions that must not run
// behind a lock screen even if the screen were bypassed.
func (a *App) licenseBlocked() error {
	if s := a.GetLicenseStatus(); s.State != licenseOK {
		return errors.New("error starting up: can't access central")
	}
	return nil
}

// ActivateLicense takes the product key from the activation screen, then
// finishes the startup that waited for it.
func (a *App) ActivateLicense(productKey string) (LicenseStatus, error) {
	if !licenseRequired || a.license == nil {
		return a.GetLicenseStatus(), nil
	}
	host, _ := os.Hostname()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := a.license.activate(ctx, productKey, host); err != nil {
		return a.license.Status(), err
	}
	a.license.start()
	if a.conn == nil {
		a.startupErr = nil
		a.openStartupDatabase()
	}
	return a.license.Status(), nil
}

// RetryLicense is the lock screen's button: read the hardware again if that
// failed, and check in now.
func (a *App) RetryLicense() LicenseStatus {
	m := a.license
	if !licenseRequired || m == nil {
		return a.GetLicenseStatus()
	}
	before := m.Status().State
	if errors.Is(a.startupErr, errLicenseLocked) {
		if err := m.load(); err == nil || errors.Is(err, errLicenseNotActivated) {
			a.startupErr = err
			if err == nil {
				m.start()
				a.openStartupDatabase()
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	m.checkIn(ctx)
	cancel()
	after := m.Status()
	if after.State != before {
		a.licenseChanged(after)
	}
	return after
}
