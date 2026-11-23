package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// fakeKashi stands in for kashi's /branch/live: it records the headers the
// till dialed with and every frame it sent, and lets a test push frames back.
type fakeKashi struct {
	*httptest.Server
	status  atomic.Int32 // non-zero: refuse with this HTTP status
	dials   atomic.Int32
	closed  atomic.Int32 // streams that ended
	mu      sync.Mutex
	headers http.Header
	conns   []*websocket.Conn
	frames  chan string
	// never reads: a stalled kashi
	stall atomic.Bool
}

func newFakeKashi(t *testing.T) *fakeKashi {
	t.Helper()
	k := &fakeKashi{frames: make(chan string, 4096)}
	up := websocket.Upgrader{EnableCompression: true}
	k.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k.dials.Add(1)
		if s := k.status.Load(); s != 0 {
			http.Error(w, "nobody is watching", int(s))
			return
		}
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		k.mu.Lock()
		k.headers = r.Header.Clone()
		k.conns = append(k.conns, c)
		k.mu.Unlock()
		if k.stall.Load() {
			select {} // never read: the till's writes back up
		}
		for {
			_, data, err := c.ReadMessage()
			if err != nil {
				k.closed.Add(1)
				return
			}
			k.frames <- string(data)
		}
	}))
	t.Cleanup(k.Close)
	return k
}

func (k *fakeKashi) dial(ctx context.Context) (*websocket.Conn, int, error) {
	client := newCentralClient(k.URL, "CT1", "key-1")
	return client.DialLive(ctx)
}

func (k *fakeKashi) conn(t *testing.T, n int) *websocket.Conn {
	t.Helper()
	waitFor(t, "a stream connection", func() bool {
		k.mu.Lock()
		defer k.mu.Unlock()
		return len(k.conns) >= n
	})
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.conns[n-1]
}

// events records what the till tells its UI.
type events struct {
	mu   sync.Mutex
	list []string
}

func (e *events) emit(name string, data ...any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := name
	if len(data) > 0 {
		b, _ := json.Marshal(data[0])
		s += " " + string(b)
	}
	e.list = append(e.list, s)
}
func (e *events) all() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.list...)
}
func (e *events) has(s string) bool {
	for _, x := range e.all() {
		if x == s {
			return true
		}
	}
	return false
}

func liveRig(t *testing.T) (*liveView, *fakeKashi, *events) {
	k := newFakeKashi(t)
	ev := &events{}
	l := newLiveView(ev.emit)
	l.backoff = 10 * time.Millisecond
	l.setFeatures([]string{"live_view", "live_view_control"})
	t.Cleanup(l.stop)
	return l, k, ev
}

func TestLiveViewIgnoredUnlessKashiOffersIt(t *testing.T) {
	k := newFakeKashi(t)
	ev := &events{}
	l := newLiveView(ev.emit)
	// Never offered (feature off in kashi, or an older kashi): no socket, no event.
	l.start(k.dial, true)
	time.Sleep(100 * time.Millisecond)
	if k.dials.Load() != 0 || len(ev.all()) != 0 {
		t.Fatalf("dials=%d events=%v", k.dials.Load(), ev.all())
	}
	if l.send("{}") || l.status().Active {
		t.Fatal("nothing is running")
	}
}

