package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

func setStack(t *testing.T, shop dealShop, on bool) {
	t.Helper()
	if err := shop.app.SetDealsStackEnabled(on); err != nil {
		t.Fatal(err)
	}
}

func addTiers(t *testing.T, shop dealShop) {
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "t50", Name: "10% above $50", MinTotal: 5000, Kind: invoiceDiscountPercent, Percent: 10})
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "t100", Name: "15% above $100", MinTotal: 10000, Kind: invoiceDiscountPercent, Percent: 15})
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "t150", Name: "20% above $150", MinTotal: 15000, Kind: invoiceDiscountPercent, Percent: 20})
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "students", Name: "Students 15%", Kind: invoiceDiscountPercent, Percent: 15, CashierPicks: true})
}

// The reported case: with the branch's deals set to add up, a $200 sale for
// a student takes 20%, then 15%, 15% and 10% off what is left each time.
func TestStackedDeals(t *testing.T) {
	shop := setUpDealShop(t)
	addTiers(t, shop)

	// Off (the default): the biggest takes over.
	sale, _ := shop.sellPicked(t, []string{"students"}, "JACKET", "JACKET")
	if sale.InvoiceDiscount.Name != "20% above $150" || sale.InvoiceDiscount.Cents != 4000 || sale.GrandTotal != 16000 {
		t.Fatalf("takes over: %+v %d", sale.InvoiceDiscount, sale.GrandTotal)
	}

	setStack(t, shop, true)
	// 20000 -20% 16000 -15% 13600 -15% 11560 -10% 10404.
	sale, quote := shop.sellPicked(t, []string{"students"}, "JACKET", "JACKET")
	wantName := "20% above $150 + 15% above $100 + Students 15% + 10% above $50"
	if sale.InvoiceDiscount.Name != wantName || quote.InvoiceDiscountName != wantName {
		t.Fatalf("name %q / %q", sale.InvoiceDiscount.Name, quote.InvoiceDiscountName)
	}
	// $104.04 after the deals; this shop rounds what is paid to $104.00.
	if sale.InvoiceDiscount.Cents != 9596 || quote.DiscountedSubtotalCents != 10404 || sale.GrandTotal != quote.GrandTotalCents {
		t.Fatalf("stacked: %+v subtotal %d total %d quote %d", sale.InvoiceDiscount, quote.DiscountedSubtotalCents, sale.GrandTotal, quote.GrandTotalCents)
	}
	inv, err := shop.store.GetInvoiceByID(context.Background(), sale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	if inv.InvoiceDiscountName != wantName || inv.GrandTotal != sale.GrandTotal {
		t.Fatalf("stored: %q %d", inv.InvoiceDiscountName, inv.GrandTotal)
	}

	// Returned jacket by jacket: the two jackets are one line paying $104.04
	// (all of the deals off it); each unit back is half, rounded as this
	// shop pays: $52.
	lines, err := shop.store.ListInvoiceProducts(context.Background(), sale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lines {
		if l.LineTotal != 10404 || l.InvoiceDiscount != 9596 || l.Quantity != 2 {
			t.Fatalf("line %+v", l)
		}
	}
	first := shop.refund(t, sale.InvoiceID, "JACKET")
	second := shop.refund(t, sale.InvoiceID, "JACKET")
	if first != -5200 || second != -5200 {
		t.Fatalf("refunds %d, %d, want -5200 each", first, second)
	}
	// Central hears the joined name and the whole amount.
	entries, err := shop.store.ListDueSyncOutboxEntries(context.Background(), 200)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		var p branchInvoicePayload
		if e.EntityType == "sales_invoice" && json.Unmarshal([]byte(e.PayloadJson), &p) == nil && p.BranchInvoiceCode == sale.InvoiceCode {
			found = p.InvoiceDiscount == 9596 && p.InvoiceDiscountName == wantName
		}
	}
	if !found {
		t.Fatal("central's payload lacks the stacked deal")
	}

	// Not picked: only the tiers the sale reaches. $60: 10% only.
	sale, _ = shop.sellPicked(t, nil, "SOCKS", "SOCKS", "SOCKS")
	if sale.InvoiceDiscount.Name != "10% above $50" || sale.InvoiceDiscount.Cents != 600 {
		t.Fatalf("$60: %+v", sale.InvoiceDiscount)
	}
	// A minimum is judged on the sale before any deal: $160 reaches $150
	// though 20% off leaves it under.
	sale, _ = shop.sellPicked(t, nil, "JACKET", "SOCKS", "SOCKS", "SOCKS")
	if !strings.HasPrefix(sale.InvoiceDiscount.Name, "20% above $150 + 15% above $100") {
		t.Fatalf("$160: %+v", sale.InvoiceDiscount)
	}
	// Back off: the biggest again.
	setStack(t, shop, false)
	if sale, _ = shop.sellPicked(t, []string{"students"}, "JACKET", "JACKET"); sale.InvoiceDiscount.Name != "20% above $150" {
		t.Fatalf("off again: %+v", sale.InvoiceDiscount)
	}
}

// Fixed amounts come after the percents and never take the sale below zero.
func TestStackedDealsAmounts(t *testing.T) {
	rules := []db.InvoiceDiscount{
		{Uid: "a", Name: "$30 off", Kind: invoiceDiscountAmount, Amount: 3000},
		{Uid: "p", Name: "10% off", Kind: invoiceDiscountPercent, Percent: 10},
		{Uid: "b", Name: "$5 off", Kind: invoiceDiscountAmount, Amount: 500},
	}
	got := invoiceDiscountsStacked(rules, 10000, time.Now(), "half_up")
	// 10000 -10% 9000 -3000 6000 -500 5500.
	if got.Name != "10% off + $30 off + $5 off" || got.Cents != 4500 {
		t.Fatalf("%+v", got)
	}
	got = invoiceDiscountsStacked(rules, 2000, time.Now(), "half_up")
	// 2000 -200 1800 -1800 (capped) 0; the $5 has nothing left to take.
	if got.Cents != 2000 || got.Name != "10% off + $30 off" {
		t.Fatalf("capped: %+v", got)
	}
	if got := invoiceDiscountsStacked(rules, 0, time.Now(), "half_up"); got.Cents != 0 || got.Name != "" {
		t.Fatalf("zero: %+v", got)
	}
	// One rule alone is what bestInvoiceDiscount gives.
	one := rules[1:2]
	if a, b := invoiceDiscountsStacked(one, 12345, time.Now(), "half_up"), bestInvoiceDiscount(one, 12345, time.Now(), "half_up"); a != b {
		t.Fatalf("one rule: %+v vs %+v", a, b)
	}
}

// stackReference is the rule written plainly, to check the till against.
func stackReference(rules []db.InvoiceDiscount, total int64, picked []string, mode string) (int64, []string) {
	type r struct {
		name     string
		pct, amt int64
	}
	var on []r
	for _, x := range rules {
		if total < x.MinTotal || (x.CashierPicks != 0 && !slices.Contains(picked, x.Uid)) {
			continue
		}
		on = append(on, r{x.Name, x.Percent, x.Amount})
	}
	left := total
	var names []string
	take := func(x r, c int64) {
		c = min(c, left)
		if c > 0 {
			left -= c
			names = append(names, x.name)
		}
	}
	for p := int64(100); p >= 1; p-- {
		var same []r
		for _, x := range on {
			if x.pct == p {
				same = append(same, x)
			}
		}
		slices.SortFunc(same, func(a, b r) int { return strings.Compare(a.name, b.name) })
		for _, x := range same {
			take(x, percentOf(left, x.pct, mode))
		}
	}
	var amts []r
	for _, x := range on {
		if x.pct == 0 && x.amt > 0 {
			amts = append(amts, x)
		}
	}
	slices.SortFunc(amts, func(a, b r) int {
		if a.amt != b.amt {
			return int(b.amt - a.amt)
		}
		return strings.Compare(a.name, b.name)
	})
	for _, x := range amts {
		take(x, x.amt)
	}
	return total - left, names
}

// Randomized: with deals adding up, hundreds of sales through quote and
// checkout: what comes off is the plain rule's, the quote is what checkout
// charges and stores, and the lines carry the discount to the cent.
func TestStackedDealsRandomized(t *testing.T) {
	shop := setUpDealShop(t)
	setStack(t, shop, true)
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
	ctx := context.Background()
	rules, err := shop.store.ListInvoiceDiscounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	codes := []string{"A", "B", "C", "JACKET", "SOCKS"}
	sold := 0
	for run := 0; run < 200; run++ {
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
		quote := shop.quotePicked(t, picked, cart...)
		if quote.GrandTotalCents <= 0 {
			continue
		}
		// The sale before any invoice deal (this shop has no referral reward).
		before := quote.DiscountedSubtotalCents + quote.InvoiceDiscountCents
		wantCents, wantNames := stackReference(rules, before, picked, "half_up")
		sale, q2 := shop.sellPicked(t, picked, cart...)
		if q2.GrandTotalCents != sale.GrandTotal {
			t.Fatalf("quote %d, checkout %d", q2.GrandTotalCents, sale.GrandTotal)
		}
		if sale.InvoiceDiscount.Cents != wantCents || sale.InvoiceDiscount.Name != strings.Join(wantNames, " + ") {
			t.Fatalf("cart %v picked %v: got %+v, want %d %q", cart, picked, sale.InvoiceDiscount, wantCents, strings.Join(wantNames, " + "))
		}
		lines, err := shop.store.ListInvoiceProducts(ctx, sale.InvoiceID)
		if err != nil {
			t.Fatal(err)
		}
		var shares int64
		for _, l := range lines {
			shares += l.InvoiceDiscount
		}
		if shares != sale.InvoiceDiscount.Cents {
			t.Fatalf("lines carry %d of %d", shares, sale.InvoiceDiscount.Cents)
		}
		inv, err := shop.store.GetInvoiceByID(ctx, sale.InvoiceID)
		if err != nil {
			t.Fatal(err)
		}
		if inv.GrandTotal != sale.GrandTotal || inv.InvoiceDiscountName != sale.InvoiceDiscount.Name {
			t.Fatalf("stored %d %q", inv.GrandTotal, inv.InvoiceDiscountName)
		}
		sold++
	}
	if sold < 50 {
		t.Fatalf("only %d sales ran", sold)
	}
}

// Central turns adding-up on and off for a branch with update_settings, and
// hears it back in the till's settings push.
func TestDealsStackFromCentral(t *testing.T) {
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/branch/settings" {
			body, _ = io.ReadAll(r.Body)
		}
		w.Write([]byte(`{}`))
	}))
	defer server.Close()
	app, _, _, _ := setUpTestInvoiceApp(t, server.URL)
	ctx := app.ctx
	report := func() bool {
		t.Helper()
		if err := app.pushSettings(ctx, newCentralClient(server.URL, "TEST01", "test-key")); err != nil {
			t.Fatal(err)
		}
		var got branchSettingsPayload
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		return got.DealsStack
	}
	if app.flagEnabled(ctx, dealsStackFlag) || report() {
		t.Fatal("deals add up before anyone said so")
	}
	// Every command carries central's core settings alongside what changed.
	rate, half, usd, name := int64(1), "half_up", "USD", "Test"
	command := func(stack bool) json.RawMessage {
		raw, _ := json.Marshal(updateSettingsCommandPayload{BranchName: &name, RoundingMode: &half, RoundingCurrency: &usd, ExchangeRate: &rate, DealsStack: &stack})
		return raw
	}
	if err := app.executeUpdateSettingsCommand(ctx, command(true)); err != nil {
		t.Fatal(err)
	}
	settings, err := app.GetAppSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !app.flagEnabled(ctx, dealsStackFlag) || settings["dealsStack"] != true || !report() {
		t.Fatalf("on: flag %v, screen %v", app.flagEnabled(ctx, dealsStackFlag), settings["dealsStack"])
	}
	if err := app.executeUpdateSettingsCommand(ctx, command(false)); err != nil {
		t.Fatal(err)
	}
	if app.flagEnabled(ctx, dealsStackFlag) || report() {
		t.Fatal("deals_stack still on")
	}
}

