package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// What the cashier sees of the connection, in one place. The sync indicator
// used to say only "Synced" or "Syncing" from the push queue, so a till
// whose live link to central was down (central shows its branch offline) and a
// till with no internet at all both read "Synced". Here the three are kept
// apart, with what the till is doing right now:
//
//   - Internet: can this machine reach the internet at all?
//   - central:    did the last request to the server get an answer?
//   - Live:     is the push channel (websocket) up? This is the same thing
//     central's Branches page shows as connected.
//   - Activity: pulling an entity (3 of 21), sending queued records.
//
// One package-level tracker, because the transport that sees every request
// (linkTransport) has no App to hand. Reads are snapshots; changes ask the
// frontend to refetch through "link_state_changed", at most a few times a
// second so a 21-entity pull doesn't flood the bridge.

const (
	linkEmitMinGap       = 250 * time.Millisecond
	linkInternetInterval = 20 * time.Second
	linkInternetTimeout  = 3 * time.Second
	// In quiet mode a step that really received something stays on the light
	// at least this long, so a change that lands in a few milliseconds can
	// still be seen.
	linkMinShow = 1500 * time.Millisecond
)

// Phases of the till's current work. "idle" means nothing is moving.
const (
	phaseIdle    = "idle"
	phasePulling = "pulling"
	phasePushing = "pushing"
)

// LinkStatus is what GetLinkStatus returns. Timestamps are RFC 3339 (UTC) so
// the screen can say "12 s ago" itself and keep the text fresh without
// asking again.
type LinkStatus struct {
	// Internet is "up", "down" or "unknown" (not tested yet).
	Internet string `json:"internet"`
	// Central is "up" (the last request got an answer), "down" or "unknown".
	Kashi        string `json:"kashi"`
	KashiError   string `json:"kashiError"`
	KashiLastOK  string `json:"kashiLastOk"`
	KashiDownFor int64  `json:"kashiDownForSeconds"`
	// Live is the push channel: true while the websocket is connected.
	Live          bool   `json:"live"`
	LiveSince     string `json:"liveSince"`
	LiveLastBeat  string `json:"liveLastBeat"`
	LiveError     string `json:"liveError"`
	LiveRetryAt   string `json:"liveRetryAt"`
	LiveEverUp    bool   `json:"liveEverUp"`
	Phase         string `json:"phase"`
	Label         string `json:"label"`
	Detail        string `json:"detail"`
	Done          int    `json:"done"`
	Total         int    `json:"total"`
	LastPullAt    string `json:"lastPullAt"`
	LastPushAt    string `json:"lastPushAt"`
	LastPullError string `json:"lastPullError"`
}

type linkTracker struct {
	mu sync.Mutex

	internet        string
	internetChecked time.Time

	kashi       string
	kashiErr    string
	kashiLastOK time.Time
	kashiDownAt time.Time

	live      bool
	liveEver  bool
	liveSince time.Time
	liveBeat  time.Time
	liveErr   string
	liveRetry time.Time

	phase  string
	label  string
	detail string
	done   int
	total  int

	lastPull    time.Time
	lastPush    time.Time
	lastPullErr string

	// verbose: show every step of a pull, even one that finds nothing new.
	// Quiet (the default) shows a step only once it has really received
	// something different. The step being checked waits in step* until then.
	verbose     bool
	stepLabel   string
	stepDone    int
	stepTotal   int
	revealedAt  time.Time
	hideTimer   *time.Timer
	fingerprint map[string][32]byte

	emit      func()
	lastEmit  time.Time
	emitTimer *time.Timer
	now       func() time.Time
}

var link = &linkTracker{now: time.Now}

func (l *linkTracker) setEmitter(fn func()) {
	l.mu.Lock()
	l.emit = fn
	l.mu.Unlock()
}

// changed asks for a refetch, coalesced: the first change emits at once, the
// ones that follow inside linkEmitMinGap share one emit when the gap ends.
// Caller holds l.mu.
func (l *linkTracker) changedLocked() {
	if l.emit == nil || l.emitTimer != nil {
		return
	}
	wait := linkEmitMinGap - l.now().Sub(l.lastEmit)
	if wait < 0 {
		wait = 0
	}
	l.emitTimer = time.AfterFunc(wait, func() {
		l.mu.Lock()
		l.emitTimer = nil
		l.lastEmit = l.now()
		fn := l.emit
		l.mu.Unlock()
		if fn != nil {
			fn()
		}
	})
}

