package main

import (
	"sync"
	"testing"
	"time"
)

// Every combination of the idle check's inputs: a forced update installs
// silently whenever it is ready and supported, shift or not; a normal one
// only prompts when idle; nothing happens twice or on unsupported systems.
func TestInstallDecisionEveryCombination(t *testing.T) {
	states := []string{updateUpToDate, updateDownloading, updateReady, updateInstalling, updateFailed}
	for _, state := range states {
		for _, force := range []bool{false, true} {
			for _, idle := range []bool{false, true} {
				for _, prompted := range []bool{false, true} {
					for _, supported := range []bool{false, true} {
						s := updaterState{State: state, Force: force}
						prompt, silent := installDecision(s, idle, prompted, supported)
						live := state == updateReady && !prompted && supported
						wantSilent := live && force
						wantPrompt := live && !force && idle
						if prompt != wantPrompt || silent != wantSilent {
							t.Fatalf("state=%s force=%v idle=%v prompted=%v supported=%v: got prompt=%v silent=%v, want %v %v",
								state, force, idle, prompted, supported, prompt, silent, wantPrompt, wantSilent)
						}
						if prompt && silent {
							t.Fatalf("both prompt and silent for %+v", s)
						}
					}
				}
			}
		}
	}
}

func TestForcedQuitIsQuickerThanNormal(t *testing.T) {
	if quitDelay(true) >= quitDelay(false) {
		t.Fatalf("forced %v should be shorter than %v", quitDelay(true), quitDelay(false))
	}
}

func TestWaitForQuietSales(t *testing.T) {
	a := &App{}
	if !a.waitForQuietSales(time.Second) {
		t.Fatal("no sale in flight should be quiet at once")
	}

	// A sale in flight holds the restart until it ends.
	end := a.beginSale()
	done := make(chan bool)
	go func() { done <- a.waitForQuietSales(2 * time.Second) }()
	time.Sleep(150 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("waited through a sale that was still running")
	default:
	}
	end()
	if !<-done {
		t.Fatal("did not notice the sale ended")
	}

	// A sale that never ends: give up at the timeout, never hang.
	end = a.beginSale()
	defer end()
	start := time.Now()
	if a.waitForQuietSales(200 * time.Millisecond) {
		t.Fatal("reported quiet during a sale")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("wait overran its timeout")
	}
}

// Many overlapping sales: the counter returns to zero.
func TestSaleCounterBalances(t *testing.T) {
	a := &App{}
	var wg sync.WaitGroup
	for i := 0; i < 300; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer a.beginSale()()
			time.Sleep(time.Millisecond)
		}()
	}
	wg.Wait()
	if n := a.updater.sales.Load(); n != 0 {
		t.Fatalf("counter = %d after all sales ended", n)
	}
}
