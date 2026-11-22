package main

import (
	"context"
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

// "Independence Day $50" that replaces item discounts: $50 off every $100,
// else the list's 20%, never both.
var indepReplacing = syncedInvoiceDiscount{
	UID: "indep", Name: "Independence Day $50", MinTotal: 10000, Kind: invoiceDiscountAmount,
	Amount: 5000, CashierPicks: true, Repeats: true, ReplacesItemDiscounts: true,
}

// setUpTwentyShop is the deal shop with a 20% list on the jacket and the
// socks (A-D stay on Buy 2 get 1 with no percent of their own).
func setUpTwentyShop(t *testing.T) dealShop {
	t.Helper()
	shop := setUpDealShop(t)
	if err := applySyncedDiscountList(context.Background(), shop.store.Queries, syncedDiscountList{
		UID: "disc-20", Name: "Twenty", IsActive: true,
		Items: []syncedDiscountListItem{
			{ProductVariantUID: "var-JACKET", Discount: 20},
			{ProductVariantUID: "var-SOCKS", Discount: 20},
		},
	}); err != nil {
		t.Fatal(err)
	}
	return shop
}

// checkStored: what checkout stored is the quote, the lines add up to the
// invoice and carry its discount to the cent.
func checkStored(t *testing.T, shop dealShop, sale SalesInvoiceResult, quote InvoiceQuote) []db.InvoiceProduct {
	t.Helper()
	ctx := context.Background()
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
	if quote.GrandTotalCents != sale.GrandTotal || inv.GrandTotal != sale.GrandTotal ||
		inv.InvoiceDiscount != quote.InvoiceDiscountCents || shares != inv.InvoiceDiscount ||
		totals != inv.DiscountedTotal || inv.DiscountedTotal != quote.DiscountedSubtotalCents {
		t.Fatalf("quote %+v, sale %d, stored %+v, shares %d, lines %d", quote, sale.GrandTotal, inv, shares, totals)
	}
	return lines
}

func TestInvoiceDiscountReplacesItemDiscounts(t *testing.T) {
	shop := setUpTwentyShop(t)
	addDiscount(t, shop, indepReplacing)

	// The reported case: a $100 jacket at 20% is $80; with the $50 picked it
	// goes without the 20% and reaches $100, so it's $50.
	if s, q := shop.sellPicked(t, nil, "JACKET"); s.GrandTotal != 8000 || q.InvoiceDiscountCents != 0 {
		t.Fatalf("unpicked = %d %+v", s.GrandTotal, q)
	}
	sale, quote := shop.sellPicked(t, []string{"indep"}, "JACKET")
	if sale.GrandTotal != 5000 || quote.InvoiceDiscountCents != 5000 || quote.InvoiceDiscountName != "Independence Day $50" {
		t.Fatalf("picked = %d %+v", sale.GrandTotal, quote)
	}
	if len(quote.Lines) != 1 || quote.Lines[0].DiscountCents != 0 || quote.Lines[0].ReplacedDiscount != 20 || quote.Lines[0].LineTotalCents != 10000 {
		t.Fatalf("line = %+v", quote.Lines)
	}
	lines := checkStored(t, shop, sale, quote)
	if lines[0].Discount != 0 || lines[0].InvoiceDiscount != 5000 {
		t.Fatalf("stored line = %+v", lines[0])
	}

	// Two jackets: $200 at full price, $100 off; a returned one gets back
	// what it was paid.
	sale, quote = shop.sellPicked(t, []string{"indep"}, "JACKET", "JACKET")
	if sale.GrandTotal != 10000 {
		t.Fatalf("two jackets = %d", sale.GrandTotal)
	}
	checkStored(t, shop, sale, quote)
	if got := shop.refund(t, sale.InvoiceID, "JACKET"); got != -5000 {
		t.Fatalf("refund = %d", got)
	}

	// Four socks are $80 at full price: no $50, so the 20% stays ($64).
	sale, quote = shop.sellPicked(t, []string{"indep"}, "SOCKS", "SOCKS", "SOCKS", "SOCKS")
	if sale.GrandTotal != 6400 || quote.InvoiceDiscountCents != 0 || quote.Lines[0].ReplacedDiscount != 0 {
		t.Fatalf("socks = %d %+v", sale.GrandTotal, quote)
	}

	// Deals stay: A+B+C is $230 with C free, $180; plus a jacket at full
	// price $280, $100 off: $180 (with the 20%: 180+80 = $260, then $50 =
	// $210 would be the old stacking).
	sale, quote = shop.sellPicked(t, []string{"indep"}, "A", "B", "C", "JACKET")
	if sale.GrandTotal != 18000 {
		t.Fatalf("with a deal = %d %+v", sale.GrandTotal, quote)
	}
	checkStored(t, shop, sale, quote)

	// The same rule without the switch comes off on top of the 20%: the
	// jacket's $80 doesn't reach $100, two reach $160 and get $50 off.
	plain := indepReplacing
	plain.ReplacesItemDiscounts = false
	addDiscount(t, shop, plain)
	if s, _ := shop.sellPicked(t, []string{"indep"}, "JACKET"); s.GrandTotal != 8000 {
		t.Fatalf("plain one = %d", s.GrandTotal)
	}
	if s, _ := shop.sellPicked(t, []string{"indep"}, "JACKET", "JACKET"); s.GrandTotal != 11000 {
		t.Fatalf("plain two = %d", s.GrandTotal)
	}

	// Over: the 20% again.
	over := indepReplacing
	past := time.Now().Add(-time.Hour)
	over.ValidTo = &past
	addDiscount(t, shop, over)
	if s, _ := shop.sellPicked(t, []string{"indep"}, "JACKET"); s.GrandTotal != 8000 {
		t.Fatalf("over = %d", s.GrandTotal)
	}

	// A branch that lets deals add up: the same.
	addDiscount(t, shop, indepReplacing)
	if err := shop.app.SetDealsStackEnabled(true); err != nil {
		t.Fatal(err)
	}
	if s, _ := shop.sellPicked(t, []string{"indep"}, "JACKET"); s.GrandTotal != 5000 {
		t.Fatalf("stacked = %d", s.GrandTotal)
	}
}

// One that applies by itself (not picked) replaces the 20% too, and loses
// to it when it saves less: "$10 off $100" on a $100 jacket is $90 against
// the 20%'s $80.
func TestReplacingDiscountOnlyWhenItSavesMore(t *testing.T) {
	shop := setUpTwentyShop(t)
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "ten", Name: "$10 off $100", MinTotal: 10000, Kind: invoiceDiscountAmount, Amount: 1000, ReplacesItemDiscounts: true})
	sale, quote := shop.sellPicked(t, nil, "JACKET")
	if sale.GrandTotal != 8000 || quote.InvoiceDiscountCents != 0 || quote.Lines[0].DiscountCents != 20 || quote.Lines[0].ReplacedDiscount != 0 {
		t.Fatalf("$10 = %d %+v", sale.GrandTotal, quote)
	}
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "ten", Name: "$30 off $100", MinTotal: 10000, Kind: invoiceDiscountAmount, Amount: 3000, ReplacesItemDiscounts: true})
	sale, quote = shop.sellPicked(t, nil, "JACKET")
	if sale.GrandTotal != 7000 || quote.InvoiceDiscountName != "$30 off $100" {
		t.Fatalf("$30 = %d %+v", sale.GrandTotal, quote)
	}
	checkStored(t, shop, sale, quote)
}