// kashiResult records what a request to central came back with. Any HTTP
// answer, even a 500, means central is reachable; only a failure to get one
// (no route, DNS, timeout, refused) means it is not.
func (l *linkTracker) kashiResult(err error) {
	if errors.Is(err, context.Canceled) {
		return // the caller gave up; that says nothing about central
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	prev, prevErr := l.kashi, l.kashiErr
	if err == nil {
		l.kashi, l.kashiErr = "up", ""
		l.kashiLastOK = now
		l.kashiDownAt = time.Time{}
		// An answer proves the internet is there; no need to probe it.
		l.internet, l.internetChecked = "up", now
	} else {
		l.kashi, l.kashiErr = "down", friendlyNetError(err)
		if prev != "down" {
			l.kashiDownAt = now
		}
	}
	if prev != l.kashi || prevErr != l.kashiErr {
		l.changedLocked()
	}
}

func (l *linkTracker) wsUp() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.live, l.liveEver = true, true
	l.liveSince, l.liveBeat = now, now
	l.liveErr, l.liveRetry = "", time.Time{}
	l.changedLocked()
}

// wsBeat notes a sign of life on the live channel (a pong or a frame). It
// doesn't ask the screen to refetch: it only moves the "last heard" clock.
func (l *linkTracker) wsBeat() {
	l.mu.Lock()
	l.liveBeat = l.now()
	l.mu.Unlock()
}

// wsDown records the channel being lost (or never coming up) and when the
// next try is.
func (l *linkTracker) wsDown(err error, retryIn time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.live = false
	if err != nil {
		l.liveErr = friendlyNetError(err)
	}
	if retryIn > 0 {
		l.liveRetry = l.now().Add(retryIn)
	}
	l.changedLocked()
}

func (l *linkTracker) setInternet(state string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.internetChecked = l.now()
	if l.internet != state {
		l.internet = state
		l.changedLocked()
	}
}

func (l *linkTracker) setVerbose(v bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.verbose = v
	if v && l.phase == phasePulling {
		return
	}
	l.changedLocked()
}

func (l *linkTracker) isVerbose() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.verbose
}

// checking says which pull step the till is on. Verbose shows it at once; quiet
// only remembers it, and shows it later if the step turns out to receive
// something (reveal).
func (l *linkTracker) checking(label string, done, total int) {
	l.mu.Lock()
	l.stepLabel, l.stepDone, l.stepTotal = label, done, total
	verbose := l.verbose
	l.mu.Unlock()
	if verbose {
		l.begin(phasePulling, label, done, total)
		return
	}
	// Moving on from a step that did receive something: let it show a moment.
	l.end(phasePulling)
}

// reveal shows the step being checked, because it really received something
// (quiet mode). detail is the short "N changes".
func (l *linkTracker) reveal(detail string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.verbose {
		l.detail = detail
		l.changedLocked()
		return
	}
	if l.phase != phasePulling {
		l.revealedAt = l.now()
	}
	l.phase, l.label, l.done, l.total, l.detail = phasePulling, l.stepLabel, l.stepDone, l.stepTotal, detail
	if l.hideTimer != nil {
		l.hideTimer.Stop()
		l.hideTimer = nil
	}
	l.changedLocked()
}

// sameAsLast reports whether this entity's batch is byte for byte what the
// last pass already applied (and remembers it). Central re-sends rows changed in
// the last couple of minutes (its cursor overlap), and an unchanged one is not
// news.
func (l *linkTracker) sameAsLast(entity string, raw []byte) bool {
	sum := sha256.Sum256(raw)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.fingerprint == nil {
		l.fingerprint = map[string][32]byte{}
	}
	prev, ok := l.fingerprint[entity]
	l.fingerprint[entity] = sum
	return ok && prev == sum
}

// begin starts (or moves) the current activity. total 0 means "unknown".
func (l *linkTracker) begin(phase, label string, done, total int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.phase, l.label, l.done, l.total, l.detail = phase, label, done, total, ""
	l.changedLocked()
}

func (l *linkTracker) setDetail(detail string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.detail != detail {
		l.detail = detail
		l.changedLocked()
	}
}

func (l *linkTracker) progress(done, total int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.done, l.total = done, total
	l.changedLocked()
}

// end finishes the activity if it is still the one named (a push finishing
// must not wipe a pull that started meanwhile).
func (l *linkTracker) end(phase string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.phase != phase {
		return
	}
	// A revealed quiet step stays up for linkMinShow, then goes.
	if phase == phasePulling && !l.verbose && !l.revealedAt.IsZero() {
		if wait := linkMinShow - l.now().Sub(l.revealedAt); wait > 0 {
			if l.hideTimer == nil {
				l.hideTimer = time.AfterFunc(wait, func() {
					l.mu.Lock()
					l.hideTimer = nil
					l.mu.Unlock()
					l.end(phasePulling)
				})
			}
			return
		}
	}
	l.revealedAt = time.Time{}
	l.phase, l.label, l.detail, l.done, l.total = phaseIdle, "", "", 0, 0
	l.changedLocked()
}

func (l *linkTracker) pulled(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lastPull = l.now()
	l.lastPullErr = ""
	if err != nil {
		l.lastPullErr = friendlyNetError(err)
	}
	l.changedLocked()
}

