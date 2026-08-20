package main

import (
	"fmt"
	"math"
	"math/big"
)

const (
	roundingHalfUp = "half_up"
	roundingDown   = "down"
	roundingUp     = "up"
)

// unitsPerUsdMicroScale is the fixed-point scale used by
// currencies.units_per_usd_micros: that column stores how many micro-units
// (1/1,000,000) of a currency equal one USD, so both weaker and
// stronger-than-USD currencies can be represented as a plain integer with
// no floating point precision loss.
const unitsPerUsdMicroScale = 1_000_000

func normalizeRoundingMode(mode string) (string, error) {
	if mode == "" {
		return roundingHalfUp, nil
	}
	switch mode {
	case roundingHalfUp, roundingDown, roundingUp:
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported rounding mode %q", mode)
	}
}

func roundRatio(numerator, denominator int64, mode string) int64 {
	if numerator <= 0 {
		return 0
	}
	quotient, remainder := numerator/denominator, numerator%denominator
	switch mode {
	case roundingUp:
		if remainder != 0 {
			quotient++
		}
	case roundingHalfUp:
		if remainder*2 >= denominator {
			quotient++
		}
	}
	return quotient
}

func roundFloatToCents(value float64, mode string) (int64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0, fmt.Errorf("invalid monetary value")
	}
	if value > float64(math.MaxInt64)/100 {
		return 0, fmt.Errorf("monetary value is too large")
	}
	scaled := value * 100
	switch mode {
	case roundingDown:
		return int64(math.Floor(scaled + 1e-9)), nil
	case roundingUp:
		return int64(math.Ceil(scaled - 1e-9)), nil
	default:
		return int64(math.Floor(scaled + 0.5)), nil
	}
}

// applyRoundingRemainder bumps a big.Int quotient by one whenever the
// configured mode calls for rounding away from zero: always for "up", or
// for "half_up" when the remainder is at least half of the denominator.
func applyRoundingRemainder(quotient, remainder, denominator *big.Int, mode string) {
	if mode == roundingUp || (mode == roundingHalfUp && new(big.Int).Mul(remainder, big.NewInt(2)).Cmp(denominator) >= 0) {
		quotient.Add(quotient, big.NewInt(1))
	}
}

// roundCurrencyUnitsFromCents converts an accounting (USD cents) amount into
// a currency's major units, rounded to that currency's configured cash
// rounding unit. unitsPerUsdMicros and cashRoundingUnit come from a
// currencies row (Currency.UnitsPerUsdMicros/CashRoundingUnit) - this
// function is currency-agnostic: plugging in USD's own rate/unit
// (1,000,000 / 1) reproduces "round to the nearest whole dollar", and any
// other currency's own configured rate/unit works the same way. The
// conversion is integer-based so the printed amount cannot drift from the
// amount used by checkout.
func roundCurrencyUnitsFromCents(cents, unitsPerUsdMicros, cashRoundingUnit int64, mode string) (int64, error) {
	if cents < 0 || unitsPerUsdMicros <= 0 || cashRoundingUnit <= 0 {
		return 0, fmt.Errorf("invalid currency conversion")
	}
	normalizedMode, err := normalizeRoundingMode(mode)
	if err != nil {
		return 0, err
	}
	numerator := new(big.Int).Mul(big.NewInt(cents), big.NewInt(unitsPerUsdMicros))
	denominator := big.NewInt(100 * unitsPerUsdMicroScale * cashRoundingUnit)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	applyRoundingRemainder(quotient, remainder, denominator, normalizedMode)
	quotient.Mul(quotient, big.NewInt(cashRoundingUnit))
	if !quotient.IsInt64() {
		return 0, fmt.Errorf("currency value is too large")
	}
	return quotient.Int64(), nil
}

// roundCentsFromCurrencyUnits converts a physically counted amount in a
// currency's major units back to the accounting currency, so mixed-currency
// shift totals can be reconciled. This is the inverse of
// roundCurrencyUnitsFromCents (without the cash-rounding-unit step).
func roundCentsFromCurrencyUnits(units, unitsPerUsdMicros int64, mode string) (int64, error) {
	if units < 0 || unitsPerUsdMicros <= 0 {
		return 0, fmt.Errorf("invalid currency conversion")
	}
	normalizedMode, err := normalizeRoundingMode(mode)
	if err != nil {
		return 0, err
	}
	numerator := new(big.Int).Mul(big.NewInt(units), big.NewInt(100*unitsPerUsdMicroScale))
	denominator := big.NewInt(unitsPerUsdMicros)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	applyRoundingRemainder(quotient, remainder, denominator, normalizedMode)
	if !quotient.IsInt64() {
		return 0, fmt.Errorf("currency value is too large")
	}
	return quotient.Int64(), nil
}

// roundCashEquivalentCents converts an accounting amount to its physical
// cash-rounded value in the given currency and then back to accounting
// cents. This is the canonical amount to store when a cash payment in that
// currency is rounded at checkout.
func roundCashEquivalentCents(cents, unitsPerUsdMicros, cashRoundingUnit int64, mode string) (int64, error) {
	if cents < 0 {
		return 0, fmt.Errorf("invalid cash conversion")
	}
	roundedUnits, err := roundCurrencyUnitsFromCents(cents, unitsPerUsdMicros, cashRoundingUnit, mode)
	if err != nil {
		return 0, err
	}
	return roundCentsFromCurrencyUnits(roundedUnits, unitsPerUsdMicros, mode)
}

// roundCentsByCurrency rounds the canonical accounting amount to the given
// currency's cash-rounding grid (e.g. nearest whole dollar for USD, nearest
// 10,000 for LBP, or whatever a shop has configured for any currency it
// adds), then returns the equivalent accounting cents.
func roundCentsByCurrency(cents, unitsPerUsdMicros, cashRoundingUnit int64, mode string) (int64, error) {
	negative := cents < 0
	rounded, err := roundCashEquivalentCents(absInt64(cents), unitsPerUsdMicros, cashRoundingUnit, mode)
	if err != nil {
		return 0, err
	}
	if negative {
		return -rounded, nil
	}
	return rounded, nil
}
