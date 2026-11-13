package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"
)

// heldSaleSnapshot is a sale as the screen holds it (lib/holdSnapshot.ts):
// a line at a custom price with per-unit prices, a line with a custom
// discount, a line central priced from the live cart, a line with its list
// discount turned off, an unknown item with its picture ref, and the sale's
// ref and picture session.
const heldSaleSnapshot = `{"version":1,
	"items":[
		{"name":"Item A","code":"A","barcode":"bc-A","price":85,"originalPrice":100,"originalDiscount":0,"discount":0,"qty":2,"unitPrices":[85,90],"unitDiscounts":[0,0],"note":"","salespersonId":3},
		{"name":"Item B","code":"B","barcode":"bc-B","price":80,"originalPrice":80,"originalDiscount":0,"discount":15,"qty":1,"unitPrices":[80],"unitDiscounts":[15],"note":"gift wrap"},
		{"name":"Item JACKET","code":"JACKET","barcode":"bc-JACKET","price":90,"originalPrice":100,"originalDiscount":0,"discount":50,"qty":1,"unitPrices":[90],"unitDiscounts":[50],"note":"","remote":{"priceCents":9000,"discount":50,"noDeal":false,"prevPrice":100,"prevDiscount":0}},
		{"name":"Item SOCKS","code":"SOCKS","barcode":"bc-SOCKS","price":20,"originalPrice":20,"originalDiscount":10,"discount":0,"discountOff":true,"dealOff":true,"saleOption":"Summer","qty":1,"unitPrices":[20],"unitDiscounts":[0],"note":""},
		{"name":"Unknown item","code":"UNKNOWN","barcode":"UNKNOWN","price":12.5,"originalPrice":12.5,"originalDiscount":0,"discount":0,"note":"","qty":1,"unitPrices":[12.5],"unitDiscounts":[0],"unknownRef":"u1abc","unknownDeal":"none"}
	],
	"client":{"id":1,"name":"Walk-in"},
	"salesperson":{"id":3,"fullName":"Cashier"},
	"calculator":{"currentInput":"20","activeMethod":"cash","entries":[{"method":"cash","amount":20}]},
	"notes":"pick up at 5",
	"saleRef":"sale-1",
	"unknownClientRef":"pics-123"}`

// The user's report: a held sale lost central's price, its custom prices and
// discounts and its unknown item's pictures. It now comes back exactly as
// held, and central's overrides of that sale go back in force with it.
func TestHoldResumeKeepsTheWholeSale(t *testing.T) {
	shop := liveCartShop(t)
	waitLiveCartIdle(t, shop.app)
	app := shop.app
	setOverrides(t, app, liveCartOverrides{Lines: []liveCartLineOverride{
		{Barcode: "bc-JACKET", PriceCents: cents(9000), Discount: cents(50)},
	}})
	cart, _ := json.Marshal([]salesInvoiceItem{{Barcode: "bc-JACKET", Qty: 1}})
	subtotal := func() int64 {
		t.Helper()
		quote, err := app.QuoteInvoiceTotal(string(cart))
		if err != nil {
			t.Fatal(err)
		}
		return quote.DiscountedSubtotalCents
	}
	if got := subtotal(); got != 4500 {
		t.Fatalf("central's jacket: %d, want 4500", got)
	}

	id, err := app.HoldInvoice(heldSaleSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	// The next customer's sale drops central's overrides, as it must.
	reportSale(t, app, "sale-2")
	if got := subtotal(); got != 10000 {
		t.Fatalf("next sale: %d, want the list price", got)
	}

	// The hold list shows the sale, not central's overrides.
	holds, err := app.ListHoldInvoices()
	if err != nil {
		t.Fatal(err)
	}
	if len(holds) != 1 {
		t.Fatalf("holds = %+v", holds)
	}
	if _, leaked := holds[0][holdLiveOverridesKey]; leaked {
		t.Fatal("the hold list must not carry central's overrides")
	}
	assertSameSnapshot(t, heldSaleSnapshot, holds[0])

	resumed, err := app.ResumeHoldInvoice(id)
	if err != nil {
		t.Fatal(err)
	}
	assertSameSnapshot(t, heldSaleSnapshot, resumed)
	if _, leaked := resumed[holdLiveOverridesKey]; leaked {
		t.Fatal("the resumed hold must not carry central's overrides")
	}

	// Central's price is back for the held sale, and stays as the screen
	// reports it again.
	if o := app.GetLiveCartOverrides(); o.SaleRef != "sale-1" || len(o.Lines) != 1 || *o.Lines[0].PriceCents != 9000 {
		t.Fatalf("overrides after resume = %+v", o)
	}
	reportSale(t, app, "sale-1")
	if got := subtotal(); got != 4500 {
		t.Fatalf("resumed: %d, want central's 4500 again", got)
	}
	// Central can still change it: the sale is open again.
	setOverrides(t, app, liveCartOverrides{AllDealsOff: true})

	// The hold is gone; resuming it again fails and changes nothing.
	if holds, _ := app.ListHoldInvoices(); len(holds) != 0 {
		t.Fatalf("still on hold: %+v", holds)
	}
	if _, err := app.ResumeHoldInvoice(id); err == nil {
		t.Fatal("resuming a hold twice must fail")
	}
}

// A sale held with no overrides, an exchange, or a sale whose ref isn't the
// one central changed carries none, and resuming it puts none back.
func TestHoldResumeTakesOnlyItsOwnSalesOverrides(t *testing.T) {
	for _, tc := range []struct {
		name, mode, heldRef string
		overrides           bool
	}{
		{"no overrides", "sale", "sale-1", false},
		{"another sale's ref", "sale", "sale-9", true},
		{"no ref (an old screen)", "sale", "", true},
		{"an exchange", "exchange", "sale-1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shop := liveCartShop(t)
			waitLiveCartIdle(t, shop.app)
			app := shop.app
			if tc.overrides {
				setOverrides(t, app, liveCartOverrides{AllDealsOff: true})
			}
			if tc.mode != "sale" {
				raw, _ := json.Marshal(map[string]any{"saleRef": "sale-1", "mode": tc.mode, "lines": []any{}})
				if err := app.ReportLiveCart(string(raw)); err != nil {
					t.Fatal(err)
				}
				waitLiveCartIdle(t, app)
			}
			snap := fmt.Sprintf(`{"version":1,"items":[{"code":"A","barcode":"bc-A","price":100,"qty":1}],"saleRef":%q}`, tc.heldRef)
			id, err := app.HoldInvoice(snap)
			if err != nil {
				t.Fatal(err)
			}
			stored := storedHoldJSON(t, shop, id)
			if bytes.Contains([]byte(stored), []byte(holdLiveOverridesKey)) {
				t.Fatalf("held overrides it doesn't own: %s", stored)
			}
			reportSale(t, app, "sale-2")
			if _, err := app.ResumeHoldInvoice(id); err != nil {
				t.Fatal(err)
			}
			if o := app.GetLiveCartOverrides(); !o.empty() {
				t.Fatalf("resume put overrides back: %+v", o)
			}
		})
	}
}

