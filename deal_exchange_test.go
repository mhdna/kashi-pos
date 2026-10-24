package main

import (
	"context"
	"encoding/json"
	"testing"

	db "kashi-pos/db/sqlc"
)

// exchange swaps returned codes for replacement codes on a sale, paying what
// the quote asks, and answers that amount.
func (s dealShop) exchange(t *testing.T, saleID int64, returned, replacements []string) int64 {
	t.Helper()
	total, err := s.tryExchange(saleID, returned, replacements)
	if err != nil {
		t.Fatal(err)
	}
	return total
}

func (s dealShop) tryExchange(saleID int64, returned, replacements []string) (int64, error) {
	var lines []exchangeLine
	for _, code := range returned {
		lines = append(lines, exchangeLine{Code: code, Barcode: "bc-" + code, Qty: -1})
	}
	for _, code := range replacements {
		lines = append(lines, exchangeLine{Code: code, Barcode: "bc-" + code, Qty: 1})
	}
	itemsJSON, _ := json.Marshal(lines)
	quote, err := s.app.QuoteExchangeTotal(string(itemsJSON), saleID)
	if err != nil {
		return 0, err
	}
	payments := "[]"
	if quote.GrandTotalCents > 0 {
		raw, _ := json.Marshal([]checkoutPayment{{AccountID: s.cashbox.ID, AmountCents: quote.GrandTotalCents}})
		payments = string(raw)
	}
	if _, err := s.app.CheckoutExchangeInvoice(string(itemsJSON), payments, saleID, 0, s.salesperson.ID, ""); err != nil {
		return 0, err
	}
	return quote.GrandTotalCents, nil
}

