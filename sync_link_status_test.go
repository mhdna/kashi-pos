package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newTestTracker() (*linkTracker, *int32) {
	var emits int32
	l := &linkTracker{now: time.Now}
	l.setEmitter(func() { atomic.AddInt32(&emits, 1) })
	return l, &emits
}

func TestLinkKashiResultTracksReachability(t *testing.T) {
	l, _ := newTestTracker()
	if s := l.snapshot(); s.Kashi != "unknown" || s.Internet != "unknown" || s.Phase != phaseIdle {
		t.Fatalf("fresh tracker = %+v, want everything unknown and idle", s)
	}

	l.kashiResult(&url.Error{Op: "Get", URL: "http://k", Err: &net.OpError{Op: "dial", Err: errors.New("refused")}})
	s := l.snapshot()
	if s.Kashi != "down" || s.KashiError != "can't connect to the server" {
		t.Fatalf("after a failed request: %+v", s)
	}

	l.kashiResult(nil)
	s = l.snapshot()
	if s.Kashi != "up" || s.KashiError != "" || s.KashiLastOK == "" || s.KashiDownFor != 0 {
		t.Fatalf("after an answer: %+v", s)
	}
	if s.Internet != "up" {
		t.Fatalf("an answer from kashi proves the internet, got %q", s.Internet)
	}

	// The caller giving up says nothing about kashi.
	l.kashiResult(context.Canceled)
	if l.snapshot().Kashi != "up" {
		t.Fatal("a cancelled request must not mark kashi down")
	}
}

func TestLinkKashiDownForCountsFromTheFirstFailure(t *testing.T) {
	l, _ := newTestTracker()
	clock := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return clock }
	l.kashiResult(errors.New("boom"))
	clock = clock.Add(90 * time.Second)
	l.kashiResult(errors.New("boom again")) // a second failure doesn't restart the clock
	if got := l.snapshot().KashiDownFor; got != 90 {
		t.Fatalf("down for %ds, want 90", got)
	}
}

func TestFriendlyNetError(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{&net.DNSError{Err: "no such host", Name: "x"}, "can't find the server (no internet, or DNS is down)"},
		{context.DeadlineExceeded, "the server took too long to answer"},
		{&net.OpError{Op: "dial", Err: errors.New("connection refused")}, "can't connect to the server"},
		{errors.New("something else"), "something else"},
	}
	for _, c := range cases {
		if got := friendlyNetError(c.err); got != c.want {
			t.Errorf("friendlyNetError(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

func TestLinkLiveChannelStates(t *testing.T) {
	l, _ := newTestTracker()
	if s := l.snapshot(); s.Live || s.LiveEverUp {
		t.Fatalf("never connected: %+v", s)
	}
	l.wsDown(errors.New("dial: refused"), 8*time.Second)
	s := l.snapshot()
	if s.Live || s.LiveError == "" || s.LiveRetryAt == "" || s.LiveEverUp {
		t.Fatalf("a failed dial: %+v", s)
	}

	l.wsUp()
	s = l.snapshot()
	if !s.Live || !s.LiveEverUp || s.LiveSince == "" || s.LiveError != "" || s.LiveRetryAt != "" {
		t.Fatalf("connected: %+v", s)
	}

	clock := time.Now().Add(time.Minute)
	l.now = func() time.Time { return clock }
	l.wsBeat()
	if got := l.snapshot().LiveLastBeat; got != clock.UTC().Format(time.RFC3339) {
		t.Fatalf("beat = %q, want the new clock", got)
	}

	l.wsDown(errors.New("read: closed"), 2*time.Second)
	s = l.snapshot()
	if s.Live || !s.LiveEverUp || s.LiveError == "" {
		t.Fatalf("dropped: %+v", s)
	}
}

func TestLinkActivity(t *testing.T) {
	l, _ := newTestTracker()
	l.begin(phasePulling, "Products", 2, 21)
	l.setDetail("500 changes")
	s := l.snapshot()
	if s.Phase != phasePulling || s.Label != "Products" || s.Done != 2 || s.Total != 21 || s.Detail != "500 changes" {
		t.Fatalf("pulling: %+v", s)
	}
	// A push finishing must not wipe a pull that is running.
	l.end(phasePushing)
	if l.snapshot().Phase != phasePulling {
		t.Fatal("ending another phase cleared the pull")
	}
	l.begin(phasePushing, "Sending to kashi", 0, 4)
	l.progress(3, 4)
	if s := l.snapshot(); s.Phase != phasePushing || s.Done != 3 || s.Detail != "" {
		t.Fatalf("pushing: %+v (a new activity starts with no detail)", s)
	}
	l.end(phasePushing)
	if s := l.snapshot(); s.Phase != phaseIdle || s.Label != "" || s.Total != 0 {
		t.Fatalf("after end: %+v", s)
	}
}

func TestLinkPullAndPushTimestamps(t *testing.T) {
	l, _ := newTestTracker()
	l.pulled(errors.New("fetch changes: boom"))
	s := l.snapshot()
	if s.LastPullAt == "" || s.LastPullError != "fetch changes: boom" {
		t.Fatalf("a failed pull: %+v", s)
	}
	l.pulled(nil)
	if s = l.snapshot(); s.LastPullError != "" {
		t.Fatalf("a good pull clears the error: %+v", s)
	}
	if s.LastPushAt != "" {
		t.Fatal("no push yet")
	}
	l.pushed()
	if l.snapshot().LastPushAt == "" {
		t.Fatal("push time not set")
	}
}

// A burst of progress ticks reaches the screen as a couple of events, not one
// per tick, and the last state is never lost.
func TestLinkEmitsAreCoalesced(t *testing.T) {
	l, emits := newTestTracker()
	for i := 0; i < 200; i++ {
		l.progress(i, 200)
	}
	time.Sleep(linkEmitMinGap + 150*time.Millisecond)
	n := atomic.LoadInt32(emits)
	if n < 1 || n > 3 {
		t.Fatalf("200 progress ticks made %d emits, want 1 to 3", n)
	}
	l.progress(200, 200)
	time.Sleep(linkEmitMinGap + 150*time.Millisecond)
	if atomic.LoadInt32(emits) <= n {
		t.Fatal("a change after the burst never emitted")
	}
	if l.snapshot().Done != 200 {
		t.Fatal("last tick lost")
	}
}

// Every request through a central client's transport updates "kashi".
func TestLinkTransportReportsEveryRequest(t *testing.T) {
	link.kashiResult(nil) // known good to start with
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/err" {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{}`))
	}))
	client := newCentralClient(server.URL, "T", "k").httpClient

	resp, err := client.Get(server.URL + "/err")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if link.snapshot().Kashi != "up" {
		t.Fatal("a 500 is an answer; kashi is reachable")
	}

	server.Close()
	if _, err := client.Get(server.URL + "/gone"); err == nil {
		t.Fatal("expected a connection error")
	}
	if s := link.snapshot(); s.Kashi != "down" || s.KashiError == "" {
		t.Fatalf("a refused connection must mark kashi down: %+v", s)
	}
}

