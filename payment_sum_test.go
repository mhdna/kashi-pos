package main

import (
	"math"
	"testing"
)

// A refund split over two methods was refused as "payment total is too large".
func TestPaymentSumOverflows(t *testing.T) {
	for _, c := range []struct {
		paid, amount int64
		want         bool
	}{
		{0, -1500, false},
		{-1500, -500, false}, // the reported case: second refund line
		{-1500, 500, false},
		{1500, 500, false},
		{math.MaxInt64 - 1, 1, false},
		{math.MaxInt64, 1, true},
		{math.MinInt64 + 1, -1, false},
		{math.MinInt64, -1, true},
		{math.MinInt64, 1, false},
		{math.MaxInt64, -1, false},
	} {
		if got := paymentSumOverflows(c.paid, c.amount); got != c.want {
			t.Errorf("paymentSumOverflows(%d, %d) = %v, want %v", c.paid, c.amount, got, c.want)
		}
	}
}
