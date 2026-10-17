package main

import (
	"testing"
	"time"
)

func TestClockStatus(t *testing.T) {
	saved := osZone
	defer func() { osZone = saved }()
	beirut, err := time.LoadLocation("Asia/Beirut")
	if err != nil {
		t.Fatal(err)
	}
	summer := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC) // Beirut is UTC+3

	osZone = beirut
	got := clockStatus("Asia/Beirut", 30*time.Second, summer)
	if got.ZoneMismatch || got.ClockOff || got.DriftSeconds != 30 || got.PCZone != "UTC+03:00" {
		t.Fatalf("PC on Beirut time, 30s fast: %+v", got)
	}

	osZone = time.UTC
	got = clockStatus("Asia/Beirut", -5*time.Minute, summer)
	if !got.ZoneMismatch || !got.ClockOff || got.DriftSeconds != -300 || got.PCZone != "UTC+00:00" {
		t.Fatalf("PC on UTC, 5 min slow: %+v", got)
	}

	// An older kashi sends no zone: nothing to compare the PC's against.
	got = clockStatus("", 0, summer)
	if got.ZoneMismatch {
		t.Fatalf("no branch zone: %+v", got)
	}
}

func TestUTCOffset(t *testing.T) {
	for in, want := range map[int]string{0: "UTC+00:00", 10800: "UTC+03:00", -16200: "UTC-04:30"} {
		if got := utcOffset(in); got != want {
			t.Errorf("utcOffset(%d) = %q, want %q", in, got, want)
		}
	}
}
