package main

import (
	"math/rand"
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

// Round up left exact amounts alone only in roundRatio: the currency
// conversions added one anyway, so $100 came to LL 9,010,000 and
// LL 90,000 to 101¢.
func TestRoundUpLeavesExactAmountsAlone(t *testing.T) {
	const lbp90k = 90_000 * unitsPerUsdMicroScale
	if got, err := roundCurrencyUnitsFromCents(10_000, lbp90k, 10_000, roundingUp); err != nil || got != 9_000_000 {
		t.Fatalf("$100 in LL, rounding up = %d, %v; want 9,000,000", got, err)
	}
	if got, err := roundCentsFromCurrencyUnits(90_000, lbp90k, roundingUp); err != nil || got != 100 {
		t.Fatalf("LL 90,000 in cents, rounding up = %d, %v; want 100", got, err)
	}
	if got, err := roundCurrencyUnitsFromCents(1000, unitsPerUsdMicroScale, 1, roundingUp); err != nil || got != 10 {
		t.Fatalf("$10.00 to whole dollars, rounding up = %d, %v; want 10", got, err)
	}
	if got, err := roundCashEquivalentCents(10_000, lbp90k, 10_000, roundingUp); err != nil || got != 10_000 {
		t.Fatalf("$100 rounded to LL notes, rounding up = %d, %v; want 10,000", got, err)
	}
}

// Every mode, hundreds of amounts: each conversion is the true value
// rounded the mode's way, never more than one step from it, and an exact
// amount is unchanged.
func TestCurrencyRoundingModesRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for _, rate := range []int64{90_000, 89_500, 1} {
		micros := rate * unitsPerUsdMicroScale
		for n := 0; n < 500; n++ {
			units := rng.Int63n(50_000_000)
			for _, mode := range []string{roundingHalfUp, roundingDown, roundingUp} {
				got, err := roundCentsFromCurrencyUnits(units, micros, mode)
				if err != nil {
					t.Fatal(err)
				}
				// The true value is units*100/rate; got must be its floor or ceiling.
				num, den := units*100, rate
				floor, rem := num/den, num%den
				want := floor
				switch {
				case mode == roundingUp && rem != 0:
					want = floor + 1
				case mode == roundingHalfUp && 2*rem >= den:
					want = floor + 1
				}
				if got != want {
					t.Fatalf("LL %d at %d, %s = %d¢, want %d¢", units, rate, mode, got, want)
				}
			}
		}
	}
}
