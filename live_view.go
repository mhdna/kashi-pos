package main

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Live view of this till: while an admin watches it in kashi, the UI records
// its own page (rrweb, loaded only then) and this file carries the frames to
// kashi over a socket of their own, /branch/live. Docs: docs/dev/live-view.md.
//
// Nothing here runs unless kashi has switched the feature on and someone is
// watching: with it off kashi never says "live_view" in its hello, so start is
// ignored and no socket opens. The frontend never talks to kashi directly; the
// branch credentials stay on this side.

const (
	// liveSilence is how long the stream may hear nothing from kashi (which
	// pings every 30 seconds) before the till stops sharing on its own.
	liveSilence    = 2 * time.Minute
	liveQueue      = 64
	liveMaxBatch   = 8 << 20
	liveWriteWait  = 10 * time.Second
	liveBackoffMin = time.Second
	liveBackoffMax = 8 * time.Second
)

// liveView is the till's side of one live session. The seams (dial, emit and
// the timings) are fields so tests run it against a stand-in kashi.
type liveView struct {
	emit func(name string, data ...any)
	// prefix names the events this session emits ("live_view" for the mirror,
	// "live_virtual" for the separate screen); virtual sessions also need
	// kashi to offer control and always run with it.
	prefix  string
	virtual bool
	silence time.Duration
	backoff time.Duration

	mu sync.Mutex
	// what kashi's hello and "live_view_settings" said it offers.
	offersView, offersControl bool
	active                    bool
	control                   bool
	cancel                    context.CancelFunc
	conn                      *websocket.Conn
	out                       chan []byte
	// started: the UI was told to start recording for this session.
	started bool
}

func newLiveView(emit func(string, ...any)) *liveView {
	return &liveView{emit: emit, prefix: "live_view", silence: liveSilence, backoff: liveBackoffMin}
}

// newVirtualLiveView is the separate screen's session: its own events, and
// only while kashi offers control (it is read and write).
func newVirtualLiveView(emit func(string, ...any)) *liveView {
	l := newLiveView(emit)
	l.prefix, l.virtual = "live_virtual", true
	return l
}

// event is an event name for this session, e.g. "live_view:start".
func (l *liveView) event(name string) string { return l.prefix + ":" + name }

// liveDial opens the stream and returns the HTTP status of a refusal (409:
// nobody is watching any more).
type liveDial func(ctx context.Context) (*websocket.Conn, int, error)

// LiveViewStatus is what the UI asks for when it loads, to resume a session
// that began while it was reloading.
type LiveViewStatus struct {
	Active  bool `json:"active"`
	Control bool `json:"control"`
}

// setFeatures records what kashi offers. Losing the feature ends a session.
func (l *liveView) setFeatures(features []string) {
	l.mu.Lock()
	l.offersView = slices.Contains(features, "live_view")
	l.offersControl = l.offersView && slices.Contains(features, "live_view_control")
	l.control = l.control && l.offersControl
	off := !l.offersView && l.active
	if l.virtual && !l.offersControl && l.active {
		off = true
	}
	l.mu.Unlock()
	if off {
		l.stop()
	}
}

// start begins sharing, or only updates whether control is allowed when a
// session is already running. Ignored unless kashi offers the feature.
func (l *liveView) start(dial liveDial, control bool) {
	l.mu.Lock()
	if !l.offersView || (l.virtual && !l.offersControl) {
		l.mu.Unlock()
		return
	}
	if l.virtual {
		control = true
	}
	l.control = control && l.offersControl
	if l.active {
		c := l.control
		l.mu.Unlock()
		l.emit(l.event("mode"), map[string]any{"control": c})
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	l.active, l.cancel, l.started = true, cancel, false
	l.mu.Unlock()
	go l.run(ctx, dial)
}

// stop ends the session and tells the UI to stop recording.
func (l *liveView) stop() {
	l.mu.Lock()
	if !l.active {
		l.mu.Unlock()
		return
	}
	l.active, l.control, l.started = false, false, false
	l.cancel()
	conn := l.conn
	l.conn, l.out = nil, nil
	l.mu.Unlock()
	if conn != nil {
		conn.Close()
	}
	l.emit(l.event("stop"))
}

func (l *liveView) status() LiveViewStatus {
	l.mu.Lock()
	defer l.mu.Unlock()
	return LiveViewStatus{Active: l.active && l.started, Control: l.control}
}

// send queues one batch of recorded frames. It never waits: when the stream
// cannot keep up the batch is dropped and false tells the UI to take a full
// snapshot instead, so the picture heals rather than the till slowing down.
func (l *liveView) send(batch string) bool {
	if len(batch) > liveMaxBatch {
		return false
	}
	l.mu.Lock()
	out := l.out
	l.mu.Unlock()
	if out == nil {
		return false
	}
	select {
	case out <- []byte(batch):
		return true
	default:
		return false
	}
}

// run keeps a stream open for as long as the session is active, redialing
// after a drop, until kashi says nobody watches (409) or the session stops.
func (l *liveView) run(ctx context.Context, dial liveDial) {
	wait := l.backoff
	for ctx.Err() == nil {
		conn, status, err := dial(ctx)
		if status == http.StatusConflict {
			l.stop()
			return
		}
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			wait = min(wait*2, liveBackoffMax)
			continue
		}
		wait = l.backoff

		out := make(chan []byte, liveQueue)
		l.mu.Lock()
		if !l.active || ctx.Err() != nil {
			l.mu.Unlock()
			conn.Close()
			return
		}
		l.conn, l.out = conn, out
		first := !l.started
		l.started = true
		control := l.control
		l.mu.Unlock()
		// The UI starts recording only once the stream is up, so the first
		// snapshot is not lost; after a redial it takes a fresh one.
		if first {
			l.emit(l.event("start"), map[string]any{"control": control})
		} else {
			l.emit(l.event("snapshot"))
		}

		if silent := l.serve(conn, out); silent {
			l.stop()
			return
		}
		l.mu.Lock()
		if l.conn == conn {
			l.conn, l.out = nil, nil
		}
		l.mu.Unlock()
	}
}

