package main

import (
	"context"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func (k *fakeKashi) dialVirtual(ctx context.Context) (*websocket.Conn, int, error) {
	return newCentralClient(k.URL, "CT1", "key-1").DialLiveVirtual(ctx)
}

func (k *fakeKashi) lastURI() string {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.uris) == 0 {
		return ""
	}
	return k.uris[len(k.uris)-1]
}

func virtualRig(t *testing.T) (*liveView, *fakeKashi, *events) {
	k := newFakeKashi(t)
	ev := &events{}
	l := newVirtualLiveView(ev.emit)
	l.backoff = 10 * time.Millisecond
	l.setFeatures([]string{"live_view", "live_view_control"})
	t.Cleanup(l.stop)
	return l, k, ev
}

func TestVirtualDialsItsOwnStreamWithItsOwnEvents(t *testing.T) {
	l, k, ev := virtualRig(t)
	l.start(k.dialVirtual, false) // control is always on for the separate screen
	k.conn(t, 1)
	if got := k.lastURI(); got != "/branch/live?mode=virtual" {
		t.Fatalf("dialled %q", got)
	}
	waitFor(t, "start", func() bool { return ev.has(`live_virtual:start {"control":true}`) })
	for _, e := range ev.all() {
		if len(e) >= 9 && e[:9] == "live_view" {
			t.Fatalf("the separate screen emitted a mirror event: %v", ev.all())
		}
	}
	l.stop()
	waitFor(t, "stop", func() bool { return ev.has("live_virtual:stop") })
}

func TestMirrorStillDialsTheMirrorStream(t *testing.T) {
	l, k, _ := liveRig(t)
	l.start(k.dial, true)
	k.conn(t, 1)
	if got := k.lastURI(); got != "/branch/live" {
		t.Fatalf("mirror dialled %q", got)
	}
}

func TestVirtualIgnoredWithoutControlOffer(t *testing.T) {
	k := newFakeKashi(t)
	ev := &events{}
	l := newVirtualLiveView(ev.emit)
	l.setFeatures([]string{"live_view"}) // view yes, control no
	l.start(k.dialVirtual, true)
	time.Sleep(100 * time.Millisecond)
	if k.dials.Load() != 0 || len(ev.all()) != 0 || l.status().Active {
		t.Fatalf("dials=%d events=%v", k.dials.Load(), ev.all())
	}
}

func TestVirtualEndsWhenKashiStopsOfferingControl(t *testing.T) {
	l, k, ev := virtualRig(t)
	l.start(k.dialVirtual, true)
	k.conn(t, 1)
	waitFor(t, "start", func() bool { return ev.has(`live_virtual:start {"control":true}`) })
	l.setFeatures([]string{"live_view"})
	waitFor(t, "stop", func() bool { return ev.has("live_virtual:stop") })
	if l.status().Active {
		t.Fatal("still active after control was withdrawn")
	}
}

func TestMirrorAndVirtualDoNotCrossTalk(t *testing.T) {
	mirror, k, mev := liveRig(t)
	vev := &events{}
	virtual := newVirtualLiveView(vev.emit)
	virtual.backoff = 10 * time.Millisecond
	virtual.setFeatures([]string{"live_view", "live_view_control"})
	t.Cleanup(virtual.stop)

	mirror.start(k.dial, true)
	virtual.start(k.dialVirtual, true)
	k.conn(t, 2)
	k.mu.Lock()
	var vc *websocket.Conn
	for i, u := range k.uris {
		if u == "/branch/live?mode=virtual" {
			vc = k.conns[i]
		}
	}
	k.mu.Unlock()
	if vc == nil {
		t.Fatal("no separate screen stream")
	}
	waitFor(t, "both start", func() bool {
		return mev.has(`live_view:start {"control":true}`) && vev.has(`live_virtual:start {"control":true}`)
	})

	// Frames sent on one never reach the other's stream: each session has its own queue.
	if !mirror.send(`{"e":[],"a":[]}`) || !virtual.send(`{"full":true,"e":[],"a":[]}`) {
		t.Fatal("send failed")
	}
	got := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case f := <-k.frames:
			got[f] = true
		case <-time.After(2 * time.Second):
			t.Fatal("frames missing")
		}
	}
	if len(got) != 2 {
		t.Fatalf("frames: %v", got)
	}

	// A control for the separate screen reaches only its page.
	_ = vc.WriteMessage(websocket.TextMessage, []byte(`{"t":"key","key":"5"}`))
	waitFor(t, "virtual control", func() bool { return vev.has(`live_virtual:control "{\"t\":\"key\",\"key\":\"5\"}"`) })
	for _, e := range mev.all() {
		if len(e) > 17 && e[:17] == "live_view:control" {
			t.Fatalf("the mirror got the separate screen's control: %v", mev.all())
		}
	}

	// Stopping one leaves the other running.
	virtual.stop()
	if !mirror.status().Active {
		t.Fatal("stopping the separate screen stopped the mirror")
	}
}

func TestVirtualSessionOnlyWhileStreamIsUp(t *testing.T) {
	// App level: the hidden copy's sign-in exists exactly while its session does.
	k := newFakeKashi(t)
	app := newTestApp(t)
	app.liveVirtual = newVirtualLiveView(func(name string, data ...any) {
		switch name {
		case "live_virtual:start":
			app.setVirtualSession(true)
		case "live_virtual:stop":
			app.setVirtualSession(false)
		}
	})
	app.liveVirtual.backoff = 10 * time.Millisecond
	app.liveVirtual.setFeatures([]string{"live_view", "live_view_control"})
	app.liveVOnce.Do(func() {})
	if app.virtualSessionOn() {
		t.Fatal("on before start")
	}
	app.liveV().start(k.dialVirtual, true)
	waitFor(t, "session on", app.virtualSessionOn)
	if _, err := app.LiveVirtualCall("GetPINLockoutStatus", "[]"); err != nil {
		t.Fatalf("a call while open: %v", err)
	}
	app.liveV().stop()
	if app.virtualSessionOn() {
		t.Fatal("still on after stop")
	}
	if _, err := app.LiveVirtualCall("GetPINLockoutStatus", "[]"); err == nil {
		t.Fatal("a call after close must be refused")
	}
}
