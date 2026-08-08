package main

import "testing"

func TestRoundRatio(t *testing.T) {
	tests := []struct {
		name string
		n, d int64
		mode string
		want int64
	}{
		{"half up below midpoint", 149, 100, roundingHalfUp, 1},
		{"half up midpoint", 150, 100, roundingHalfUp, 2},
		{"down", 199, 100, roundingDown, 1},
		{"up", 101, 100, roundingUp, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := roundRatio(tt.n, tt.d, tt.mode); got != tt.want {
				t.Fatalf("roundRatio(%d, %d, %q) = %d, want %d", tt.n, tt.d, tt.mode, got, tt.want)
			}
		})
	}
}

func TestRoundFloatToCents(t *testing.T) {
	tests := []struct {
		value float64
		mode  string
		want  int64
	}{
		{1.234, roundingHalfUp, 123},
		{1.235, roundingHalfUp, 124},
		{1.239, roundingDown, 123},
		{1.231, roundingUp, 124},
	}
	for _, tt := range tests {
		got, err := roundFloatToCents(tt.value, tt.mode)
		if err != nil || got != tt.want {
			t.Fatalf("roundFloatToCents(%v, %q) = %d, %v; want %d", tt.value, tt.mode, got, err, tt.want)
		}
	}
}

func TestRoundLBPFromCents(t *testing.T) {
	const rate = int64(90000)
	tests := []struct {
		name  string
		cents int64
		mode  string
		want  int64
	}{
		{"half up below midpoint", 5, roundingHalfUp, 0},
		{"half up midpoint", 6, roundingHalfUp, 10000},
		{"down", 11, roundingDown, 0},
		{"up", 1, roundingUp, 10000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := roundLBPFromCents(tt.cents, rate, tt.mode)
			if err != nil || got != tt.want {
				t.Fatalf("roundLBPFromCents(%d, %d, %q) = %d, %v; want %d", tt.cents, rate, tt.mode, got, err, tt.want)
			}
		})
	}
}

func TestRoundCashEquivalentCents(t *testing.T) {
	got, err := roundCashEquivalentCents(996, 90000, roundingHalfUp)
	if err != nil || got != 1000 {
		t.Fatalf("roundCashEquivalentCents(996, 90000) = %d, %v; want 1000", got, err)
	}
}

func TestParseSalesItemsRejectsUnsafeInput(t *testing.T) {
	for _, raw := range []string{
		`[]`,
		`[{"code":"A","qty":0}]`,
		`[{"code":"A","qty":-1}]`,
		`[{"code":"A","qty":1},{"code":"A","qty":1}]`,
	} {
		if _, err := parseSalesItems(raw); err == nil {
			t.Fatalf("parseSalesItems(%s) unexpectedly succeeded", raw)
		}
	}
}