func TestReplacingRuleOffered(t *testing.T) {
	now := time.Now()
	picks := db.InvoiceDiscount{Uid: "p", CashierPicks: 1, ReplacesItemDiscounts: 1}
	self := db.InvoiceDiscount{Uid: "s", ReplacesItemDiscounts: 1}
	plain := db.InvoiceDiscount{Uid: "x"}
	over := db.InvoiceDiscount{Uid: "o", ReplacesItemDiscounts: 1, ValidTo: now.Add(-time.Minute).UTC().Format(time.RFC3339)}
	for _, tt := range []struct {
		rules  []db.InvoiceDiscount
		picked []string
		want   bool
	}{
		{nil, nil, false},
		{[]db.InvoiceDiscount{plain}, []string{"x"}, false},
		{[]db.InvoiceDiscount{picks}, nil, false},
		{[]db.InvoiceDiscount{picks}, []string{"q"}, false},
		{[]db.InvoiceDiscount{picks}, []string{"p"}, true},
		{[]db.InvoiceDiscount{self}, nil, true},
		{[]db.InvoiceDiscount{over}, nil, false},
		{[]db.InvoiceDiscount{plain, over, picks}, []string{"p"}, true},
	} {
		if got := replacingRuleOffered(tt.rules, now, tt.picked); got != tt.want {
			t.Errorf("%+v picked %v = %v", tt.rules, tt.picked, got)
		}
	}
}

