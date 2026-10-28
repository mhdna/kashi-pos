package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

func TestBestInvoiceDiscount(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	over150 := db.InvoiceDiscount{Name: "15% at $150", MinTotal: 15000, Kind: invoiceDiscountPercent, Percent: 15}
	over300 := db.InvoiceDiscount{Name: "20% at $300", MinTotal: 30000, Kind: invoiceDiscountPercent, Percent: 20}
	tenOff := db.InvoiceDiscount{Name: "$10 at $100", MinTotal: 10000, Kind: invoiceDiscountAmount, Amount: 1000}
	ended := over300
	ended.ValidTo = now.Add(-time.Hour).Format(time.RFC3339)
	notYet := over300
	notYet.ValidFrom = now.Add(time.Hour).Format(time.RFC3339)
	tests := []struct {
		name  string
		rules []db.InvoiceDiscount
		total int64
		want  invoiceDiscount
	}{
		{"below every minimum", []db.InvoiceDiscount{over150}, 14999, invoiceDiscount{}},
		{"exactly the minimum", []db.InvoiceDiscount{over150}, 15000, invoiceDiscount{"15% at $150", 2250}},
		{"the best one wins", []db.InvoiceDiscount{tenOff, over150, over300}, 40000, invoiceDiscount{"20% at $300", 8000}},
		{"an amount can beat a percent", []db.InvoiceDiscount{tenOff, over150}, 16000, invoiceDiscount{"15% at $150", 2400}},
		{"only the amount reaches", []db.InvoiceDiscount{tenOff, over150}, 12000, invoiceDiscount{"$10 at $100", 1000}},
		{"an ended rule is out", []db.InvoiceDiscount{over150, ended}, 40000, invoiceDiscount{"15% at $150", 6000}},
		{"a rule not started yet is out", []db.InvoiceDiscount{notYet}, 40000, invoiceDiscount{}},
		{"never more than the total", []db.InvoiceDiscount{{Name: "big", Kind: invoiceDiscountAmount, Amount: 99999}}, 500, invoiceDiscount{"big", 500}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bestInvoiceDiscount(tt.rules, tt.total, now, "half_up"); got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSpreadInvoiceDiscount(t *testing.T) {
	lines := []pricedInvoiceProduct{{lineTotal: 10000}, {lineTotal: 0}, {lineTotal: 3333}, {lineTotal: 6667}}
	out := spreadInvoiceDiscount(lines, 1001)
	var shares, totals int64
	for _, l := range out {
		shares += l.invoiceDiscount
		totals += l.lineTotal
		if l.lineTotal < 0 {
			t.Fatalf("line went below zero: %+v", l)
		}
	}
	if shares != 1001 || totals != 20000-1001 {
		t.Fatalf("shares = %d, totals = %d", shares, totals)
	}
	if out[1].invoiceDiscount != 0 {
		t.Fatalf("a free line got a share: %+v", out[1])
	}
	if lines[0].invoiceDiscount != 0 {
		t.Fatal("the input lines were changed")
	}
}

// A $150-or-more sale gets 15% off on top of its deal, the lines carry their
// shares, kashi is told, and a returned line is refunded what it was paid.
func TestCheckoutWithInvoiceDiscount(t *testing.T) {
	ctx := context.Background()
	shop := setUpDealShop(t)
	if err := applySyncedInvoiceDiscount(ctx, shop.store.Queries, syncedInvoiceDiscount{
		UID: "inv-15", Name: "15% over $150", MinTotal: 15000, Kind: invoiceDiscountPercent, Percent: 15, Applies: true,
	}); err != nil {
		t.Fatal(err)
	}

	// Under the minimum: nothing off.
	if small := shop.sell(t, "JACKET"); small.GrandTotal != 10000 || small.InvoiceDiscount.Cents != 0 {
		t.Fatalf("small sale = %d, %+v", small.GrandTotal, small.InvoiceDiscount)
	}

	// A, B, C on B2G1 (C free) + jacket: $280 after the deal, 15% = $42.
	sale := shop.sell(t, "A", "B", "C", "JACKET")
	if sale.GrandTotal != 28000-4200 || sale.InvoiceDiscount != (invoiceDiscount{"15% over $150", 4200}) {
		t.Fatalf("sale = %d, %+v", sale.GrandTotal, sale.InvoiceDiscount)
	}
	inv, err := shop.store.GetInvoiceByID(ctx, sale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	if inv.InvoiceDiscount != 4200 || inv.InvoiceDiscountName != "15% over $150" || inv.DiscountedTotal != 23800 {
		t.Fatalf("invoice = %+v", inv)
	}
	lines, err := shop.store.ListInvoiceProducts(ctx, sale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	var shares, totals int64
	for _, l := range lines {
		shares += l.InvoiceDiscount
		totals += l.LineTotal
		if l.UnitPrice == 10000 && l.DealUid == "" && (l.InvoiceDiscount != 1500 || l.LineTotal != 8500) {
			t.Fatalf("jacket line = %+v", l)
		}
	}
	if shares != 4200 || totals != 23800 {
		t.Fatalf("lines: shares %d, totals %d", shares, totals)
	}

	var payload branchInvoicePayload
	entries, err := shop.store.ListDueSyncOutboxEntries(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		var p branchInvoicePayload
		if e.EntityType == "sales_invoice" && json.Unmarshal([]byte(e.PayloadJson), &p) == nil && p.BranchInvoiceCode == sale.InvoiceCode {
			payload = p
		}
	}
	if payload.InvoiceDiscount != 4200 || payload.InvoiceDiscountName != "15% over $150" {
		t.Fatalf("payload = %+v", payload)
	}

	// The jacket comes back: it was paid $85, so that's the refund.
	if got := shop.refund(t, sale.InvoiceID, "JACKET"); got != -8500 {
		t.Fatalf("refund = %d, want -8500", got)
	}

	// kashi takes the rule away: the till drops it.
	if err := applySyncedInvoiceDiscount(ctx, shop.store.Queries, syncedInvoiceDiscount{UID: "inv-15", Applies: false}); err != nil {
		t.Fatal(err)
	}
	if again := shop.sell(t, "A", "B", "C", "JACKET"); again.GrandTotal != 28000 {
		t.Fatalf("after removal = %d", again.GrandTotal)
	}
}
