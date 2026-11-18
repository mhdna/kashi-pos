package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

// sellPicked sells codes with the invoice discounts picked, checking the quote,
// the payment check and checkout agree on every cent.
func (s dealShop) sellPicked(t *testing.T, picked []string, codes ...string) (SalesInvoiceResult, InvoiceQuote) {
	t.Helper()
	var items []salesInvoiceItem
	at := map[string]int{}
	for _, code := range codes {
		if i, ok := at[code]; ok {
			items[i].Qty++
			continue
		}
		at[code] = len(items)
		items = append(items, salesInvoiceItem{Barcode: "bc-" + code, Qty: 1})
	}
	itemsJSON, _ := json.Marshal(items)
	picksJSON := ""
	if picked != nil {
		raw, _ := json.Marshal(picked)
		picksJSON = string(raw)
	}
	quote, err := s.app.QuoteInvoiceTotalPicked(string(itemsJSON), s.client.ID, picksJSON)
	if err != nil {
		t.Fatal(err)
	}
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: s.cashbox.ID, AmountCents: quote.GrandTotalCents}})
	status, err := s.app.QuotePaymentStatusPicked(string(itemsJSON), string(payments), s.client.ID, picksJSON)
	if err != nil {
		t.Fatal(err)
	}
	if status.GrandTotalCents != quote.GrandTotalCents || !status.CheckoutReady {
		t.Fatalf("payment check %+v disagrees with the quote %d", status, quote.GrandTotalCents)
	}
	result, err := s.app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(itemsJSON), PaymentsJSON: string(payments), ClientID: s.client.ID, PickedDiscounts: picked})
	if err != nil {
		t.Fatal(err)
	}
	if result.GrandTotal != quote.GrandTotalCents || result.InvoiceDiscount.Cents != quote.InvoiceDiscountCents {
		t.Fatalf("checkout charged %d (%+v), the quote said %d (%d off)", result.GrandTotal, result.InvoiceDiscount, quote.GrandTotalCents, quote.InvoiceDiscountCents)
	}
	return result, quote
}

// quotePicked is the quote alone.
func (s dealShop) quotePicked(t *testing.T, picked []string, codes ...string) InvoiceQuote {
	t.Helper()
	counts := map[string]float64{}
	var order []string
	for _, c := range codes {
		if counts[c] == 0 {
			order = append(order, c)
		}
		counts[c]++
	}
	var items []salesInvoiceItem
	for _, c := range order {
		items = append(items, salesInvoiceItem{Barcode: "bc-" + c, Qty: counts[c]})
	}
	itemsJSON, _ := json.Marshal(items)
	raw, _ := json.Marshal(picked)
	quote, err := s.app.QuoteInvoiceTotalPicked(string(itemsJSON), s.client.ID, string(raw))
	if err != nil {
		t.Fatal(err)
	}
	return quote
}

func addDiscount(t *testing.T, shop dealShop, d syncedInvoiceDiscount) {
	t.Helper()
	d.Applies = true
	if err := applySyncedInvoiceDiscount(context.Background(), shop.store.Queries, d); err != nil {
		t.Fatal(err)
	}
}

