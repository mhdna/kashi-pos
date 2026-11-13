package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"
)

const linkGraceGoldenPath = "frontend/src/lib/testdata/link_grace.json"

// TestLinkGraceGolden writes what the tracker reports through a blip and a
// real outage, so frontend/src/lib/linkStatus.test.ts can check the light
// stays calm through the blip and turns red only after linkDownGrace.
// Rewrite with: KASHI_UPDATE_GOLDEN=1 go test -run TestLinkGraceGolden .
func TestLinkGraceGolden(t *testing.T) {
	l, _ := newTestTracker()
	l.after = func(time.Duration, func()) {}
	clock := time.Date(2026, 11, 13, 22, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return clock }

	type step struct {
		Name   string     `json:"name"`
		AtUnix int64      `json:"atUnix"`
		Link   LinkStatus `json:"link"`
	}
	var steps []step
	snap := func(name string) {
		steps = append(steps, step{name, clock.Unix(), l.snapshot()})
	}

	l.kashiResult(nil)
	snap("answering")
	l.kashiResult(errors.New("dial tcp: i/o timeout"))
	snap("blip: one request failed")
	clock = clock.Add(linkDownGrace - time.Second)
	snap("blip: 29 s of silence")
	l.kashiResult(nil)
	snap("blip over: central answers")

	clock = clock.Add(time.Minute)
	l.kashiResult(errors.New("dial tcp: connection refused"))
	clock = clock.Add(linkDownGrace)
	snap("central down 30 s, internet fine")
	l.kashiResult(nil)

	clock = clock.Add(time.Minute)
	l.kashiResult(errors.New("dial tcp: no route to host"))
	clock = clock.Add(20 * time.Second)
	l.setInternet("down")
	snap("outage: 20 s in, probe failed")
	clock = clock.Add(10 * time.Second)
	snap("outage: 30 s in")

	got, err := json.MarshalIndent(steps, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if os.Getenv("KASHI_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(linkGraceGoldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(linkGraceGoldenPath)
	if err != nil {
		t.Fatalf("%v (write it with KASHI_UPDATE_GOLDEN=1)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("the link snapshots changed; check the light still reads right and rewrite %s with KASHI_UPDATE_GOLDEN=1:\n%s", linkGraceGoldenPath, got)
	}
}
