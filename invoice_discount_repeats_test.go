package main

import (
	"context"
	"math/rand"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

// "Independence Day $50": $50 off for every $100 of the sale, picked by the
// cashier. A repeating amount rule comes off once per minimum reached.
var independenceDay = syncedInvoiceDiscount{
	UID: "indep", Name: "Independence Day $50", MinTotal: 10000, Kind: invoiceDiscountAmount,
	Amount: 5000, CashierPicks: true, Repeats: true,
}

func TestRepeatingInvoiceDiscountAmounts(t *testing.T) {
	rule := db.InvoiceDiscount{Uid: "indep", Name: "Independence Day $50", MinTotal: 10000, Kind: invoiceDiscountAmount, Amount: 5000, Repeats: 1}
	once := rule
	once.Repeats = 0
	now := time.Now()
	for _, tt := range []struct {
		total, want, wantOnce int64
	}{
		{9999, 0, 0},
		{10000, 5000, 5000},
		{19999, 5000, 5000},
		{20000, 10000, 5000}, // the reported case
		{35000, 15000, 5000},
		{100000, 50000, 5000},
	} {
		if got := bestInvoiceDiscount([]db.InvoiceDiscount{rule}, tt.total, now, "half_up").Cents; got != tt.want {
			t.Errorf("repeating on %d = %d, want %d", tt.total, got, tt.want)
		}
		if got := bestInvoiceDiscount([]db.InvoiceDiscount{once}, tt.total, now, "half_up").Cents; got != tt.wantOnce {
			t.Errorf("once on %d = %d, want %d", tt.total, got, tt.wantOnce)
		}
		if got := invoiceDiscountsStacked([]db.InvoiceDiscount{rule}, tt.total, now, "half_up").Cents; got != tt.want {
			t.Errorf("stacked on %d = %d, want %d", tt.total, got, tt.want)
		}
	}
	// It never takes off more than the sale: $150 off every $100 on $200.
	big := rule
	big.Amount = 15000
	if got := bestInvoiceDiscount([]db.InvoiceDiscount{big}, 20000, now, "half_up").Cents; got != 20000 {
		t.Errorf("capped = %d", got)
	}
	// Repeating is meaningless without a minimum: once.
	nomin := rule
	nomin.MinTotal = 0
	if got := bestInvoiceDiscount([]db.InvoiceDiscount{nomin}, 30000, now, "half_up").Cents; got != 5000 {
		t.Errorf("no minimum = %d", got)
	}
	// Against "20% above $150", the bigger one wins: on $300 that's $150 vs $60.
	pct := db.InvoiceDiscount{Uid: "t150", Name: "20% above $150", MinTotal: 15000, Kind: invoiceDiscountPercent, Percent: 20}
	if got := bestInvoiceDiscount([]db.InvoiceDiscount{pct, rule}, 30000, now, "half_up"); got.Name != rule.Name || got.Cents != 15000 {
		t.Errorf("best = %+v", got)
	}
	// Stacked: the percent first, then $50 per $100 of the sale before both.
	got := invoiceDiscountsStacked([]db.InvoiceDiscount{pct, rule}, 30000, now, "half_up")
	if got.Cents != 6000+15000 || got.Name != "20% above $150 + Independence Day $50" {
		t.Errorf("stacked = %+v", got)
	}
}

// The reported case end to end: two $100 jackets with Independence Day
// picked come to $100, the lines carry the $100, it is stored and synced
// that way, and a returned jacket gets back what it was paid.
func TestCheckoutWithRepeatingDiscount(t *testing.T) {
	ctx := context.Background()
	shop := setUpDealShop(t)
	addDiscount(t, shop, independenceDay)

	deals, err := shop.app.ListRewardDeals()
	if err != nil || len(deals) != 1 || !deals[0].Repeats {
		t.Fatalf("reward deals = %+v, %v", deals, err)
	}

	sale, quote := shop.sellPicked(t, []string{"indep"}, "JACKET", "JACKET")
	if quote.GrandTotalCents != 10000 || sale.GrandTotal != 10000 || sale.InvoiceDiscount.Cents != 10000 {
		t.Fatalf("quote %d, sale %d, %+v", quote.GrandTotalCents, sale.GrandTotal, sale.InvoiceDiscount)
	}
	inv, err := shop.store.GetInvoiceByID(ctx, sale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	if inv.InvoiceDiscount != 10000 || inv.GrandTotal != 10000 || inv.InvoiceDiscountName != "Independence Day $50" {
		t.Fatalf("invoice = %+v", inv)
	}
	lines, err := shop.store.ListInvoiceProducts(ctx, sale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	var shares int64
	for _, l := range lines {
		shares += l.InvoiceDiscount
	}
	if shares != 10000 {
		t.Fatalf("lines carry %d", shares)
	}
	if got := shop.refund(t, sale.InvoiceID, "JACKET"); got != -5000 {
		t.Fatalf("refund = %d, want -5000", got)
	}

	// $180 is one $100: $50 off. Not picked: nothing.
	if s, _ := shop.sellPicked(t, []string{"indep"}, "JACKET", "SOCKS", "SOCKS", "SOCKS", "SOCKS"); s.InvoiceDiscount.Cents != 5000 {
		t.Fatalf("$180 = %+v", s.InvoiceDiscount)
	}
	if s, _ := shop.sellPicked(t, nil, "JACKET", "JACKET"); s.InvoiceDiscount.Cents != 0 {
		t.Fatalf("unpicked = %+v", s.InvoiceDiscount)
	}

	// Central turns repeating off: once again.
	once := independenceDay
	once.Repeats = false
	addDiscount(t, shop, once)
	if s, _ := shop.sellPicked(t, []string{"indep"}, "JACKET", "JACKET"); s.InvoiceDiscount.Cents != 5000 {
		t.Fatalf("after repeats off = %+v", s.InvoiceDiscount)
	}
}

// Randomized, with the biggest rule winning: hundreds of carts through quote
// and checkout. The discount is $50 per whole $100, the quote is what checkout
// charges and stores, and the lines carry it to the cent.
func TestRepeatingDiscountRandomized(t *testing.T) {
	ctx := context.Background()
	shop := setUpDealShop(t)
	addDiscount(t, shop, independenceDay)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	codes := []string{"A", "B", "C", "D", "JACKET", "SOCKS"}
	for run := 0; run < 200; run++ {
		var cart []string
		for n := 1 + rng.Intn(8); n > 0; n-- {
			cart = append(cart, codes[rng.Intn(len(codes))])
		}
		sale, quote := shop.sellPicked(t, []string{"indep"}, cart...)
		before := quote.DiscountedSubtotalCents + quote.InvoiceDiscountCents
		want := min(5000*(before/10000), before)
		if sale.InvoiceDiscount.Cents != want || quote.InvoiceDiscountCents != want {
			t.Fatalf("cart %v ($%d): got %d, quote %d, want %d", cart, before, sale.InvoiceDiscount.Cents, quote.InvoiceDiscountCents, want)
		}
		if quote.GrandTotalCents != sale.GrandTotal {
			t.Fatalf("quote %d, checkout %d", quote.GrandTotalCents, sale.GrandTotal)
		}
		inv, err := shop.store.GetInvoiceByID(ctx, sale.InvoiceID)
		if err != nil {
			t.Fatal(err)
		}
		lines, err := shop.store.ListInvoiceProducts(ctx, sale.InvoiceID)
		if err != nil {
			t.Fatal(err)
		}
		var shares, totals int64
		for _, l := range lines {
			shares += l.InvoiceDiscount
			totals += l.LineTotal
		}
		if shares != want || inv.InvoiceDiscount != want || totals != inv.DiscountedTotal || inv.GrandTotal != sale.GrandTotal {
			t.Fatalf("cart %v: shares %d, stored %d, lines %d vs %d", cart, shares, inv.InvoiceDiscount, totals, inv.DiscountedTotal)
		}
	}
}