// The live channel state follows the real websocket session: up while it is
// connected, down once kashi goes away.
func TestLinkLiveFollowsTheWebsocket(t *testing.T) {
	var (
		mu    sync.Mutex
		conns []*websocket.Conn
	)
	up := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") == "websocket" {
			c, err := up.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	app := newVisitorTestApp(t)
	setCentralURL(t, app, server.URL)
	link.wsDown(nil, 0) // start from down, whatever earlier tests left

	app.startWSClient()
	t.Cleanup(func() {
		app.stopWSClient()
		app.backgroundWG.Wait()
	})
	waitFor(t, "live channel up", func() bool { return link.snapshot().Live })
	if s := link.snapshot(); s.LiveSince == "" || !s.LiveEverUp {
		t.Fatalf("connected but no since: %+v", s)
	}

	// kashi drops the socket (a restart, a proxy cutting it).
	mu.Lock()
	for _, c := range conns {
		c.Close()
	}
	mu.Unlock()
	waitFor(t, "live channel down after the server dropped it", func() bool { return !link.snapshot().Live })
	if link.snapshot().LiveError == "" {
		t.Fatal("a dropped session should say why")
	}
}

// A pull shows what it is fetching, moves through the entities, and goes idle.
func TestLinkPullShowsProgressAndEndsIdle(t *testing.T) {
	link.setVerbose(true) // verbose: every step is shown, found something or not
	t.Cleanup(func() { link.setVerbose(false) })
	var seen atomic.Value
	seen.Store(map[string]bool{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := link.snapshot()
		if s.Phase == phasePulling {
			m := map[string]bool{}
			for k, v := range seen.Load().(map[string]bool) {
				m[k] = v
			}
			m[fmt.Sprintf("%s %d/%d", s.Label, s.Done, s.Total)] = true
			seen.Store(m)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"items":[],"server_time":"2026-11-11T00:00:00Z"}`))
	}))
	defer server.Close()

	app, _, _, _ := setUpTestInvoiceApp(t, server.URL)
	client := newCentralClient(server.URL, "TEST01", "key")
	app.syncPullMu.Lock()
	app.runSyncPullLocked(app.ctx, client)
	app.syncPullMu.Unlock()

	got := seen.Load().(map[string]bool)
	if !got["Currencies 0/21"] || !got["Products 2/21"] {
		t.Fatalf("pull never showed its progress, saw %v", got)
	}
	s := link.snapshot()
	if s.Phase != phaseIdle || s.LastPullAt == "" {
		t.Fatalf("after the pass: %+v", s)
	}
}

func TestProbeInternetFalseWhenNothingAnswers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled: every dial fails at once
	if probeInternet(ctx) {
		t.Fatal("a cancelled probe can't reach anything")
	}
}

// While kashi is upgraded its proxy answers 502/503/504 (or Cloudflare 521-523).
// That is "being updated", not "not connected": the till says so, and it ends
// with the first normal answer.
func TestLinkUpdatingFollowsGatewayAnswers(t *testing.T) {
	for _, code := range []int{502, 503, 504, 521, 522, 523} {
		l, _ := newTestTracker()
		l.kashiResult(nil)
		l.kashiStatus(code)
		s := l.snapshot()
		if !s.Updating || s.Kashi != "up" {
			t.Fatalf("status %d: %+v, want updating with kashi still answering", code, s)
		}
		l.kashiStatus(200)
		if l.snapshot().Updating {
			t.Fatalf("status %d: a normal answer must end the update state", code)
		}
	}
}

func TestLinkUpdatingIgnoresOrdinaryStatuses(t *testing.T) {
	for _, code := range []int{200, 201, 204, 301, 400, 401, 403, 404, 409, 422, 429, 500, 501, 530} {
		l, _ := newTestTracker()
		l.kashiStatus(code)
		if l.snapshot().Updating {
			t.Fatalf("status %d is not an update", code)
		}
	}
}

func TestLinkUpdatingCountsFromTheFirstAnswerAndExpires(t *testing.T) {
	l, emits := newTestTracker()
	clock := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return clock }
	l.kashiStatus(502)
	clock = clock.Add(90 * time.Second)
	l.kashiStatus(503) // another gateway answer doesn't restart the clock
	if s := l.snapshot(); !s.Updating || s.UpdatingForSeconds != 90 {
		t.Fatalf("after 90 s: %+v", s)
	}
	// A refused connection in between (the proxy restarting too) keeps it.
	l.kashiResult(errors.New("refused"))
	if !l.snapshot().Updating {
		t.Fatal("a refused connection mid-update must not end the update state")
	}
	// Nothing newer for a long time: it is just down, say so.
	clock = clock.Add(linkUpdatingMaxAge + time.Second)
	if l.snapshot().Updating {
		t.Fatal("an update that never ends must stop being believed")
	}
	time.Sleep(100 * time.Millisecond) // the emit is a timer
	if atomic.LoadInt32(emits) == 0 {
		t.Fatal("starting an update must ask the screen to refetch")
	}
}

func TestLinkUpdatingEmitsOnlyOnChange(t *testing.T) {
	l, emits := newTestTracker()
	l.kashiStatus(200)
	time.Sleep(linkEmitMinGap + 50*time.Millisecond)
	before := atomic.LoadInt32(emits)
	l.kashiStatus(200)
	l.kashiStatus(200)
	time.Sleep(linkEmitMinGap + 50*time.Millisecond)
	if atomic.LoadInt32(emits) != before {
		t.Fatal("ordinary answers must not wake the screen")
	}
}

// Through the real transport and a real server: a 502 from the proxy, then a
// good answer once the new version is up.
func TestLinkTransportSeesAnUpgrade(t *testing.T) {
	saved := link
	link = &linkTracker{now: time.Now}
	defer func() { link = saved }()

	status := int32(http.StatusBadGateway)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(int(atomic.LoadInt32(&status)))
	}))
	defer srv.Close()
	client := &http.Client{Transport: linkTransport{base: http.DefaultTransport}}

	get := func() {
		resp, err := client.Get(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	get()
	if s := link.snapshot(); !s.Updating || s.Kashi != "up" {
		t.Fatalf("while the proxy says 502: %+v", s)
	}
	atomic.StoreInt32(&status, http.StatusOK)
	get()
	if s := link.snapshot(); s.Updating || s.Kashi != "up" {
		t.Fatalf("once kashi answers normally again: %+v", s)
	}
}

// The live link's handshake is refused with the same 502 while kashi restarts.
func TestLinkWebsocketRefusedByProxyMeansUpdating(t *testing.T) {
	saved := link
	link = &linkTracker{now: time.Now}
	defer func() { link = saved }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	c := newCentralClient(srv.URL, "CT", "key")
	if _, err := c.DialWS(context.Background()); err == nil {
		t.Fatal("a 502 handshake must fail")
	}
	if !link.snapshot().Updating {
		t.Fatal("a handshake refused with 502 means kashi is being updated")
	}
}