func TestLiveViewStartsStreamsAndStops(t *testing.T) {
	l, k, ev := liveRig(t)
	l.start(k.dial, true)
	k.conn(t, 1)
	waitFor(t, "start event", func() bool { return ev.has(`live_view:start {"control":true}`) })

	// It dialed with this till's own credentials.
	k.mu.Lock()
	if k.headers.Get("X-Branch-Code") != "CT1" || k.headers.Get("X-Branch-Key") != "key-1" {
		t.Fatalf("headers %v", k.headers)
	}
	k.mu.Unlock()

	if !l.send(`{"full":true,"e":[1]}`) || !l.send(`{"e":[2]}`) {
		t.Fatal("send refused while streaming")
	}
	for _, want := range []string{`{"full":true,"e":[1]}`, `{"e":[2]}`} {
		select {
		case got := <-k.frames:
			if got != want {
				t.Fatalf("got %s want %s", got, want)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("frame never arrived")
		}
	}
	if s := l.status(); !s.Active || !s.Control {
		t.Fatalf("status %+v", s)
	}

	l.stop()
	if !ev.has("live_view:stop") {
		t.Fatal("UI not told to stop")
	}
	if l.send("{}") {
		t.Fatal("send must refuse after stop")
	}
	waitFor(t, "socket closed", func() bool { return k.closed.Load() == 1 })
	l.stop() // a second stop is harmless
}

func TestLiveViewStartTwiceKeepsOneStream(t *testing.T) {
	l, k, ev := liveRig(t)
	l.start(k.dial, true)
	k.conn(t, 1)
	waitFor(t, "start", func() bool { return ev.has(`live_view:start {"control":true}`) })
	l.start(k.dial, false)
	waitFor(t, "mode update", func() bool { return ev.has(`live_view:mode {"control":false}`) })
	time.Sleep(50 * time.Millisecond)
	if k.dials.Load() != 1 {
		t.Fatalf("dials=%d", k.dials.Load())
	}
	if l.status().Control {
		t.Fatal("control should be off now")
	}
}

func TestLiveViewControlOnlyWhenAllowed(t *testing.T) {
	l, k, ev := liveRig(t)
	l.start(k.dial, true)
	c := k.conn(t, 1)
	waitFor(t, "start", func() bool { return ev.has(`live_view:start {"control":true}`) })

	click := `{"t":"click","id":7}`
	_ = c.WriteMessage(websocket.TextMessage, []byte(click))
	waitFor(t, "control event", func() bool { return ev.has(`live_view:control "{\"t\":\"click\",\"id\":7}"`) })
	_ = c.WriteMessage(websocket.TextMessage, []byte(`{"t":"snapshot"}`))
	waitFor(t, "snapshot event", func() bool { return ev.has("live_view:snapshot") })

	// Not understood: dropped.
	_ = c.WriteMessage(websocket.TextMessage, []byte(`{"t":"eval","code":"x"}`))
	_ = c.WriteMessage(websocket.TextMessage, []byte(`not json`))

	// Kashi turns control off: the same click is ignored.
	l.start(k.dial, false)
	waitFor(t, "mode", func() bool { return ev.has(`live_view:mode {"control":false}`) })
	before := len(ev.all())
	_ = c.WriteMessage(websocket.TextMessage, []byte(click))
	_ = c.WriteMessage(websocket.TextMessage, []byte(`{"t":"snapshot"}`))
	waitFor(t, "second snapshot", func() bool { return len(ev.all()) > before })
	for _, e := range ev.all()[before:] {
		if strings.HasPrefix(e, "live_view:control") {
			t.Fatalf("a click got through with control off: %v", ev.all())
		}
	}
}

func TestLiveViewControlNeverOnWhenKashiDoesNotOfferIt(t *testing.T) {
	k := newFakeKashi(t)
	ev := &events{}
	l := newLiveView(ev.emit)
	l.setFeatures([]string{"live_view"}) // view only
	t.Cleanup(l.stop)
	l.start(k.dial, true)
	k.conn(t, 1)
	waitFor(t, "start", func() bool { return ev.has(`live_view:start {"control":false}`) })
}

func TestLiveViewRefusedMeansNobodyWatches(t *testing.T) {
	l, k, ev := liveRig(t)
	k.status.Store(http.StatusConflict)
	l.start(k.dial, false)
	waitFor(t, "stop", func() bool { return ev.has("live_view:stop") })
	time.Sleep(100 * time.Millisecond)
	if k.dials.Load() != 1 {
		t.Fatalf("redialed after a refusal: %d", k.dials.Load())
	}
	if l.status().Active {
		t.Fatal("still active")
	}
}

func TestLiveViewRedialsAfterADropAndAsksTheUIForASnapshot(t *testing.T) {
	l, k, ev := liveRig(t)
	l.start(k.dial, false)
	k.conn(t, 1).Close()
	k.conn(t, 2)
	waitFor(t, "snapshot after redial", func() bool { return ev.has("live_view:snapshot") })
	if n := strings.Count(strings.Join(ev.all(), "|"), "live_view:start"); n != 1 {
		t.Fatalf("start emitted %d times", n)
	}
	if !l.send(`{"full":true}`) {
		t.Fatal("send should work on the new stream")
	}
}

func TestLiveViewFeatureGoingAwayStopsIt(t *testing.T) {
	l, k, ev := liveRig(t)
	l.start(k.dial, false)
	k.conn(t, 1)
	waitFor(t, "start", func() bool { return ev.has(`live_view:start {"control":false}`) })
	l.setFeatures(nil) // the owner turned live view off
	if !ev.has("live_view:stop") || l.status().Active {
		t.Fatal("still running after kashi turned it off")
	}
	l.start(k.dial, false) // and it cannot be started again
	time.Sleep(50 * time.Millisecond)
	if k.dials.Load() != 1 {
		t.Fatal("dialed after the feature went away")
	}
}

func TestLiveViewStopsByItselfWhenKashiGoesSilent(t *testing.T) {
	l, k, ev := liveRig(t)
	l.silence = 150 * time.Millisecond
	l.start(k.dial, false)
	k.conn(t, 1)
	waitFor(t, "silence stop", func() bool { return ev.has("live_view:stop") })
	if l.status().Active {
		t.Fatal("still active after silence")
	}
}

// The risk that matters most: a stalled kashi must never slow the till.
func TestLiveViewSendNeverBlocksOnAStalledKashi(t *testing.T) {
	l, k, ev := liveRig(t)
	k.stall.Store(true)
	l.start(k.dial, false)
	k.conn(t, 1)
	waitFor(t, "start", func() bool { return ev.has(`live_view:start {"control":false}`) })

	big := `{"e":["` + strings.Repeat("x", 1<<20) + `"]}`
	start := time.Now()
	dropped := 0
	for i := 0; i < 500; i++ {
		if !l.send(big) {
			dropped++
		}
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("send waited on a stalled kashi: %v", time.Since(start))
	}
	if dropped == 0 {
		t.Fatal("a stalled kashi must make send drop batches, telling the UI to resnapshot")
	}
}

func TestLiveViewSendRefusesOversizeAndWhenIdle(t *testing.T) {
	l, k, ev := liveRig(t)
	if l.send("{}") {
		t.Fatal("idle send must refuse")
	}
	l.start(k.dial, false)
	k.conn(t, 1)
	waitFor(t, "start", func() bool { return ev.has(`live_view:start {"control":false}`) })
	if l.send(strings.Repeat("x", liveMaxBatch+1)) {
		t.Fatal("oversize batch accepted")
	}
	if !l.send("{}") {
		t.Fatal("normal batch refused")
	}
}

// handleWSMessage routes kashi's messages: hello announces the feature,
// live_view_start/stop/settings drive the session.
func TestHandleWSMessageLiveView(t *testing.T) {
	k := newFakeKashi(t)
	ev := &events{}
	a := &App{}
	a.liveStream = newLiveView(ev.emit)
	a.liveStream.backoff = 10 * time.Millisecond
	client := newCentralClient(k.URL, "CT1", "key-1")
	t.Cleanup(a.liveStream.stop)

	// Before any hello: start is ignored.
	a.handleWSMessage(client, []byte(`{"type":"live_view_start"}`))
	time.Sleep(50 * time.Millisecond)
	if k.dials.Load() != 0 {
		t.Fatal("started without kashi offering it")
	}

	a.handleWSMessage(client, []byte(`{"type":"hello","features":["sync_changed","live_view","live_view_control"]}`))
	a.handleWSMessage(client, []byte(`{"type":"live_view_start","control":true}`))
	k.conn(t, 1)
	waitFor(t, "start", func() bool { return ev.has(`live_view:start {"control":true}`) })
	if s := a.LiveViewStatus(); !s.Active || !s.Control {
		t.Fatalf("status %+v", s)
	}
	if !a.LiveViewSend(`{"full":true}`) {
		t.Fatal("binding refused a batch")
	}

	a.handleWSMessage(client, []byte(`{"type":"live_view_stop"}`))
	if a.LiveViewStatus().Active || a.LiveViewSend("{}") {
		t.Fatal("still running after stop")
	}

	// kashi turning the setting off mid-session.
	a.handleWSMessage(client, []byte(`{"type":"live_view_start"}`))
	k.conn(t, 2)
	a.handleWSMessage(client, []byte(`{"type":"live_view_settings"}`))
	if a.LiveViewStatus().Active {
		t.Fatal("settings message without the feature must stop it")
	}
}