func TestSplitDealWords(t *testing.T) {
	short := func(s string) bool { return len(s) <= 20 }
	got := splitDealWords("20% above $150 + Students 15% + 10% above $50", short)
	want := []string{"20% above $150 +", "Students 15% +", "10% above $50"}
	if !slices.Equal(got, want) {
		t.Fatalf("%q", got)
	}
	if got := splitDealWords("A + B + C", short); !slices.Equal(got, []string{"A + B + C"}) {
		t.Fatalf("fits: %q", got)
	}
	// One long deal can't be broken: it stays as it is.
	if got := splitDealWords("A very long single deal name indeed", short); len(got) != 1 {
		t.Fatalf("single: %q", got)
	}
	// Every deal is kept, in order.
	words := "Rule 1 + Rule 2 + Rule 3 + Rule 4 + Rule 5"
	if got := strings.ReplaceAll(strings.Join(splitDealWords(words, func(s string) bool { return len(s) <= 15 }), " "), "+ ", "+ "); got != words {
		t.Fatalf("joined back: %q", got)
	}
}

// The reported case on paper: deals that added up print every name, on as
// many lines as they need, never over the amount; the text receipt too.
func TestStackedDealsOnTheReceipt(t *testing.T) {
	name := "20% above $150 + 15% above $100 + Students 15% + 10% above $50"
	data := receiptPreviewData(designBase(), receiptModeSale)
	data.InvoiceDiscount = invoiceDiscount{Name: name, Cents: 9596}
	for _, theme := range receiptThemeCatalog() {
		drawn, _, _ := drawnTexts(t, data, theme.ID, theme.Layouts[0].ID)
		all := strings.Join(drawn, " ")
		for _, deal := range strings.Split(name, " + ") {
			if !strings.Contains(all, deal) {
				t.Fatalf("%s: %q missing from %q", theme.ID, deal, drawn)
			}
		}
		// No drawn piece holds the whole joined name: it was broken up.
		for _, d := range drawn {
			if d == name {
				t.Fatalf("%s: the whole name on one line", theme.ID)
			}
		}
	}
	text := string(buildESCPosReceipt(data))
	for _, deal := range strings.Split(name, " + ") {
		if !strings.Contains(text, deal) {
			t.Fatalf("text receipt lacks %q:\n%s", deal, text)
		}
	}
	if strings.Contains(text, "~") {
		t.Fatalf("text receipt cut a name:\n%s", text)
	}
}
