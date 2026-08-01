package main

import (
	"fmt"
	"math"
	"math/big"
)

const lbpRoundingUnit int64 = 10000

const (
	roundingHalfUp = "half_up"
	roundingDown   = "down"
	roundingUp     = "up"
)

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

// roundLBPFromCents converts an accounting amount to LBP and applies the
// configured cash rounding rule. The conversion is integer-based so the
// printed amount cannot drift from the amount used by checkout.
func roundLBPFromCents(cents, exchangeRate int64, mode string) (int64, error) {
	if cents < 0 || exchangeRate <= 0 {
		return 0, fmt.Errorf("invalid LBP conversion")
	}
	normalizedMode, err := normalizeRoundingMode(mode)
	if err != nil {
		return 0, err
	}
	numerator := new(big.Int).Mul(big.NewInt(cents), big.NewInt(exchangeRate))
	denominator := big.NewInt(100 * lbpRoundingUnit)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	if normalizedMode == roundingUp || (normalizedMode == roundingHalfUp && new(big.Int).Mul(remainder, big.NewInt(2)).Cmp(denominator) >= 0) {
		quotient.Add(quotient, big.NewInt(1))
	}
	quotient.Mul(quotient, big.NewInt(lbpRoundingUnit))
	if !quotient.IsInt64() {
		return 0, fmt.Errorf("LBP value is too large")
	}
	return quotient.Int64(), nil
}

// roundCentsFromLBP converts a physically counted LBP amount back to the
// accounting currency so mixed-currency shift totals can be reconciled.
func roundCentsFromLBP(amount, exchangeRate int64, mode string) (int64, error) {
	if amount < 0 || exchangeRate <= 0 {
		return 0, fmt.Errorf("invalid LBP conversion")
	}
	normalizedMode, err := normalizeRoundingMode(mode)
	if err != nil {
		return 0, err
	}
	numerator := new(big.Int).Mul(big.NewInt(amount), big.NewInt(100))
	denominator := big.NewInt(exchangeRate)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	if normalizedMode == roundingUp || (normalizedMode == roundingHalfUp && new(big.Int).Mul(remainder, big.NewInt(2)).Cmp(denominator) >= 0) {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, fmt.Errorf("LBP value is too large")
	}
	return quotient.Int64(), nil
}