func (l *linkTracker) pushed() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lastPush = l.now()
	l.changedLocked()
}

func (l *linkTracker) snapshot() LinkStatus {
	l.mu.Lock()
	defer l.mu.Unlock()
	ts := func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.UTC().Format(time.RFC3339)
	}
	s := LinkStatus{
		Internet:      orDefault(l.internet, "unknown"),
		Kashi:         orDefault(l.kashi, "unknown"),
		KashiError:    l.kashiErr,
		KashiLastOK:   ts(l.kashiLastOK),
		Live:          l.live,
		LiveSince:     ts(l.liveSince),
		LiveLastBeat:  ts(l.liveBeat),
		LiveError:     l.liveErr,
		LiveRetryAt:   ts(l.liveRetry),
		LiveEverUp:    l.liveEver,
		Phase:         orDefault(l.phase, phaseIdle),
		Label:         l.label,
		Detail:        l.detail,
		Done:          l.done,
		Total:         l.total,
		LastPullAt:    ts(l.lastPull),
		LastPushAt:    ts(l.lastPush),
		LastPullError: l.lastPullErr,
	}
	if !l.kashiDownAt.IsZero() {
		s.KashiDownFor = int64(l.now().Sub(l.kashiDownAt).Seconds())
	}
	return s
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// friendlyNetError turns a Go network error into something a cashier can
// act on. The raw text stays in the log; this is for the screen.
func friendlyNetError(err error) string {
	if err == nil {
		return ""
	}
	var dns *net.DNSError
	switch {
	case errors.As(err, &dns):
		return "can't find the server (no internet, or DNS is down)"
	case errors.Is(err, context.DeadlineExceeded):
		return "the server took too long to answer"
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "the server took too long to answer"
	}
	var op *net.OpError
	if errors.As(err, &op) {
		return "can't connect to the server"
	}
	return err.Error()
}

// linkTransport reports every request's outcome to the tracker. It wraps the
// transport of each centralClient, so any call to central (push, pull,
// commands, health) keeps the "Central reachable" answer fresh for free.
type linkTransport struct{ base http.RoundTripper }

func (t linkTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	link.kashiResult(err)
	return resp, err
}

// probeInternet asks "is the internet there?" by opening a TCP connection to
// well-known public addresses. Tried only while central isn't answering (when
// Central answers, that already proves it): it is how the screen tells "no
// internet" from "internet fine, central unreachable".
func probeInternet(ctx context.Context) bool {
	d := net.Dialer{Timeout: linkInternetTimeout}
	for _, addr := range []string{"1.1.1.1:443", "8.8.8.8:53", "9.9.9.9:443"} {
		c, err := d.DialContext(ctx, "tcp", addr)
		if err == nil {
			c.Close()
			return true
		}
	}
	return false
}

// runInternetProbe re-tests the internet whenever central has stopped
// answering, and now and then when it has not been proven for a while.
func (a *App) runInternetProbe() {
	t := time.NewTicker(linkInternetInterval)
	defer t.Stop()
	probe := func() {
		link.mu.Lock()
		kashiDown := link.kashi == "down"
		stale := link.now().Sub(link.internetChecked) > 2*linkInternetInterval
		link.mu.Unlock()
		if !kashiDown && !stale {
			return
		}
		if probeInternet(a.ctx) {
			link.setInternet("up")
		} else {
			link.setInternet("down")
		}
	}
	for {
		select {
		case <-a.wsClientStop:
			return
		case <-t.C:
			probe()
		}
	}
}

// GetLinkStatus is the Wails binding behind the connection panel. Cheap:
// it only copies the tracker.
func (a *App) GetLinkStatus() LinkStatus {
	return link.snapshot()
}

func (a *App) emitLinkStateChanged() {
	select {
	case <-a.ready:
		runtime.EventsEmit(a.ctx, "link_state_changed")
	default:
	}
}

// entityLabels are the names the screen shows for what a pull is fetching.
var entityLabels = map[string]string{
	"currencies": "Currencies", "cashbox_accounts": "Cash accounts", "products": "Products",
	"price_lists": "Price lists", "discount_lists": "Discount lists", "clients": "Clients",
	"branch_targets": "Targets", "salespersons": "Salespersons", "branch_users": "Users",
	"expense_categories": "Expense categories", "loan_categories": "Loan categories",
	"coupon_types": "Coupon types", "coupons": "Coupons", "branches": "Branches",
	"transfer_requests": "Transfers", "incoming_transfer_requests": "Incoming transfers",
	"branch_stock": "Stock", "quick_access": "Quick access", "invoice_discounts": "Invoice discounts",
	"calendar_events": "Calendar events", "calendar_settings": "Calendar settings",
}

func entityLabel(entity string) string {
	if l, ok := entityLabels[entity]; ok {
		return l
	}
	return entity
}
