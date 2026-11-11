package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// The sync light has two modes. Verbose shows every step of a pull, even one
// that finds nothing new. Quiet (the default) shows a step only once it has
// really received something different: "salespersons" must not keep showing
// up just because the till asked whether it changed.

func newQuietTracker() *linkTracker {
	l := &linkTracker{now: time.Now}
	return l
}

func TestQuietTrackerHidesStepsThatFindNothing(t *testing.T) {
	l := newQuietTracker()
	for i, label := range []string{"Currencies", "Products", "Salespersons"} {
		l.checking(label, i, 3)
		if s := l.snapshot(); s.Phase != phaseIdle {
			t.Fatalf("checking %s with nothing found showed %+v", label, s)
		}
	}
}

func TestQuietTrackerShowsAStepThatReceivedSomething(t *testing.T) {
	l := newQuietTracker()
	l.checking("Products", 2, 21)
	l.reveal("500 changes")
	s := l.snapshot()
	if s.Phase != phasePulling || s.Label != "Products" || s.Detail != "500 changes" || s.Done != 2 || s.Total != 21 {
		t.Fatalf("revealed step: %+v", s)
	}
}

func TestQuietRevealedStepStaysUpLongEnoughToSee(t *testing.T) {
	l := newQuietTracker()
	clock := time.Now()
	l.now = func() time.Time { return clock }

	l.checking("Salespersons", 5, 21)
	l.reveal("1 changes")
	// The next step starts at once, finding nothing: the revealed one stays.
	l.checking("Branch users", 6, 21)
	if l.snapshot().Label != "Salespersons" {
		t.Fatal("a revealed step vanished the moment the pass moved on")
	}
	// The pass ends: still up inside the minimum.
	l.end(phasePulling)
	if l.snapshot().Phase != phasePulling {
		t.Fatal("a revealed step vanished at the end of the pass")
	}
	// After the minimum it goes.
	deadline := time.Now().Add(linkMinShow + time.Second)
	clock = clock.Add(linkMinShow + time.Millisecond)
	for time.Now().Before(deadline) && l.snapshot().Phase != phaseIdle {
		time.Sleep(20 * time.Millisecond)
	}
	if s := l.snapshot(); s.Phase != phaseIdle {
		t.Fatalf("still showing after the minimum: %+v", s)
	}
}

func TestQuietRevealAfterTheMinimumIsDroppedByTheNextStep(t *testing.T) {
	l := newQuietTracker()
	clock := time.Now()
	l.now = func() time.Time { return clock }
	l.checking("Products", 2, 21)
	l.reveal("3 changes")
	clock = clock.Add(2 * linkMinShow)
	l.checking("Price lists", 3, 21)
	if s := l.snapshot(); s.Phase != phaseIdle {
		t.Fatalf("an old reveal should not outlive the next step: %+v", s)
	}
}

func TestVerboseTrackerShowsEveryStep(t *testing.T) {
	l := newQuietTracker()
	l.setVerbose(true)
	l.checking("Salespersons", 5, 21)
	if s := l.snapshot(); s.Phase != phasePulling || s.Label != "Salespersons" || s.Done != 5 {
		t.Fatalf("verbose should show the step being checked: %+v", s)
	}
	l.reveal("2 changes")
	if l.snapshot().Detail != "2 changes" {
		t.Fatal("verbose still shows the count")
	}
	l.end(phasePulling)
	if l.snapshot().Phase != phaseIdle {
		t.Fatal("verbose ends at once")
	}
}

func TestSameAsLastRemembersPerEntity(t *testing.T) {
	l := newQuietTracker()
	a := []byte(`[{"id":1,"name":"Ali"}]`)
	b := []byte(`[{"id":1,"name":"Ali B"}]`)
	if l.sameAsLast("salespersons", a) {
		t.Fatal("the first batch is news")
	}
	if !l.sameAsLast("salespersons", a) {
		t.Fatal("the same batch again is not news")
	}
	if l.sameAsLast("salespersons", b) {
		t.Fatal("a different batch is news")
	}
	if !l.sameAsLast("salespersons", b) {
		t.Fatal("and then it is remembered")
	}
	// Another entity has its own memory.
	if l.sameAsLast("products", a) {
		t.Fatal("products never saw this batch")
	}
}

// --- through a real pull ------------------------------------------------------

// feed is a stand-in for kashi's /branch/sync/changes that answers each entity
// from a map (empty when absent) and notes what the light showed at that moment.
type feed struct {
	mu      sync.Mutex
	items   map[string]string
	showing []string
}