// serve writes queued batches and reads kashi's messages until the socket
// ends. It reports true when kashi went silent for liveSilence.
func (l *liveView) serve(conn *websocket.Conn, out chan []byte) (silent bool) {
	done := make(chan struct{})
	defer close(done)
	defer conn.Close()
	go func() {
		for {
			select {
			case <-done:
				return
			case b := <-out:
				_ = conn.SetWriteDeadline(time.Now().Add(liveWriteWait))
				if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
					conn.Close()
					return
				}
			}
		}
	}()

	conn.SetReadLimit(64 << 10)
	arm := func() { _ = conn.SetReadDeadline(time.Now().Add(l.silence)) }
	arm()
	conn.SetPingHandler(func(data string) error {
		arm()
		return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(liveWriteWait))
	})
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
				return true
			}
			return false
		}
		arm()
		l.fromKashi(data)
	}
}

// fromKashi handles a message on the stream: a snapshot request, or a
// watcher's click, typing or key press (only while control is allowed).
func (l *liveView) fromKashi(data []byte) {
	var m struct {
		T string `json:"t"`
	}
	if json.Unmarshal(data, &m) != nil {
		return
	}
	switch m.T {
	case "snapshot":
		l.emit(l.event("snapshot"))
	case "click", "input", "key":
		l.mu.Lock()
		allowed := l.active && l.control
		l.mu.Unlock()
		if allowed {
			l.emit(l.event("control"), string(data))
		}
	}
}

// DialLive opens the stream to kashi with this till's branch credentials.
func (c *centralClient) DialLive(ctx context.Context) (*websocket.Conn, int, error) {
	return c.dialLive(ctx, "/branch/live")
}

// DialLiveVirtual opens the separate screen's own stream.
func (c *centralClient) DialLiveVirtual(ctx context.Context) (*websocket.Conn, int, error) {
	return c.dialLive(ctx, "/branch/live?mode=virtual")
}

func (c *centralClient) dialLive(ctx context.Context, path string) (*websocket.Conn, int, error) {
	if !c.enabled() {
		return nil, 0, errCentralSyncDisabled
	}
	wsURL := strings.Replace(c.baseURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	header := http.Header{}
	header.Set("X-Branch-Code", c.branchCode)
	header.Set("X-Branch-Key", c.apiKey)

	dialer := *websocket.DefaultDialer
	dialer.EnableCompression = true
	conn, resp, err := dialer.DialContext(ctx, wsURL+path, header)
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	if err != nil {
		return nil, status, err
	}
	conn.EnableWriteCompression(true)
	return conn, status, nil
}

// LiveViewSend is called by the UI with a batch of recorded frames (JSON, with
// "full":true as its first key when it holds a full snapshot). False means it
// was dropped: take a full snapshot.
func (a *App) LiveViewSend(batch string) bool {
	return a.live().send(batch)
}

// LiveViewStatus tells a reloaded UI whether a session is running.
func (a *App) LiveViewStatus() LiveViewStatus {
	return a.live().status()
}

// live is the till's live view, built on first use.
func (a *App) live() *liveView {
	a.liveOnce.Do(func() {
		if a.liveStream == nil {
			a.liveStream = newLiveView(a.emitAttendanceEvent)
		}
	})
	return a.liveStream
}

// LiveVirtualSend is LiveViewSend for the separate screen's own stream.
func (a *App) LiveVirtualSend(batch string) bool {
	return a.liveV().send(batch)
}

// LiveVirtualStatus tells a reloaded UI whether the separate screen is open.
func (a *App) LiveVirtualStatus() LiveViewStatus {
	return a.liveV().status()
}

// liveV is the separate screen's session, built on first use. Its sign-in
// (live_virtual_call.go) starts and ends with the session.
func (a *App) liveV() *liveView {
	a.liveVOnce.Do(func() {
		if a.liveVirtual == nil {
			a.liveVirtual = newVirtualLiveView(func(name string, data ...any) {
				switch name {
				case "live_virtual:start":
					a.setVirtualSession(true)
				case "live_virtual:stop":
					a.setVirtualSession(false)
				}
				a.emitAttendanceEvent(name, data...)
			})
		}
	})
	return a.liveVirtual
}

// handleLiveMessage applies kashi's live view messages; it reports whether
// msg was one of them.
func (a *App) handleLiveMessage(client *centralClient, msg wsMessage) bool {
	switch msg.Type {
	case "live_view_start":
		if msg.Virtual {
			a.liveV().start(client.DialLiveVirtual, true)
		} else {
			a.live().start(client.DialLive, msg.Control)
		}
	case "live_view_stop":
		if msg.Virtual {
			a.liveV().stop()
		} else {
			a.live().stop()
		}
	case "live_view_settings":
		a.live().setFeatures(msg.Features)
		a.liveV().setFeatures(msg.Features)
	default:
		return false
	}
	return true
}
