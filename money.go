package main

import (
	"fmt"
	"math"
)

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