func (f *feed) handler(w http.ResponseWriter, r *http.Request) {
	entity := r.URL.Query().Get("entity")
	f.mu.Lock()
	body := f.items[entity]
	if s := link.snapshot(); s.Phase == phasePulling {
		f.showing = append(f.showing, s.Label)
	}
	f.mu.Unlock()
	if body == "" {
		body = "[]"
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"entity":%q,"items":%s,"server_time":"2026-11-11T00:00:00Z"}`, entity, body)
}

func (f *feed) set(entity, body string) {
	f.mu.Lock()
	f.items[entity] = body
	f.mu.Unlock()
}

func (f *feed) seen() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.showing, ",")
}

func (f *feed) reset() {
	f.mu.Lock()
	f.showing = nil
	f.mu.Unlock()
}

func runPass(t *testing.T, app *App, serverURL string) {
	t.Helper()
	client := newCentralClient(serverURL, "TEST01", "key")
	app.syncPullMu.Lock()
	app.runSyncPullLocked(app.ctx, client)
	app.syncPullMu.Unlock()
}

// The reported case: a pass in which nothing changed shows nothing, whatever
// it checked.
func TestQuietPullOfNothingShowsNothing(t *testing.T) {
	link.setVerbose(false)
	f := &feed{items: map[string]string{}}
	server := httptest.NewServer(http.HandlerFunc(f.handler))
	defer server.Close()
	app, _, _, _ := setUpTestInvoiceApp(t, server.URL)

	// An empty feed makes the first pass drop the test database's own
	// currencies and cash accounts (a whole-set entity is whatever kashi says),
	// which really is a change. Settle that, then the pass under test finds
	// nothing at all.
	runPass(t, app, server.URL)
	time.Sleep(linkMinShow + 300*time.Millisecond)
	link.end(phasePulling)
	f.reset()

	runPass(t, app, server.URL)
	if got := f.seen(); got != "" {
		t.Fatalf("a pass that found nothing showed %q", got)
	}
	if s := link.snapshot(); s.Phase != phaseIdle || s.LastPullAt == "" {
		t.Fatalf("after the pass: %+v", s)
	}
}

func TestQuietPullShowsOnlyTheStepThatReceivedSomethingNew(t *testing.T) {
	link.setVerbose(false)
	f := &feed{items: map[string]string{}}
	server := httptest.NewServer(http.HandlerFunc(f.handler))
	defer server.Close()
	app, _, _, _ := setUpTestInvoiceApp(t, server.URL)

	// A salesperson kashi re-sends unchanged (its two-minute overlap): the
	// first time is news, the same rows again are not.
	f.set("salespersons", `[{"id":9001,"name":"Layla","is_active":true,"color":"#336699"}]`)
	runPass(t, app, server.URL)
	if s := link.snapshot(); s.Phase != phasePulling || s.Label != "Salespersons" || !strings.Contains(s.Detail, "1 change") {
		t.Fatalf("first time: %+v", s)
	}

	// Let the minimum pass, clear the light, and pass again with the same rows.
	time.Sleep(linkMinShow + 300*time.Millisecond)
	link.end(phasePulling)
	if link.snapshot().Phase != phaseIdle {
		t.Fatal("the light should be idle now")
	}
	runPass(t, app, server.URL)
	if s := link.snapshot(); s.Phase != phaseIdle {
		t.Fatalf("the same salesperson again must not show: %+v", s)
	}

	// A real change shows again.
	f.set("salespersons", `[{"id":9001,"name":"Layla K","is_active":true,"color":"#336699"}]`)
	runPass(t, app, server.URL)
	if s := link.snapshot(); s.Phase != phasePulling || s.Label != "Salespersons" {
		t.Fatalf("a changed salesperson: %+v", s)
	}
}

func TestVerbosePullShowsEveryStepEvenWhenNothingChanged(t *testing.T) {
	link.setVerbose(true)
	t.Cleanup(func() { link.setVerbose(false) })
	f := &feed{items: map[string]string{}}
	server := httptest.NewServer(http.HandlerFunc(f.handler))
	defer server.Close()
	app, _, _, _ := setUpTestInvoiceApp(t, server.URL)

	runPass(t, app, server.URL)
	got := f.seen()
	for _, want := range []string{"Currencies", "Products", "Salespersons", "Users", "Calendar settings"} {
		if !strings.Contains(got, want) {
			t.Errorf("verbose never showed %q (saw %q)", want, got)
		}
	}
}

// kashi's switch: off by default, turned on by an update_settings command, and
// reported back in the till's settings push.
func TestSyncVerboseSettingRoundTrips(t *testing.T) {
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/branch/settings" {
			b := new(strings.Builder)
			buf := make([]byte, 4096)
			for {
				n, err := r.Body.Read(buf)
				b.Write(buf[:n])
				if err != nil {
					break
				}
			}
			body = []byte(b.String())
		}
		w.Write([]byte(`{}`))
	}))
	defer server.Close()
	link.setVerbose(false)
	t.Cleanup(func() { link.setVerbose(false) })
	app, _, _, _ := setUpTestInvoiceApp(t, server.URL)

	if app.flagEnabled(app.ctx, syncVerboseFlag) || link.isVerbose() {
		t.Fatal("verbose must be off by default")
	}
	reported := func() bool {
		t.Helper()
		if err := app.pushSettings(app.ctx, newCentralClient(server.URL, "TEST01", "test-key")); err != nil {
			t.Fatal(err)
		}
		return strings.Contains(string(body), `"sync_verbose":true`)
	}
	if reported() {
		t.Fatal("a till that never had it reports it off")
	}

	on := true
	str := func(v string) *string { return &v }
	rate := int64(1)
	cmd := func(p updateSettingsCommandPayload) []byte {
		p.BranchName, p.RoundingMode, p.RoundingCurrency, p.ExchangeRate = str("Test"), str("half_up"), str("USD"), &rate
		raw, _ := json.Marshal(p)
		return raw
	}
	if err := app.executeUpdateSettingsCommand(app.ctx, cmd(updateSettingsCommandPayload{SyncVerbose: &on})); err != nil {
		t.Fatal(err)
	}
	if !app.flagEnabled(app.ctx, syncVerboseFlag) || !link.isVerbose() {
		t.Fatal("the command must turn it on, in the flag and in the live light")
	}
	if !reported() {
		t.Fatal("the till must report it on")
	}
	off := false
	if err := app.executeUpdateSettingsCommand(app.ctx, cmd(updateSettingsCommandPayload{SyncVerbose: &off})); err != nil {
		t.Fatal(err)
	}
	if link.isVerbose() || reported() {
		t.Fatal("and off again")
	}
}
