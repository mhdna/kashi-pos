package main

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"os"
	"testing"

	db "kashi-pos/db/sqlc"
)

// LBP as Centro has it: 90,000 LL to the dollar, notes in 10,000s.
const (
	testLBPMicros = 90_000 * unitsPerUsdMicroScale
	testLBPGrid   = 10_000
)

func lbpCents(t *testing.T, units int64) int64 {
	t.Helper()
	cents, err := roundCentsFromCurrencyUnits(units, testLBPMicros, roundingHalfUp)
	if err != nil {
		t.Fatal(err)
	}
	return cents
}

func sumCents(values []int64) int64 {
	var total int64
	for _, v := range values {
		total += v
	}
	return total
}

// The Centro sale SA/CT/2026/03167: $41.56 paid with $40 and LL 200,000,
// LL 60,000 back. The LL kept is 140,000, which is 156¢; adding the
// entries' own cents (222 - 67) recorded 155¢.
func TestSettleCashEntryCentsNetsChangeInLBP(t *testing.T) {
	entries := []cashEntry{
		{Cents: lbpCents(t, 200_000), Units: 200_000},
		{Cents: -lbpCents(t, 60_000), Units: 60_000},
	}
	if entries[0].Cents+entries[1].Cents != 155 {
		t.Fatalf("setup: the entries' own cents add to %d, want the 155 the bug recorded", entries[0].Cents+entries[1].Cents)
	}
	got, err := settleCashEntryCents(entries, testLBPMicros, testLBPGrid, roundingHalfUp)
	if err != nil {
		t.Fatal(err)
	}
	if sumCents(got) != 156 {
		t.Fatalf("LL 200,000 less LL 60,000 recorded %v = %d¢, want 156¢ (LL 140,000)", got, sumCents(got))
	}
	if got[0] != 222 || got[1] != -66 {
		t.Fatalf("entries = %v, want [222 -66]", got)
	}
}