// The reported case: a student discount the cashier picks, 15% off the whole
// invoice; and "15% off $100 invoices" applying by itself, for reference.
func TestPickedInvoiceDiscount(t *testing.T) {
	shop := setUpDealShop(t)
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "students", Name: "Students 15%", Kind: invoiceDiscountPercent, Percent: 15, CashierPicks: true})

	// Not picked: nothing off, even though no minimum stands in the way.
	if sale, _ := shop.sellPicked(t, nil, "JACKET"); sale.GrandTotal != 10000 || sale.InvoiceDiscount.Cents != 0 {
		t.Fatalf("unpicked = %d %+v", sale.GrandTotal, sale.InvoiceDiscount)
	}
	// Picked: 15% off the jacket's $100.
	sale, quote := shop.sellPicked(t, []string{"students"}, "JACKET")
	if sale.GrandTotal != 8500 || sale.InvoiceDiscount.Name != "Students 15%" || quote.InvoiceDiscountName != "Students 15%" {
		t.Fatalf("picked = %d %+v", sale.GrandTotal, sale.InvoiceDiscount)
	}
	inv, err := shop.store.GetInvoiceByID(context.Background(), sale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	if inv.InvoiceDiscount != 1500 || inv.InvoiceDiscountName != "Students 15%" {
		t.Fatalf("stored %+v", inv)
	}
	// A uid the till doesn't have, or picked twice, changes nothing more.
	if sale, _ := shop.sellPicked(t, []string{"nope", "students", "students"}, "JACKET"); sale.GrandTotal != 8500 {
		t.Fatalf("with junk = %d", sale.GrandTotal)
	}
}

// When a picked deal and an automatic one both apply, the one that saves more
// wins, as between automatic ones; they never add up.
func TestPickedAndAutomaticDiscountsCompete(t *testing.T) {
	shop := setUpDealShop(t)
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "auto-10", Name: "10% over $100", MinTotal: 10000, Kind: invoiceDiscountPercent, Percent: 10})
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "students", Name: "Students 15%", Kind: invoiceDiscountPercent, Percent: 15, CashierPicks: true})
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "staff-5", Name: "Staff $5", Kind: invoiceDiscountAmount, Amount: 500, CashierPicks: true})

	if sale, _ := shop.sellPicked(t, nil, "JACKET"); sale.InvoiceDiscount.Name != "10% over $100" || sale.GrandTotal != 9000 {
		t.Fatalf("automatic alone = %+v", sale.InvoiceDiscount)
	}
	if sale, _ := shop.sellPicked(t, []string{"students"}, "JACKET"); sale.InvoiceDiscount.Name != "Students 15%" || sale.GrandTotal != 8500 {
		t.Fatalf("picked bigger = %+v", sale.InvoiceDiscount)
	}
	if sale, _ := shop.sellPicked(t, []string{"staff-5"}, "JACKET"); sale.InvoiceDiscount.Name != "10% over $100" || sale.GrandTotal != 9000 {
		t.Fatalf("picked smaller loses = %+v", sale.InvoiceDiscount)
	}
	if sale, _ := shop.sellPicked(t, []string{"staff-5", "students"}, "JACKET"); sale.GrandTotal != 8500 {
		t.Fatalf("two picked: the bigger = %d", sale.GrandTotal)
	}
}

// A picked deal keeps its own minimum and dates.
func TestPickedDiscountKeepsItsTerms(t *testing.T) {
	shop := setUpDealShop(t)
	past := time.Now().Add(-48 * time.Hour)
	yesterday := time.Now().Add(-24 * time.Hour)
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "big", Name: "VIP over $500", MinTotal: 50000, Kind: invoiceDiscountPercent, Percent: 20, CashierPicks: true})
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "over", Name: "Ended", Kind: invoiceDiscountPercent, Percent: 50, CashierPicks: true, ValidFrom: &past, ValidTo: &yesterday})
	if sale, _ := shop.sellPicked(t, []string{"big", "over"}, "JACKET"); sale.InvoiceDiscount.Cents != 0 {
		t.Fatalf("a deal under its minimum or ended applied: %+v", sale.InvoiceDiscount)
	}
}

