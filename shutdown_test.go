package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The reported case: Close App left a black screen with the keys locked when
// something in the background was slow to stop. Closing now gives Windows
// back before waiting on anything, and finishes even if work never stops.
func TestShutdownLeavesKioskFirstAndNeverHangs(t *testing.T) {
	app, _, _, _ := setUpTestInvoiceApp(t, "http://localhost:9999")
	savedLeave, savedExit, savedWait, savedDeadline := leaveKioskNow, exitApp, backgroundStopWait, shutdownDeadline
	t.Cleanup(func() { leaveKioskNow, exitApp, backgroundStopWait, shutdownDeadline = savedLeave, savedExit, savedWait, savedDeadline })

	var left atomic.Int64
	leaveKioskNow = func() { left.Store(time.Now().UnixNano()) }
	var exited atomic.Bool
	exitApp = func(int) { exited.Store(true) }
	backgroundStopWait, shutdownDeadline = 150*time.Millisecond, 5*time.Second

	// A background job that never finishes.
	app.backgroundWG.Add(1)
	t.Cleanup(app.backgroundWG.Done)

	start := time.Now()
	app.shutdown(context.Background())
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("shutdown hung for %s", took)
	}
	if left.Load() == 0 || left.Load() > start.Add(100*time.Millisecond).UnixNano() {
		t.Fatal("kiosk mode wasn't left before waiting on background work")
	}
	if exited.Load() {
		t.Fatal("the watchdog fired though closing finished")
	}
}

// If closing itself is still stuck past the deadline, the app ends.
func TestShutdownWatchdogEndsTheApp(t *testing.T) {
	app, _, _, _ := setUpTestInvoiceApp(t, "http://localhost:9999")
	savedLeave, savedExit, savedWait, savedDeadline := leaveKioskNow, exitApp, backgroundStopWait, shutdownDeadline
	t.Cleanup(func() { leaveKioskNow, exitApp, backgroundStopWait, shutdownDeadline = savedLeave, savedExit, savedWait, savedDeadline })
	leaveKioskNow = func() {}
	exited := make(chan int, 1)
	exitApp = func(code int) { exited <- code }
	backgroundStopWait, shutdownDeadline = 400*time.Millisecond, 100*time.Millisecond
	app.backgroundWG.Add(1)
	t.Cleanup(app.backgroundWG.Done)

	app.shutdown(context.Background())
	select {
	case code := <-exited:
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
	default:
		t.Fatal("the watchdog didn't end a stuck close")
	}
}

func TestWaitTimeout(t *testing.T) {
	var wg sync.WaitGroup
	if !waitTimeout(&wg, time.Second) {
		t.Fatal("nothing to wait for, yet it gave up")
	}
	wg.Add(1)
	if waitTimeout(&wg, 50*time.Millisecond) {
		t.Fatal("a job that never ends was waited out")
	}
	wg.Done()
}