// Several holds, each with central's own overrides: each resumes with its
// own, never another's, and the till prices it as before it was held.
func TestHoldResumeRandomized(t *testing.T) {
	shop := liveCartShop(t)
	waitLiveCartIdle(t, shop.app)
	app := shop.app
	rng := rand.New(rand.NewSource(7))
	barcodes := []string{"bc-A", "bc-B", "bc-C", "bc-D", "bc-JACKET", "bc-SOCKS"}

	type held struct {
		id       int64
		ref      string
		snapshot string
		cart     string
		quote    int64
	}
	var holds []held
	for n := 0; n < 300; n++ {
		ref := fmt.Sprintf("sale-r%d", n)
		reportSale(t, app, ref)

		var cart []salesInvoiceItem
		var lines []map[string]any
		for _, bc := range barcodes {
			if rng.Intn(2) == 0 {
				continue
			}
			qty := float64(1 + rng.Intn(3))
			cart = append(cart, salesInvoiceItem{Barcode: bc, Qty: qty})
			unitPrices := make([]float64, int(qty))
			for i := range unitPrices {
				unitPrices[i] = float64(1+rng.Intn(20000)) / 100
			}
			lines = append(lines, map[string]any{
				"code": bc[3:], "barcode": bc, "qty": qty, "price": unitPrices[0],
				"unitPrices": unitPrices, "discount": rng.Intn(101), "note": fmt.Sprintf("n%d", rng.Intn(9)),
				"discountOff": rng.Intn(2) == 0, "dealOff": rng.Intn(2) == 0,
			})
		}
		if len(cart) == 0 {
			cart = append(cart, salesInvoiceItem{Barcode: "bc-A", Qty: 1})
			lines = append(lines, map[string]any{"code": "A", "barcode": "bc-A", "qty": 1.0, "price": 100.0})
		}
		o := liveCartOverrides{SaleRef: ref, AllDealsOff: rng.Intn(3) == 0}
		for _, it := range cart {
			if rng.Intn(2) == 0 {
				continue
			}
			l := liveCartLineOverride{Barcode: it.Barcode, NoDeal: rng.Intn(3) == 0}
			if rng.Intn(2) == 0 {
				l.PriceCents = cents(int64(1 + rng.Intn(50000)))
			}
			if rng.Intn(2) == 0 {
				l.Discount = cents(int64(rng.Intn(101)))
			}
			o.Lines = append(o.Lines, l)
		}
		raw, _ := json.Marshal(o)
		if err := app.executeCartOverrideCommand(raw); err != nil {
			t.Fatal(err)
		}
		cartJSON, _ := json.Marshal(cart)
		quote, err := app.QuoteInvoiceTotal(string(cartJSON))
		if err != nil {
			t.Fatal(err)
		}
		snapshot, _ := json.Marshal(map[string]any{
			"version": 1, "items": lines, "saleRef": ref, "notes": ref,
			"unknownClientRef": fmt.Sprintf("pics-%d", n),
		})
		id, err := app.HoldInvoice(string(snapshot))
		if err != nil {
			t.Fatal(err)
		}
		holds = append(holds, held{id, ref, string(snapshot), string(cartJSON), quote.GrandTotalCents})

		// Now and then a held sale comes back, in any order.
		if rng.Intn(3) == 0 {
			reportSale(t, app, fmt.Sprintf("other-%d", n))
			at := rng.Intn(len(holds))
			h := holds[at]
			holds = append(holds[:at], holds[at+1:]...)
			resumed, err := app.ResumeHoldInvoice(h.id)
			if err != nil {
				t.Fatal(err)
			}
			assertSameSnapshot(t, h.snapshot, resumed)
			reportSale(t, app, h.ref)
			quote, err := app.QuoteInvoiceTotal(h.cart)
			if err != nil {
				t.Fatal(err)
			}
			if quote.GrandTotalCents != h.quote {
				t.Fatalf("hold %s: resumed at %d, held at %d", h.ref, quote.GrandTotalCents, h.quote)
			}
		}
	}
	// Whatever is left is still listed as held, line for line.
	listed, err := app.ListHoldInvoices()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != len(holds) {
		t.Fatalf("listed %d holds, want %d", len(listed), len(holds))
	}
}

