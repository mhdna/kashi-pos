package main

import (
	"testing"

	db "kashi-pos/db/sqlc"
)

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

// lbpUnitsPerUsdMicros mirrors what the 000052 migration backfills for the
// LBP currency row from the historical default exchange rate of 90000 LBP
// per USD, scaled to the units_per_usd_micros fixed-point representation.
const lbpUnitsPerUsdMicros = int64(90000) * unitsPerUsdMicroScale
const lbpCashRoundingUnit = int64(10000)

// testLBPReceiptCurrency is shared by receipt-printing tests that need a
// resolved LBP currency without a live database.
var testLBPReceiptCurrency = receiptCurrency{
	Code: "LBP", Symbol: "L.L.",
	UnitsPerUsdMicros: lbpUnitsPerUsdMicros, CashRoundingUnit: lbpCashRoundingUnit,
}

// testLBPCurrency is the db.Currency equivalent, for tests exercising
// functions that take the raw sqlc model directly.
var testLBPCurrency = db.Currency{
	Code: "LBP", Symbol: "L.L.",
	UnitsPerUsdMicros: lbpUnitsPerUsdMicros, CashRoundingUnit: lbpCashRoundingUnit,
}

func TestRoundCurrencyUnitsFromCents(t *testing.T) {
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
			got, err := roundCurrencyUnitsFromCents(tt.cents, lbpUnitsPerUsdMicros, lbpCashRoundingUnit, tt.mode)
			if err != nil || got != tt.want {
				t.Fatalf("roundCurrencyUnitsFromCents(%d, ..., %q) = %d, %v; want %d", tt.cents, tt.mode, got, err, tt.want)
			}
		})
	}
}

func TestRoundCashEquivalentCents(t *testing.T) {
	got, err := roundCashEquivalentCents(996, lbpUnitsPerUsdMicros, lbpCashRoundingUnit, roundingHalfUp)
	if err != nil || got != 1000 {
		t.Fatalf("roundCashEquivalentCents(996, ...) = %d, %v; want 1000", got, err)
	}
}

func TestRoundCurrencyUnitsFromCentsReproducesWholeDollarUSDRounding(t *testing.T) {
	// USD's own rate/rounding-unit (1,000,000 micros per USD, round to the
	// nearest whole dollar) must reproduce exactly what the old hardcoded
	// USD branch of roundCentsByCurrency did: round cents to the nearest
	// 100-cent boundary.
	const usdUnitsPerUsdMicros = unitsPerUsdMicroScale
	const usdCashRoundingUnit = int64(1)
	tests := []struct {
		cents int64
		mode  string
		want  int64 // whole dollars
	}{
		{1049, roundingHalfUp, 10},
		{1050, roundingHalfUp, 11},
		{1099, roundingDown, 10},
		{1001, roundingUp, 11},
	}
	for _, tt := range tests {
		got, err := roundCurrencyUnitsFromCents(tt.cents, usdUnitsPerUsdMicros, usdCashRoundingUnit, tt.mode)
		if err != nil || got != tt.want {
			t.Fatalf("roundCurrencyUnitsFromCents(%d, USD, %q) = %d, %v; want %d", tt.cents, tt.mode, got, err, tt.want)
		}
	}
}

func TestParseSalesItemsRejectsUnsafeInput(t *testing.T) {
	for _, raw := range []string{
		`[]`,
		`[{"code":"A","qty":0}]`,
		`[{"code":"A","qty":-1}]`,
		`[{"code":"A","qty":1},{"code":"A","qty":1}]`,
	} {
		if _, err := parseSalesItems(raw, "UNKNOWN", "UNKNOWN"); err == nil {
			t.Fatalf("parseSalesItems(%s) unexpectedly succeeded", raw)
		}
	}
}