// refund is an admin return of codes; it answers the (negative) total.
func (s dealShop) refund(t *testing.T, saleID int64, codes ...string) int64 {
	t.Helper()
	var lines []exchangeLine
	for _, code := range codes {
		lines = append(lines, exchangeLine{Code: code, Barcode: "bc-" + code, Qty: -1})
	}
	itemsJSON, _ := json.Marshal(lines)
	result, err := s.app.checkoutAdminReturnInvoice(string(itemsJSON), saleID, s.salesperson.ID, s.cashbox.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	return int64(result["difference"].(float64)*100 - 0.5)
}

func (s dealShop) lastAdjustments(t *testing.T) []db.ExchangeDealAdjustment {
	t.Helper()
	var id int64
	if err := s.app.conn.QueryRow(`SELECT max(invoice_id) FROM return_invoices`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	adjustments, err := s.store.ListExchangeDealAdjustments(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return adjustments
}

// Sale: A $100, B $80, C $50 on Buy 2 get 1, C free, $180 paid.
func TestDealExchanges(t *testing.T) {
	cases := []struct {
		name                  string
		returned, replacement []string
		pays                  int64
		forfeited, clawback   int64
	}{
		// The free unit swaps for anything up to its $50; what's left of
		// its credit is gone, never paid out.
		{"free unit for cheaper socks", []string{"C"}, []string{"SOCKS"}, 0, 3000, 0},
		// ... and above $50 the customer pays the difference.
		{"free unit for dearer D", []string{"C"}, []string{"D"}, 4000, 0, 0},
		// A paid unit swapped out of the deal: the free C stops being free.
		{"paid unit for a jacket off the deal", []string{"A"}, []string{"JACKET"}, 5000, 0, 5000},
		// A paid unit swapped for another unit on the deal keeps it going.
		{"paid unit for D on the deal", []string{"A"}, []string{"D", "SOCKS"}, 1000, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shop := setUpDealShop(t)
			sale := shop.sell(t, "A", "B", "C")
			if got := shop.exchange(t, sale.InvoiceID, tc.returned, tc.replacement); got != tc.pays {
				t.Fatalf("pays %d, want %d", got, tc.pays)
			}
			var forfeited, clawback int64
			for _, adj := range shop.lastAdjustments(t) {
				forfeited += adj.Forfeited
				clawback += adj.Clawback
			}
			if forfeited != tc.forfeited || clawback != tc.clawback {
				t.Fatalf("forfeited %d clawback %d, want %d %d", forfeited, clawback, tc.forfeited, tc.clawback)
			}
		})
	}
}

// A paid unit still needs replacements worth what was paid for it.
func TestDealExchangePaidUnitNeedsItsValue(t *testing.T) {
	shop := setUpDealShop(t)
	sale := shop.sell(t, "A", "B", "C")
	if _, err := shop.tryExchange(sale.InvoiceID, []string{"A"}, []string{"D"}); err == nil {
		t.Fatal("swapping the $100 A for the $90 D should need more")
	}
}

// The hole two exchanges could open: swap the free C for D, then a paid B
// for a jacket. D came for free, so it can't keep the deal alive: the
// free unit's saving is charged back on the second exchange.
func TestDealExchangeChainChargesBack(t *testing.T) {
	shop := setUpDealShop(t)
	sale := shop.sell(t, "A", "B", "C")
	if got := shop.exchange(t, sale.InvoiceID, []string{"C"}, []string{"D"}); got != 4000 {
		t.Fatalf("first exchange pays %d, want 4000", got)
	}
	// B's $80 credit, jacket $100: $20, plus C's $50 saving charged back.
	if got := shop.exchange(t, sale.InvoiceID, []string{"B"}, []string{"JACKET"}); got != 7000 {
		t.Fatalf("second exchange pays %d, want 7000", got)
	}
}

func TestDealRefunds(t *testing.T) {
	t.Run("the free unit refunds nothing", func(t *testing.T) {
		shop := setUpDealShop(t)
		sale := shop.sell(t, "A", "B", "C")
		if got := shop.refund(t, sale.InvoiceID, "C"); got != 0 {
			t.Fatalf("refund %d, want 0", got)
		}
	})
	t.Run("a paid unit less the free one it earned", func(t *testing.T) {
		shop := setUpDealShop(t)
		sale := shop.sell(t, "A", "B", "C")
		if got := shop.refund(t, sale.InvoiceID, "A"); got != -5000 {
			t.Fatalf("refund %d, want -5000", got)
		}
	})
	t.Run("a paid unit with the free one", func(t *testing.T) {
		shop := setUpDealShop(t)
		sale := shop.sell(t, "A", "B", "C")
		if got := shop.refund(t, sale.InvoiceID, "A", "C"); got != -10000 {
			t.Fatalf("refund %d, want -10000", got)
		}
	})
	t.Run("everything, then nothing more", func(t *testing.T) {
		shop := setUpDealShop(t)
		sale := shop.sell(t, "A", "B", "C")
		if got := shop.refund(t, sale.InvoiceID, "A", "B", "C"); got != -18000 {
			t.Fatalf("refund %d, want -18000", got)
		}
	})
	t.Run("the free unit first, then a paid one", func(t *testing.T) {
		shop := setUpDealShop(t)
		sale := shop.sell(t, "A", "B", "C")
		shop.refund(t, sale.InvoiceID, "C")
		// C came back for nothing; A is refunded in full and B is kept,
		// $180 paid less $100 back for $80 of goods.
		if got := shop.refund(t, sale.InvoiceID, "A"); got != -10000 {
			t.Fatalf("refund %d, want -10000", got)
		}
	})
}

// Three alike units, one free: a returned one is taken as the free one, so
// it swaps for anything up to its price.
func TestDealExchangeAlikeUnitsAreTheFreeOneFirst(t *testing.T) {
	shop := setUpDealShop(t)
	sale := shop.sell(t, "D", "D", "D")
	if sale.GrandTotal != 18000 {
		t.Fatalf("total %d", sale.GrandTotal)
	}
	if got := shop.exchange(t, sale.InvoiceID, []string{"D"}, []string{"JACKET"}); got != 1000 {
		t.Fatalf("pays %d, want 1000 (the jacket less the free D's $90)", got)
	}
}

// The exchange receipt says what happened to the deal.
func TestDealExchangeNotes(t *testing.T) {
	shop := setUpDealShop(t)
	sale := shop.sell(t, "A", "B", "C")
	shop.exchange(t, sale.InvoiceID, []string{"A"}, []string{"JACKET"})
	var notes string
	if err := shop.app.conn.QueryRow(`SELECT notes FROM invoices ORDER BY id DESC LIMIT 1`).Scan(&notes); err != nil {
		t.Fatal(err)
	}
	if notes != "Buy 2 get 1 no longer applies: $50.00 charged back" {
		t.Fatalf("notes = %q", notes)
	}
}

// A unit sold at the fallback discount was paid for ($70 of the $100 shirt)
// and is no reward: it swaps for what it cost, with no swap credit up to its
// shelf price as a free unit would have.
func TestDealExchangeFallbackUnitHasNoSwapCredit(t *testing.T) {
	shop := setUpDealShopWithFallback(t, 30)
	sale := shop.sell(t, "A")
	if got := shop.exchange(t, sale.InvoiceID, []string{"A"}, []string{"JACKET"}); got != 3000 {
		t.Fatalf("pays %d, want 3000 ($100 jacket less the $70 paid)", got)
	}
	for _, adj := range shop.lastAdjustments(t) {
		if adj.Forfeited != 0 {
			t.Fatalf("swap credit forfeited = %d, want none", adj.Forfeited)
		}
	}
}