// The resumed sample hold, for the screen's test (holdSnapshot.test.ts) to
// put back on the cart; rewrite it with KASHI_UPDATE_GOLDEN=1.
const heldSaleGoldenPath = "frontend/src/lib/testdata/held_sale.json"

func TestHoldResumeGolden(t *testing.T) {
	shop := liveCartShop(t)
	waitLiveCartIdle(t, shop.app)
	id, err := shop.app.HoldInvoice(heldSaleSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := shop.app.ResumeHoldInvoice(id)
	if err != nil {
		t.Fatal(err)
	}
	delete(resumed, "id")
	delete(resumed, "createdAt")
	got, err := json.MarshalIndent(resumed, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if os.Getenv("KASHI_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(heldSaleGoldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(heldSaleGoldenPath)
	if err != nil {
		t.Fatalf("%v (write it with KASHI_UPDATE_GOLDEN=1)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("a resumed hold changed; check the screen still restores it and rewrite %s with KASHI_UPDATE_GOLDEN=1:\n%s", heldSaleGoldenPath, got)
	}
}

func reportSale(t *testing.T, app *App, saleRef string) {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"saleRef": saleRef, "mode": "sale", "lines": []any{}})
	if err := app.ReportLiveCart(string(raw)); err != nil {
		t.Fatal(err)
	}
	waitLiveCartIdle(t, app)
}

// waitLiveCartIdle waits for the report sender to finish: the test database
// has no busy timeout, so a hold written while it reads settings fails.
func waitLiveCartIdle(t *testing.T, app *App) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		app.liveCart.mu.Lock()
		busy := app.liveCart.sending
		app.liveCart.mu.Unlock()
		if !busy {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("the live cart report never finished")
}

// assertSameSnapshot: the hold as listed or resumed is the snapshot as sent,
// plus the list's id, createdAt and itemCount.
func assertSameSnapshot(t *testing.T, sent string, got map[string]any) {
	t.Helper()
	var want map[string]any
	if err := json.Unmarshal([]byte(sent), &want); err != nil {
		t.Fatal(err)
	}
	rest := map[string]any{}
	for k, v := range got {
		switch k {
		case "id", "createdAt":
		case "itemCount":
			if v != len(want["items"].([]any)) {
				t.Fatalf("itemCount = %v", v)
			}
		default:
			rest[k] = v
		}
	}
	if !reflect.DeepEqual(rest, want) {
		a, _ := json.Marshal(rest)
		b, _ := json.Marshal(want)
		t.Fatalf("hold came back changed:\n got %s\nwant %s", a, b)
	}
}

func storedHoldJSON(t *testing.T, shop dealShop, id int64) string {
	t.Helper()
	list, err := shop.store.ListHoldInvoices(shop.app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range list {
		if h.ID == id {
			return h.SnapshotJson
		}
	}
	t.Fatalf("hold %d not stored", id)
	return ""
}