func TestParsePickedDiscounts(t *testing.T) {
	for in, want := range map[string]string{"": "[]", "  ": "[]", `[]`: "[]", `["a"," a ","b",""]`: `["a","b"]`} {
		got, err := parsePickedDiscounts(in)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(got)
		if string(raw) == "null" {
			raw = []byte("[]")
		}
		if string(raw) != want {
			t.Fatalf("%q: %s, want %s", in, raw, want)
		}
	}
	if _, err := parsePickedDiscounts(`{"a":1}`); err == nil {
		t.Fatal("not a list accepted")
	}
	shop := setUpDealShop(t)
	if _, err := shop.app.QuoteInvoiceTotalPicked(`[]`, 0, "oops"); err == nil {
		t.Fatal("a bad list was priced")
	}
	if _, err := shop.app.CheckoutInvoicePicked(`[]`, `[]`, 0, "", "", "", "", "", "", "", "oops"); err == nil {
		t.Fatal("a bad list was checked out")
	}
}

// Randomized: carts, rules and picks; the quote, the payment check and
// checkout always agree, the lines carry exactly the discount, and a deal
// the cashier picks never applies unpicked.
func TestPickedDiscountsRandomized(t *testing.T) {
	shop := setUpDealShop(t)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var uids []string
	for i := 0; i < 6; i++ {
		d := syncedInvoiceDiscount{UID: fmt.Sprintf("r%d", i), Name: fmt.Sprintf("Rule %d", i), MinTotal: int64(rng.Intn(4)) * 5000, CashierPicks: i%2 == 0}
		if rng.Intn(2) == 0 {
			d.Kind, d.Percent = invoiceDiscountPercent, int64(1+rng.Intn(40))
		} else {
			d.Kind, d.Amount = invoiceDiscountAmount, int64(1+rng.Intn(60))*100
		}
		addDiscount(t, shop, d)
		uids = append(uids, d.UID)
	}
	rules, err := shop.store.ListInvoiceDiscounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byUID := map[string]db.InvoiceDiscount{}
	for _, r := range rules {
		byUID[r.Uid] = r
	}
	codes := []string{"A", "B", "C", "JACKET"}
	for run := 0; run < 150; run++ {
		var cart []string
		for n := 1 + rng.Intn(5); n > 0; n-- {
			cart = append(cart, codes[rng.Intn(len(codes))])
		}
		var picked []string
		for _, uid := range uids {
			if rng.Intn(2) == 0 {
				picked = append(picked, uid)
			}
		}
		if shop.quotePicked(t, picked, cart...).GrandTotalCents <= 0 {
			continue // a sale that comes to nothing takes no payment
		}
		sale, quote := shop.sellPicked(t, picked, cart...)
		lines, err := shop.store.ListInvoiceProducts(context.Background(), sale.InvoiceID)
		if err != nil {
			t.Fatal(err)
		}
		var shares int64
		for _, l := range lines {
			shares += l.InvoiceDiscount
		}
		if shares != sale.InvoiceDiscount.Cents {
			t.Fatalf("lines carry %d of the %d discount", shares, sale.InvoiceDiscount.Cents)
		}
		if name := quote.InvoiceDiscountName; name != "" {
			for uid, r := range byUID {
				if r.Name == name && r.CashierPicks != 0 {
					found := false
					for _, p := range picked {
						found = found || p == uid
					}
					if !found {
						t.Fatalf("%q applied without being picked (picked %v)", name, picked)
					}
				}
			}
		}
	}
}

func TestListRewardDeals(t *testing.T) {
	shop := setUpDealShop(t)
	past, yesterday := time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour)
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "auto", Name: "15% off $100", MinTotal: 10000, Kind: invoiceDiscountPercent, Percent: 15})
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "students", Name: "Students 15%", Kind: invoiceDiscountPercent, Percent: 15, CashierPicks: true})
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "old", Name: "Ended", Kind: invoiceDiscountPercent, Percent: 5, ValidFrom: &past, ValidTo: &yesterday})
	deals, err := shop.app.ListRewardDeals()
	if err != nil {
		t.Fatal(err)
	}
	if len(deals) != 2 || deals[0].UID != "students" || !deals[0].CashierPicks || deals[1].UID != "auto" || deals[1].MinTotal != 10000 {
		t.Fatalf("deals = %+v", deals)
	}
}