func TestSettleCashEntryCentsEdges(t *testing.T) {
	cases := []struct {
		name    string
		entries []cashEntry
		want    []int64
	}{
		{"one payment snaps to the grid", []cashEntry{{Cents: 161, Units: 145_000}}, []int64{167}},
		{"a payment without its LL snaps from its cents", []cashEntry{{Cents: 161}}, []int64{156}},
		{"two notes are added in LL", []cashEntry{{Cents: 56, Units: 50_000}, {Cents: 56, Units: 50_000}}, []int64{56, 55}},
		{"change without its LL keeps its cents", []cashEntry{{Cents: 222, Units: 200_000}, {Cents: -67}}, []int64{222, -67}},
		{"change whose LL doesn't give its cents keeps them", []cashEntry{{Cents: 222, Units: 200_000}, {Cents: -70, Units: 60_000}}, []int64{222, -70}},
		{"change alone", []cashEntry{{Cents: -67, Units: 60_000}}, []int64{-67}},
		{"change in 5,000s", []cashEntry{{Cents: 1111, Units: 1_000_000}, {Cents: -6, Units: 5_000}, {Cents: -6, Units: 5_000}}, []int64{1111, -5, -6}},
		{"all of it back", []cashEntry{{Cents: 222, Units: 200_000}, {Cents: -222, Units: 200_000}}, []int64{222, -222}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := settleCashEntryCents(tc.entries, testLBPMicros, testLBPGrid, roundingHalfUp)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// Hundreds of random LL tenders and change, every rounding mode: the
// entries always add up to the cents of the LL kept, each entry is within a
// cent of its own amount, and no entry changes sign.
func TestSettleCashEntryCentsRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(20261113))
	for _, mode := range []string{roundingHalfUp, roundingDown, roundingUp} {
		for n := 0; n < 400; n++ {
			count := 1 + rng.Intn(5)
			entries := make([]cashEntry, count)
			var kept int64
			for i := range entries {
				if i == 0 || rng.Intn(2) == 0 {
					units := int64(1+rng.Intn(200)) * testLBPGrid
					cents, _ := roundCentsFromCurrencyUnits(units, testLBPMicros, mode)
					entries[i] = cashEntry{Cents: cents, Units: units}
					kept += units
				} else {
					units := int64(1+rng.Intn(40)) * 5_000
					cents, _ := roundCentsFromCurrencyUnits(units, testLBPMicros, mode)
					entries[i] = cashEntry{Cents: -cents, Units: units}
					kept -= units
				}
			}
			got, err := settleCashEntryCents(entries, testLBPMicros, testLBPGrid, mode)
			if err != nil {
				t.Fatal(err)
			}
			want, _ := signedCentsFromCurrencyUnits(kept, testLBPMicros, mode)
			if sumCents(got) != want {
				t.Fatalf("%s %+v: entries %v add to %d, want %d (LL %d kept)", mode, entries, got, sumCents(got), want, kept)
			}
			for i, e := range entries {
				own, _ := roundCentsFromCurrencyUnits(e.Units, testLBPMicros, mode)
				if e.Cents < 0 {
					own = -own
				}
				if d := got[i] - own; d > 1 || d < -1 {
					t.Fatalf("%s %+v: entry %d is %d, more than a cent from its own %d", mode, entries, i, got[i], own)
				}
				if (e.Cents > 0 && got[i] < 0) || (e.Cents < 0 && got[i] > 0) {
					t.Fatalf("%s %+v: entry %d changed sign: %v", mode, entries, i, got)
				}
			}
		}
	}
}

type lbpShop struct {
	app    *App
	usd    db.CashboxAccount
	lbp    db.CashboxAccount
	client db.Client
	mode   string
}

// setUpLBPShop is a till selling a $41.60 item with totals rounded to LL
// 10,000, like Centro.
func setUpLBPShop(t *testing.T, mode string) lbpShop {
	t.Helper()
	app, usd, client, _ := setupQuoteTestApp(t, 0, mode, "LBP")
	syncCurrency(t, app, "LBP", "Lebanese Pound", "LL", testLBPMicros, testLBPGrid)
	lbp := syncPaymentMethod(t, app, "Cash LL", "LBP")
	settings, err := app.store.GetSettings(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	product, err := app.store.CreateProduct(app.ctx, db.CreateProductParams{
		Barcode: "4160", Code: "SKU-4160", Name: "Hoodie", IsActive: 1, Price: 4160,
	})
	if err != nil {
		t.Fatal(err)
	}
	priceProductsFromRows(t, app.store.Queries)
	if err := app.store.UpsertInventoryItem(app.ctx, db.UpsertInventoryItemParams{InventoryID: settings.SalesInventoryID, ProductID: product.ID, Quantity: 100000}); err != nil {
		t.Fatal(err)
	}
	return lbpShop{app: app, usd: usd, lbp: lbp, client: client, mode: mode}
}

// lbpPayment and lbpChange are what the calculator sends for an LL entry:
// its LL, and its cents in the till's rounding mode.
func (s lbpShop) cents(t *testing.T, units int64) int64 {
	t.Helper()
	cents, err := roundCentsFromCurrencyUnits(units, testLBPMicros, s.mode)
	if err != nil {
		t.Fatal(err)
	}
	return cents
}

func (s lbpShop) lbpPayment(t *testing.T, units int64) checkoutPayment {
	return checkoutPayment{AccountID: s.lbp.ID, AmountCents: s.cents(t, units), AmountUnits: units, EntryType: "payment"}
}

func (s lbpShop) lbpChange(t *testing.T, units int64) checkoutPayment {
	return checkoutPayment{AccountID: s.lbp.ID, AmountCents: -s.cents(t, units), AmountUnits: units, EntryType: "change"}
}

// checkout runs payments through the quote and checkout and checks what
// was stored: the quote's paid is what checkout records, and the sale's
// payments add up to its total.
func (s lbpShop) checkout(t *testing.T, items string, payments []checkoutPayment) (int64, map[int64]int64) {
	t.Helper()
	raw, _ := json.Marshal(payments)
	status, err := s.app.QuotePaymentStatus(items, string(raw))
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: items, PaymentsJSON: string(raw), ClientID: s.client.ID})
	if err != nil {
		t.Fatalf("checkout of %s: %v", raw, err)
	}
	invoice, err := s.app.store.GetInvoiceByID(s.app.ctx, result.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := s.app.store.ListInvoicePayments(s.app.ctx, result.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	net := map[int64]int64{}
	var paid int64
	for _, p := range stored {
		net[p.CashboxID] += p.Amount
		paid += p.Amount
	}
	entries, err := s.app.store.ListInvoicePaymentEntries(s.app.ctx, result.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	var entryTotal int64
	for _, e := range entries {
		entryTotal += e.Amount
	}
	if paid != status.PaidCents || entryTotal != paid {
		t.Fatalf("%s: quote said %d paid, checkout stored %d net and %d in entries", raw, status.PaidCents, paid, entryTotal)
	}
	if !status.IsPaidInFull || paid != invoice.GrandTotal {
		t.Fatalf("%s: payments %d don't add up to the sale's %d (quote %+v)", raw, paid, invoice.GrandTotal, status)
	}
	return invoice.GrandTotal, net
}

func TestCheckoutRecordsLBPKeptAfterChange(t *testing.T) {
	shop := setUpLBPShop(t, roundingHalfUp)
	items := `[{"code":"SKU-4160","qty":1}]`
	total, net := shop.checkout(t, items, []checkoutPayment{
		{AccountID: shop.usd.ID, AmountCents: 4000, EntryType: "payment"},
		shop.lbpPayment(t, 200_000),
		shop.lbpChange(t, 60_000),
	})
	if total != 4156 || net[shop.usd.ID] != 4000 || net[shop.lbp.ID] != 156 {
		t.Fatalf("total %d, $ %d, LL %d; want 4156, 4000 and 156", total, net[shop.usd.ID], net[shop.lbp.ID])
	}
}

// A till still on the old calculator sends change without its LL: checkout
// takes the cents as they come, and the quote still says paid.
func TestCheckoutChangeWithoutUnitsKeepsItsCents(t *testing.T) {
	shop := setUpLBPShop(t, roundingHalfUp)
	items := `[{"code":"SKU-4160","qty":1}]`
	raw, _ := json.Marshal([]checkoutPayment{
		{AccountID: shop.usd.ID, AmountCents: 4000, EntryType: "payment"},
		{AccountID: shop.lbp.ID, AmountCents: 222, EntryType: "payment"},
		{AccountID: shop.lbp.ID, AmountCents: -67, EntryType: "change"},
	})
	status, err := shop.app.QuotePaymentStatus(items, string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if status.PaidCents != 4155 || !status.CheckoutReady {
		t.Fatalf("status = %+v, want 4155 paid and ready (cash rounded)", status)
	}
	if _, err := shop.app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: items, PaymentsJSON: string(raw), ClientID: shop.client.ID}); err != nil {
		t.Fatal(err)
	}
}

// Hundreds of sales paid partly in dollars and partly in LL, with LL change
// in 5,000s and the LL sometimes in two notes, in every rounding mode: the
// quote is what checkout stores, the payments add up to the sale's total,
// and the LL recorded is the cents of the LL kept.
func TestCheckoutLBPPaymentsAddUpRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(1113))
	for _, mode := range []string{roundingHalfUp, roundingDown, roundingUp} {
		shop := setUpLBPShop(t, mode)
		for n := 0; n < 120; n++ {
			items := `[{"code":"SKU-4160","qty":` + itoa(1+rng.Intn(6)) + `}]`
			quote, err := shop.app.QuoteInvoiceTotal(items)
			if err != nil {
				t.Fatal(err)
			}
			total := quote.GrandTotalCents
			dollars := int64(rng.Intn(int(total/100) + 1))
			// The LL left to pay is on the grid, so rounding its cents to the
			// nearest note gets it back exactly whatever the till's mode.
			owedUnits, err := roundCurrencyUnitsFromCents(total-dollars*100, testLBPMicros, testLBPGrid, roundingHalfUp)
			if err != nil {
				t.Fatal(err)
			}
			var payments []checkoutPayment
			if dollars > 0 {
				payments = append(payments, checkoutPayment{AccountID: shop.usd.ID, AmountCents: dollars * 100, EntryType: "payment"})
			}
			changeUnits := int64(rng.Intn(8)) * 5_000
			tender := owedUnits + changeUnits
			if tender%testLBPGrid != 0 {
				tender += 5_000
				changeUnits += 5_000
			}
			if tender > 0 {
				if tender >= 2*testLBPGrid && rng.Intn(2) == 0 {
					first := int64(1+rng.Intn(int(tender/testLBPGrid)-1)) * testLBPGrid
					payments = append(payments, shop.lbpPayment(t, first), shop.lbpPayment(t, tender-first))
				} else {
					payments = append(payments, shop.lbpPayment(t, tender))
				}
			}
			if changeUnits > 0 {
				payments = append(payments, shop.lbpChange(t, changeUnits))
			}
			if len(payments) == 0 {
				continue
			}
			_, net := shop.checkout(t, items, payments)
			wantLBP, _ := roundCentsFromCurrencyUnits(owedUnits, testLBPMicros, mode)
			if net[shop.lbp.ID] != wantLBP {
				t.Fatalf("%s %+v: LL recorded %d¢, want %d¢ for LL %d kept", mode, payments, net[shop.lbp.ID], wantLBP, owedUnits)
			}
		}
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// The calculator adds up its LL entries the way checkout records them
// (frontend/src/lib/cashPayments.ts, checked by cashPayments.test.ts
// against this file), so what it shows as paid is what checkout stores.
// Rewrite it with KASHI_UPDATE_GOLDEN=1 go test -run TestCashPaymentNettingGolden .
const cashPaymentNettingGoldenPath = "frontend/src/lib/testdata/cash_payment_netting.json"

type goldenCashNetting struct {
	Name             string      `json:"name"`
	Mode             string      `json:"mode"`
	UnitsPerUsd      int64       `json:"unitsPerUsd"`
	CashRoundingUnit int64       `json:"cashRoundingUnit"`
	Entries          []cashEntry `json:"entries"`
	Cents            []int64     `json:"cents"`
}

func TestCashPaymentNettingGolden(t *testing.T) {
	var golden []goldenCashNetting
	add := func(name, mode string, unitsPerUsd, grid int64, entries []cashEntry) {
		t.Helper()
		cents, err := settleCashEntryCents(entries, unitsPerUsd*unitsPerUsdMicroScale, grid, mode)
		if err != nil {
			t.Fatal(err)
		}
		golden = append(golden, goldenCashNetting{name, mode, unitsPerUsd, grid, entries, cents})
	}
	add("SA/CT/2026/03167: LL 200,000 less LL 60,000", roundingHalfUp, 90_000, 10_000, []cashEntry{{222, 200_000}, {-67, 60_000}})
	add("payment off the grid", roundingHalfUp, 90_000, 10_000, []cashEntry{{161, 145_000}})
	add("payment without its LL", roundingHalfUp, 90_000, 10_000, []cashEntry{{161, 0}})
	add("change without its LL", roundingHalfUp, 90_000, 10_000, []cashEntry{{222, 200_000}, {-67, 0}})
	add("change whose LL doesn't give its cents", roundingHalfUp, 90_000, 10_000, []cashEntry{{222, 200_000}, {-70, 60_000}})
	add("round down: LL 140,000", roundingDown, 90_000, 10_000, []cashEntry{{155, 140_000}})
	add("round up: LL 140,000", roundingUp, 90_000, 10_000, []cashEntry{{156, 140_000}})
	add("89,500 a dollar in 5,000s", roundingHalfUp, 89_500, 5_000, []cashEntry{{1117, 1_000_000}, {-6, 5_000}, {-6, 5_000}})

	rng := rand.New(rand.NewSource(7))
	for _, mode := range []string{roundingHalfUp, roundingDown, roundingUp} {
		for n := 0; n < 60; n++ {
			rate, grid := int64(90_000), int64(10_000)
			if n%3 == 0 {
				rate, grid = 89_500, 5_000
			}
			var entries []cashEntry
			for i := 0; i < 1+rng.Intn(5); i++ {
				units := int64(1+rng.Intn(150)) * 5_000
				cents, _ := roundCentsFromCurrencyUnits(units, rate*unitsPerUsdMicroScale, mode)
				if i > 0 && rng.Intn(2) == 0 {
					cents = -cents
				}
				entries = append(entries, cashEntry{cents, units})
			}
			add("random "+mode+" "+itoa(n), mode, rate, grid, entries)
		}
	}

	got, err := json.MarshalIndent(golden, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if os.Getenv("KASHI_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(cashPaymentNettingGoldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(cashPaymentNettingGoldenPath)
	if err != nil {
		t.Fatalf("%v (write it with KASHI_UPDATE_GOLDEN=1)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("checkout's LL netting changed; check the calculator still matches and rewrite %s with KASHI_UPDATE_GOLDEN=1", cashPaymentNettingGoldenPath)
	}
}