// Randomized: hundreds of carts, picked or not, best-wins or added up. The
// charge is the cheaper of the 20% and the $50 per $100 of the sale at no
// line discount, the quote is what checkout charges and stores, and the
// lines say which way it went.
func TestReplacingDiscountRandomized(t *testing.T) {
	shop := setUpTwentyShop(t)
	addDiscount(t, shop, indepReplacing)
	price := map[string]int64{"JACKET": 10000, "SOCKS": 2000}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	codes := []string{"A", "B", "C", "D", "JACKET", "SOCKS"}
	for run := 0; run < 300; run++ {
		if err := shop.app.SetDealsStackEnabled(rng.Intn(2) == 0); err != nil {
			t.Fatal(err)
		}
		var cart []string
		for n := 1 + rng.Intn(8); n > 0; n-- {
			cart = append(cart, codes[rng.Intn(len(codes))])
		}
		_, withTwenty := shop.sellPicked(t, nil, cart...)
		// At full price: the deal lines as they are, the 20% lines without it.
		full := withTwenty.DiscountedSubtotalCents
		for _, l := range withTwenty.Lines {
			if p, ok := price[l.Code]; ok {
				full += p*l.Qty - l.LineTotalCents
			}
		}
		want := withTwenty.DiscountedSubtotalCents
		if full >= 10000 {
			want = min(want, full-min(5000*(full/10000), full))
		}
		picked := rng.Intn(4) != 0
		var picks []string
		if picked {
			picks = []string{"indep"}
		} else {
			want = withTwenty.DiscountedSubtotalCents
		}
		sale, quote := shop.sellPicked(t, picks, cart...)
		if quote.DiscountedSubtotalCents != want {
			t.Fatalf("cart %v picked %v: %d, want %d (20%%: %d, full %d)", cart, picked, quote.DiscountedSubtotalCents, want, withTwenty.DiscountedSubtotalCents, full)
		}
		checkStored(t, shop, sale, quote)
		replaced := quote.InvoiceDiscountCents > 0
		for _, l := range quote.Lines {
			_, twenty := price[l.Code]
			if (l.ReplacedDiscount == 20) != (replaced && twenty) || (replaced && l.DiscountCents != 0) {
				t.Fatalf("cart %v: line %+v, replaced %v", cart, l, replaced)
			}
		}
	}
}

// The quote the cart shows for the reported case, for the UI's test
// (frontend/src/lib/replacedDiscount.test.ts).
func TestReplacedDiscountQuoteGolden(t *testing.T) {
	shop := setUpTwentyShop(t)
	addDiscount(t, shop, indepReplacing)
	_, picked := shop.sellPicked(t, []string{"indep"}, "JACKET", "SOCKS")
	_, small := shop.sellPicked(t, []string{"indep"}, "SOCKS", "SOCKS")
	raw, err := json.MarshalIndent(map[string]InvoiceQuote{"replaced": picked, "kept": small}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("frontend", "src", "lib", "testdata", "replaced_discount_quotes.json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
