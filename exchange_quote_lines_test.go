package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"
)

// The exchange screen shows each line's amount and a breakdown under the
// total; these tests hold the quote to the promise that what it shows adds
// up, line by line and cent by cent, to the total checkout then charges.

func TestSplitUnits(t *testing.T) {
	for amount := int64(-25); amount <= 25; amount++ {
		for qty := int64(1); qty <= 7; qty++ {
			units := splitUnits(amount, qty)
			if int64(len(units)) != qty {
				t.Fatalf("splitUnits(%d, %d) gave %d units", amount, qty, len(units))
			}
			var sum, lo, hi int64
			lo, hi = units[0], units[0]
			for _, u := range units {
				sum += u
				lo, hi = min(lo, u), max(hi, u)
			}
			if sum != amount || hi-lo > 1 {
				t.Fatalf("splitUnits(%d, %d) = %v: sum %d, spread %d", amount, qty, units, sum, hi-lo)
			}
		}
	}
	if splitUnits(100, 0) != nil {
		t.Fatal("no units, nothing to split")
	}
}

// line is one cart line as the till sends it.
func line(code string, qty int64) exchangeLine {
	return exchangeLine{Code: code, Barcode: "bc-" + code, Qty: qty}
}

// quoteExchangeLines quotes lines on a sale and holds the quote to every
// identity the screen relies on.
func quoteExchangeLines(t *testing.T, shop dealShop, saleID int64, lines []exchangeLine) ExchangeQuote {
	t.Helper()
	raw, _ := json.Marshal(lines)
	quote, err := shop.app.QuoteExchangeTotal(string(raw), saleID)
	if err != nil {
		t.Fatalf("quote %s: %v", raw, err)
	}
	if msg := quoteInconsistency(quote, lines); msg != "" {
		t.Fatalf("quote %s: %s\n%+v", raw, msg, quote)
	}
	return quote
}

// quoteInconsistency is "" when the quote adds up; otherwise what doesn't.
func quoteInconsistency(q ExchangeQuote, sent []exchangeLine) string {
	if len(q.Lines) != len(sent) {
		return fmt.Sprintf("%d lines quoted, %d sent", len(q.Lines), len(sent))
	}
	var sum, chargedBack, replacements, returned int64
	for i, l := range q.Lines {
		s := sent[i]
		if l.Index != i || l.Qty != s.Qty || !strings.EqualFold(l.Code, strings.TrimSpace(s.Code)) {
			return fmt.Sprintf("line %d is %+v, sent %+v", i, l, s)
		}
		sum += l.LineTotalCents
		chargedBack += l.ChargedBackCents
		if l.Qty < 0 {
			returned -= l.LineTotalCents
			if -l.LineTotalCents != l.PaidCents+l.SwapCreditCents-l.ChargedBackCents {
				return fmt.Sprintf("returned line %d: credit %d != paid %d + swap %d - charged back %d", i, -l.LineTotalCents, l.PaidCents, l.SwapCreditCents, l.ChargedBackCents)
			}
			if l.PaidCents < 0 || l.SwapCreditCents < 0 || l.ChargedBackCents < 0 {
				return fmt.Sprintf("returned line %d has a negative part: %+v", i, l)
			}
			// What was paid, and the free unit's credit, never come to more
			// than the shelf price of what came back.
			if l.PaidCents+l.SwapCreditCents > -l.ShelfCents {
				return fmt.Sprintf("returned line %d credits %d over its shelf price %d", i, l.PaidCents+l.SwapCreditCents, -l.ShelfCents)
			}
			if l.Free && l.PaidCents != 0 {
				return fmt.Sprintf("free line %d was paid %d", i, l.PaidCents)
			}
			// A free unit's swap credit is up to its shelf price, and only a
			// free (or reduced) unit has one.
			if l.SwapCreditCents > l.SwapAllowanceCents || l.SwapAllowanceCents < 0 || l.SwapAllowanceCents > -l.ShelfCents {
				return fmt.Sprintf("returned line %d swaps %d of an allowance of %d (shelf %d)", i, l.SwapCreditCents, l.SwapAllowanceCents, -l.ShelfCents)
			}
			if l.Free && l.SwapAllowanceCents != -l.ShelfCents {
				return fmt.Sprintf("free line %d has an allowance of %d, not its shelf price %d", i, l.SwapAllowanceCents, -l.ShelfCents)
			}
		} else {
			replacements += l.LineTotalCents
			if l.PaidCents != 0 || l.SwapCreditCents != 0 || l.ChargedBackCents != 0 || l.Free {
				return fmt.Sprintf("replacement line %d has a returned line's parts: %+v", i, l)
			}
			if l.LineTotalCents < 0 || l.LineTotalCents > l.ShelfCents {
				return fmt.Sprintf("replacement line %d is %d on a shelf price of %d", i, l.LineTotalCents, l.ShelfCents)
			}
		}
	}
	var adjusted int64
	for _, a := range q.Adjustments {
		adjusted += a.ChargedBackCents
		if a.Deal == "" || a.ChargedBackCents < 0 || a.ForfeitedCents < 0 || (a.ChargedBackCents == 0 && a.ForfeitedCents == 0) {
			return fmt.Sprintf("adjustment %+v", a)
		}
	}
	switch {
	case sum != q.NetCents:
		return fmt.Sprintf("lines add up to %d, net is %d", sum, q.NetCents)
	case replacements != q.ReplacementTotalCents:
		return fmt.Sprintf("replacement lines add up to %d, replacements are %d", replacements, q.ReplacementTotalCents)
	case returned != q.ReturnedTotalCents:
		return fmt.Sprintf("returned lines add up to %d, returned is %d", returned, q.ReturnedTotalCents)
	case q.NetCents != q.ReplacementTotalCents-q.ReturnedTotalCents:
		return fmt.Sprintf("net %d != replacements %d - returned %d", q.NetCents, q.ReplacementTotalCents, q.ReturnedTotalCents)
	case q.NetCents+q.TaxCents != q.UnroundedCents:
		return fmt.Sprintf("net %d + tax %d != %d", q.NetCents, q.TaxCents, q.UnroundedCents)
	case q.GrandTotalCents != max(0, q.UnroundedCents+q.RoundingCents):
		return fmt.Sprintf("total %d != max(0, %d + rounding %d)", q.GrandTotalCents, q.UnroundedCents, q.RoundingCents)
	case chargedBack != adjusted:
		return fmt.Sprintf("lines charge back %d, the deals %d", chargedBack, adjusted)
	case q.ShortfallCents < 0 || (q.ShortfallCents > 0) != q.NeedsMoreReplacementValue:
		return fmt.Sprintf("shortfall %d, needs more %v", q.ShortfallCents, q.NeedsMoreReplacementValue)
	// "Add $X more" is what the lines on screen are short of, to the cent:
	// the credit shown less the new items shown.
	case q.NeedsMoreReplacementValue && q.ShortfallCents != -q.NetCents:
		return fmt.Sprintf("asks for %d more, the lines are short %d", q.ShortfallCents, -q.NetCents)
	// And an exchange that asks for nothing more never pays out.
	case !q.NeedsMoreReplacementValue && q.NetCents < 0:
		return fmt.Sprintf("asks for nothing more with the lines at %d", q.NetCents)
	}
	return ""
}

// checkoutQuoted checks the exchange out paying what the quote asks and
// holds what was stored to the quote: the invoice's total, and each
// product's returned and sold amounts.
func checkoutQuoted(t *testing.T, shop dealShop, saleID int64, lines []exchangeLine, quote ExchangeQuote) {
	t.Helper()
	if msg := checkoutMismatch(shop, saleID, lines, quote); msg != "" {
		t.Fatal(msg)
	}
}

func checkoutMismatch(shop dealShop, saleID int64, lines []exchangeLine, quote ExchangeQuote) string {
	raw, _ := json.Marshal(lines)
	payments := "[]"
	if quote.GrandTotalCents > 0 {
		p, _ := json.Marshal([]checkoutPayment{{AccountID: shop.cashbox.ID, AmountCents: quote.GrandTotalCents}})
		payments = string(p)
	}
	if _, err := shop.app.checkoutReturnExchangeInvoice(string(raw), payments, saleID, shop.salesperson.ID, "", 0, posExchangePolicy); err != nil {
		return fmt.Sprintf("checkout of quoted %s (total %d): %v", raw, quote.GrandTotalCents, err)
	}
	var invoiceID, grandTotal int64
	if err := shop.app.conn.QueryRow(`SELECT i.id, i.grand_total FROM invoices i JOIN return_invoices r ON r.invoice_id = i.id ORDER BY i.id DESC LIMIT 1`).Scan(&invoiceID, &grandTotal); err != nil {
		return err.Error()
	}
	if want := quote.UnroundedCents + quote.RoundingCents; grandTotal != want {
		return fmt.Sprintf("stored total %d, the quote said %d", grandTotal, want)
	}
	stored := map[string]int64{}
	rows, err := shop.app.conn.Query(`SELECT p.barcode, ip.quantity, ip.line_total FROM invoice_products ip JOIN products p ON p.id = ip.product_id WHERE ip.invoice_id = ?`, invoiceID)
	if err != nil {
		return err.Error()
	}
	defer rows.Close()
	var storedSum int64
	for rows.Next() {
		var barcode string
		var qty, total int64
		if err := rows.Scan(&barcode, &qty, &total); err != nil {
			return err.Error()
		}
		stored[fmt.Sprintf("%s %v", barcode, qty < 0)] += total
		storedSum += total
	}
	quoted := map[string]int64{}
	for _, l := range quote.Lines {
		quoted[fmt.Sprintf("%s %v", l.Barcode, l.Qty < 0)] += l.LineTotalCents
	}
	if fmt.Sprint(stored) != fmt.Sprint(quoted) {
		return fmt.Sprintf("stored lines %v, quoted %v", stored, quoted)
	}
	if storedSum != quote.NetCents {
		return fmt.Sprintf("stored lines add up to %d, the quote's net is %d", storedSum, quote.NetCents)
	}
	return syncedReturnMismatch(shop, quote)
}

// syncedReturnMismatch holds the exchange sent to kashi to the quote: its
// lines add up to the net and its total to the till's, and every line says
// why it is what it is, so kashi's invoice view can show it:
//
//	line = unit*qty - percent off - deal - invoice share - swap credit + charged back
func syncedReturnMismatch(shop dealShop, quote ExchangeQuote) string {
	var raw string
	if err := shop.app.conn.QueryRow(`SELECT payload_json FROM sync_outbox WHERE entity_type = 'return_invoice' ORDER BY id DESC LIMIT 1`).Scan(&raw); err != nil {
		return "no exchange queued for kashi: " + err.Error()
	}
	var payload branchInvoicePayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return err.Error()
	}
	if want := quote.UnroundedCents + quote.RoundingCents; payload.GrandTotal != want {
		return fmt.Sprintf("kashi is sent a total of %d, the till's is %d", payload.GrandTotal, want)
	}
	var sum, chargedBack, swap int64
	for _, item := range payload.Items {
		sum += item.LineTotal
		chargedBack += item.ChargedBack
		swap += item.SwapCredit
		if item.ChargedBack < 0 || item.SwapCredit < 0 {
			return fmt.Sprintf("kashi is sent a negative charge back or swap credit: %+v", item)
		}
		if item.Quantity > 0 && (item.ChargedBack != 0 || item.SwapCredit != 0 || item.DealDiscount != 0 || item.InvoiceDiscount != 0) {
			return fmt.Sprintf("a new item is sent a returned line's parts: %+v", item)
		}
		percentOff := roundRatio(item.UnitPrice*int64(item.Discount), 100, "half_up") * item.Quantity
		want := item.UnitPrice*item.Quantity - percentOff - item.DealDiscount - item.InvoiceDiscount - item.SwapCredit + item.ChargedBack
		if item.LineTotal != want {
			return fmt.Sprintf("kashi can't tell why line %s is %d: its parts make %d (%+v)", item.Barcode, item.LineTotal, want, item)
		}
	}
	if sum != quote.NetCents {
		return fmt.Sprintf("the lines sent to kashi add up to %d, the net is %d", sum, quote.NetCents)
	}
	var quotedBack, quotedSwap int64
	for _, l := range quote.Lines {
		quotedBack += l.ChargedBackCents
		quotedSwap += l.SwapCreditCents
	}
	if chargedBack != quotedBack || swap != quotedSwap {
		return fmt.Sprintf("kashi is sent %d charged back and %d swap credit, the screen showed %d and %d", chargedBack, swap, quotedBack, quotedSwap)
	}
	return ""
}

// The till's RS/CT/2026/00068: the hoodie's credit is $12, not $15, because
// the Buy 1 Get 1 was charged back $3 off it. kashi is told so, line by line.
func TestExchangeSendsKashiWhyACreditIsWhatItIs(t *testing.T) {
	shop := setUpDealShop(t)
	sale := shop.sell(t, "A", "B", "C")
	lines := []exchangeLine{line("A", -1), line("JACKET", 1)}
	quote := quoteExchangeLines(t, shop, sale.InvoiceID, lines)
	checkoutQuoted(t, shop, sale.InvoiceID, lines, quote)
	var raw string
	if err := shop.app.conn.QueryRow(`SELECT payload_json FROM sync_outbox WHERE entity_type = 'return_invoice' ORDER BY id DESC LIMIT 1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var payload branchInvoicePayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	a := payload.Items[0]
	if a.Quantity != -1 || a.UnitPrice != 10000 || a.LineTotal != -5000 || a.ChargedBack != 5000 || a.Deal != "Buy 2 get 1" {
		t.Fatalf("returned A sent as %+v", a)
	}

	// A free unit for socks: its swap credit is sent too.
	shop = setUpDealShop(t)
	sale = shop.sell(t, "A", "B", "C")
	lines = []exchangeLine{line("C", -1), line("SOCKS", 1)}
	checkoutQuoted(t, shop, sale.InvoiceID, lines, quoteExchangeLines(t, shop, sale.InvoiceID, lines))
	if err := shop.app.conn.QueryRow(`SELECT payload_json FROM sync_outbox WHERE entity_type = 'return_invoice' ORDER BY id DESC LIMIT 1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	payload = branchInvoicePayload{} // fresh: an omitted field would keep the first payload's value
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	if c := payload.Items[0]; c.LineTotal != -2000 || c.SwapCredit != 2000 || c.DealDiscount != -5000 || c.ChargedBack != 0 {
		t.Fatalf("returned free C sent as %+v", c)
	}
}

type wantLine struct {
	total, paid, swap, back int64
	free                    bool
}

func checkLines(t *testing.T, quote ExchangeQuote, want ...wantLine) {
	t.Helper()
	if len(quote.Lines) != len(want) {
		t.Fatalf("%d lines, want %d: %+v", len(quote.Lines), len(want), quote.Lines)
	}
	for i, w := range want {
		l := quote.Lines[i]
		got := wantLine{l.LineTotalCents, l.PaidCents, l.SwapCreditCents, l.ChargedBackCents, l.Free}
		if got != w {
			t.Fatalf("line %d (%s) = %+v, want %+v", i, l.Code, got, w)
		}
	}
}

// Sale: A $100, B $80, C $50 on Buy 2 get 1, C free, $180 paid.
func TestExchangeQuoteLinesOnADeal(t *testing.T) {
	cases := []struct {
		name  string
		lines []exchangeLine
		want  []wantLine
		total int64
		adj   []ExchangeAdjustmentQuote
	}{
		{
			// The free C swaps for $20 socks: $20 of its credit used, $30 gone.
			name:  "free unit for cheaper socks",
			lines: []exchangeLine{line("C", -1), line("SOCKS", 1)},
			want:  []wantLine{{total: -2000, swap: 2000, free: true}, {total: 2000}},
			total: 0,
			adj:   []ExchangeAdjustmentQuote{{Deal: "Buy 2 get 1", ForfeitedCents: 3000}},
		},
		{
			name:  "free unit for dearer D",
			lines: []exchangeLine{line("C", -1), line("D", 1)},
			want:  []wantLine{{total: -5000, swap: 5000, free: true}, {total: 9000}},
			total: 4000,
		},
		{
			// The paid A goes for a jacket off the deal: C stops being free,
			// its $50 comes off A's $100 credit. The screen shows -$50, not
			// the -$100 the customer paid, and the reason.
			name:  "paid unit for a jacket off the deal",
			lines: []exchangeLine{line("A", -1), line("JACKET", 1)},
			want:  []wantLine{{total: -5000, paid: 10000, back: 5000}, {total: 10000}},
			total: 5000,
			adj:   []ExchangeAdjustmentQuote{{Deal: "Buy 2 get 1", ChargedBackCents: 5000}},
		},
		{
			name:  "paid unit for D on the deal keeps it",
			lines: []exchangeLine{line("A", -1), line("D", 1), line("SOCKS", 1)},
			want:  []wantLine{{total: -10000, paid: 10000}, {total: 9000}, {total: 2000}},
			total: 1000,
		},
		{
			// Lines sent in any order come back in that order.
			name:  "replacement first",
			lines: []exchangeLine{line("JACKET", 1), line("A", -1)},
			want:  []wantLine{{total: 10000}, {total: -5000, paid: 10000, back: 5000}},
			total: 5000,
		},
		{
			// Everything back for two jackets: $180 paid, $200 taken. C's
			// credit buys $20 of jacket, but with no deal left that $20 isn't
			// earned, so it comes back off A: the customer pays $20.
			name:  "everything back for jackets",
			lines: []exchangeLine{line("A", -1), line("B", -1), line("C", -1), line("JACKET", 2)},
			want:  []wantLine{{total: -8000, paid: 10000, back: 2000}, {total: -8000, paid: 8000}, {total: -2000, swap: 2000, free: true}, {total: 20000}},
			total: 2000,
			adj:   []ExchangeAdjustmentQuote{{Deal: "Buy 2 get 1", ChargedBackCents: 2000, ForfeitedCents: 3000}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shop := setUpDealShop(t)
			sale := shop.sell(t, "A", "B", "C")
			quote := quoteExchangeLines(t, shop, sale.InvoiceID, tc.lines)
			checkLines(t, quote, tc.want...)
			if quote.GrandTotalCents != tc.total {
				t.Fatalf("total %d, want %d", quote.GrandTotalCents, tc.total)
			}
			if tc.adj != nil && fmt.Sprint(quote.Adjustments) != fmt.Sprint(tc.adj) {
				t.Fatalf("adjustments %+v, want %+v", quote.Adjustments, tc.adj)
			}
			checkoutQuoted(t, shop, sale.InvoiceID, tc.lines, quote)
		})
	}
}

// The case from the shop floor: a paid unit of a deal returned for an
// unknown item at a typed price. The unknown item is never on the deal, so
// the deal is charged back off the returned line; the line shows its credit
// after that, and the lines add up to the total on screen.
func TestExchangeQuoteLinesUnknownItemForAPaidDealUnit(t *testing.T) {
	shop := setUpDealShop(t)
	unknown, err := ensureUnknownProduct(shop.app.ctx, shop.store.Queries, shop.app.unknownBarcode())
	if err != nil {
		t.Fatal(err)
	}
	sale := shop.sell(t, "A", "B", "C")
	lines := []exchangeLine{
		{Code: unknown.Code, Barcode: unknown.Barcode, Qty: 1, UnitPricesCents: []int64{12000}},
		line("B", -1),
	}
	quote := quoteExchangeLines(t, shop, sale.InvoiceID, lines)
	// B paid $80; C's $50 comes back off it: -$30 against $120.
	checkLines(t, quote, wantLine{total: 12000}, wantLine{total: -3000, paid: 8000, back: 5000})
	if quote.GrandTotalCents != 9000 {
		t.Fatalf("total %d, want 9000", quote.GrandTotalCents)
	}
	checkoutQuoted(t, shop, sale.InvoiceID, lines, quote)
}

// The shop-floor case: everything paid back from a deal sale whose free unit
// the customer keeps. The kept unit is no longer free, so its saving comes
// off the credit, and the screen asks for exactly what its lines are short
// of -- not for what was paid.
func TestExchangeQuoteShortfallIsTheLinesShortfall(t *testing.T) {
	shop := setUpDealShop(t)
	sale := shop.sell(t, "A", "B", "C")
	// A $100 and B $80 back, C (free, $50) kept: $180 paid, $50 charged
	// back, a $130 credit.
	returnOnly := []exchangeLine{line("A", -1), line("B", -1)}
	quote := quoteExchangeLines(t, shop, sale.InvoiceID, returnOnly)
	if !quote.NeedsMoreReplacementValue || quote.ShortfallCents != 13000 || quote.NetCents != -13000 {
		t.Fatalf("asks for %d more with the lines at %d, want 13000 and -13000", quote.ShortfallCents, quote.NetCents)
	}
	// $120 of jackets and socks: $10 short.
	short := append(append([]exchangeLine{}, returnOnly...), line("JACKET", 1), line("SOCKS", 1))
	if q := quoteExchangeLines(t, shop, sale.InvoiceID, short); q.ShortfallCents != 1000 {
		t.Fatalf("asks for %d more, want 1000", q.ShortfallCents)
	}
	two := append(append([]exchangeLine{}, returnOnly...), line("JACKET", 2))
	q := quoteExchangeLines(t, shop, sale.InvoiceID, two)
	// $200 of jackets against the $130 credit: $70 to pay. The customer has
	// paid $180 + $70 = $250 for C ($50) and two jackets ($200).
	if q.NeedsMoreReplacementValue || q.GrandTotalCents != 7000 {
		t.Fatalf("needs more %v, total %d, want false and 7000", q.NeedsMoreReplacementValue, q.GrandTotalCents)
	}
	checkoutQuoted(t, shop, sale.InvoiceID, two, q)
}

// A free unit returned with nothing bought shows what it could take off new
// items, so the screen can say so rather than "no credit".
func TestExchangeQuoteFreeUnitAllowance(t *testing.T) {
	shop := setUpDealShop(t)
	sale := shop.sell(t, "A", "B", "C")
	q := quoteExchangeLines(t, shop, sale.InvoiceID, []exchangeLine{line("C", -1)})
	if l := q.Lines[0]; !l.Free || l.SwapAllowanceCents != 5000 || l.SwapCreditCents != 0 || l.LineTotalCents != 0 {
		t.Fatalf("free C = %+v", l)
	}
	q = quoteExchangeLines(t, shop, sale.InvoiceID, []exchangeLine{line("C", -1), line("SOCKS", 1)})
	if l := q.Lines[0]; l.SwapAllowanceCents != 5000 || l.SwapCreditCents != 2000 {
		t.Fatalf("free C for socks = %+v", l)
	}
}

// Alike units are one returned item however the cart sends them; each sent
// line gets its own units' amounts, and they still add up.
func TestExchangeQuoteLinesMergedAndSplitLines(t *testing.T) {
	// A, C, C on the deal: one C free, the other paid.
	for _, tc := range []struct {
		name  string
		lines []exchangeLine
		want  []wantLine
	}{
		{
			name:  "both Cs on one line",
			lines: []exchangeLine{line("C", -2), line("JACKET", 1)},
			// $150 paid for A, C, C; A and a jacket kept are $200. The free
			// C's $50 swaps, and its saving is charged back (no set left):
			// a $50 credit, $50 to pay.
			want: []wantLine{{total: -5000, paid: 5000, swap: 5000, back: 5000}, {total: 10000}},
		},
		{
			name:  "the Cs on two lines",
			lines: []exchangeLine{line("C", -1), line("C", -1), line("JACKET", 1)},
			// The free unit is taken first; the charge back comes off the
			// paid one.
			want: []wantLine{{total: -5000, swap: 5000, free: true}, {total: 0, paid: 5000, back: 5000}, {total: 10000}},
		},
		{
			name:  "a jacket on two lines",
			lines: []exchangeLine{line("C", -1), line("JACKET", 1), line("JACKET", 1)},
			want:  []wantLine{{total: -5000, swap: 5000, free: true}, {total: 10000}, {total: 10000}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shop := setUpDealShop(t)
			sale := shop.sell(t, "A", "C", "C")
			quote := quoteExchangeLines(t, shop, sale.InvoiceID, tc.lines)
			checkLines(t, quote, tc.want...)
			checkoutQuoted(t, shop, sale.InvoiceID, tc.lines, quote)
		})
	}
}

// With a fallback ("20% off outside a set"), a unit returned out of a set
// leaves the others outside one: they get the fallback, and the free unit's
// saving above it is charged back.
func TestExchangeQuoteLinesWithFallback(t *testing.T) {
	shop := setUpDealShopWithFallback(t, 20)
	sale := shop.sell(t, "A", "B", "C")
	lines := []exchangeLine{line("A", -1), line("JACKET", 1)}
	quote := quoteExchangeLines(t, shop, sale.InvoiceID, lines)
	l := quote.Lines[0]
	if l.PaidCents != 10000 || l.ChargedBackCents <= 0 || -l.LineTotalCents != 10000-l.ChargedBackCents {
		t.Fatalf("returned A = %+v", l)
	}
	checkoutQuoted(t, shop, sale.InvoiceID, lines, quote)
}

// A second exchange of a sale reads what the first did to its deal.
func TestExchangeQuoteLinesSecondExchange(t *testing.T) {
	shop := setUpDealShop(t)
	sale := shop.sell(t, "A", "B", "C")
	first := []exchangeLine{line("C", -1), line("D", 1)}
	checkoutQuoted(t, shop, sale.InvoiceID, first, quoteExchangeLines(t, shop, sale.InvoiceID, first))
	// B's $80 against a $100 jacket, and C's $50 saving charged back now.
	second := []exchangeLine{line("B", -1), line("JACKET", 1)}
	quote := quoteExchangeLines(t, shop, sale.InvoiceID, second)
	checkLines(t, quote, wantLine{total: -3000, paid: 8000, back: 5000}, wantLine{total: 10000})
	if quote.GrandTotalCents != 7000 {
		t.Fatalf("total %d, want 7000", quote.GrandTotalCents)
	}
	checkoutQuoted(t, shop, sale.InvoiceID, second, quote)
}

// Tax and cash rounding are their own steps of the breakdown.
func TestExchangeQuoteTaxAndRounding(t *testing.T) {
	shop := setUpDealShop(t)
	sale := shop.sell(t, "SOCKS")
	setExchangeTaxAndRounding(t, shop, 0.11, "LBP")
	lines := []exchangeLine{line("SOCKS", -1), line("A", 1)}
	quote := quoteExchangeLines(t, shop, sale.InvoiceID, lines)
	// $80 net, 11% tax: $8.80; $88.80 rounds to LL 5,000s at 89,500.
	if quote.NetCents != 8000 || quote.TaxCents != 880 {
		t.Fatalf("net %d tax %d, want 8000 880", quote.NetCents, quote.TaxCents)
	}
	if quote.TaxCents == 0 {
		t.Fatalf("no tax on %+v", quote)
	}
	if quote.RoundingCents == 0 {
		t.Fatalf("$88.80 should round in LBP: %+v", quote)
	}
	checkoutQuoted(t, shop, sale.InvoiceID, lines, quote)
}

func setExchangeTaxAndRounding(t *testing.T, shop dealShop, tax float64, currency string) {
	t.Helper()
	if _, err := shop.app.conn.Exec(`UPDATE currencies SET units_per_usd_micros = 89500000000, cash_rounding_unit = 5000 WHERE code = 'LBP'`); err != nil {
		t.Fatal(err)
	}
	if _, err := shop.app.conn.Exec(`UPDATE settings SET tax_rate = ?, rounding_currency = ?`, tax, currency); err != nil {
		t.Fatal(err)
	}
}

// Every exchange the till can be asked for: random sales, random returns
// (merged or line by line), random replacements (deal, off-deal, unknown at
// a typed price), with and without a fallback, tax and cash rounding, and
// second exchanges of the same sale. Each quote must add up, a quote that
// asks for no more must check out paying exactly its total, the stored
// exchange must be the quote, and a quote that fails or asks for more
// must not check out.
func TestExchangeQuoteAddsUpForEveryExchange(t *testing.T) {
	codes := []string{"A", "B", "C", "D", "JACKET", "SOCKS"}
	iterations := 400
	if testing.Short() {
		iterations = 60
	}
	for _, fallback := range []int64{0, 20, 30} {
		shop := setUpDealShopWithFallback(t, fallback)
		unknown, err := ensureUnknownProduct(shop.app.ctx, shop.store.Queries, shop.app.unknownBarcode())
		if err != nil {
			t.Fatal(err)
		}
		rng := rand.New(rand.NewSource(fallback + 1))
		checkedOut := 0
		for it := 0; it < iterations; it++ {
			tax := []float64{0, 0, 0.11}[rng.Intn(3)]
			currency := []string{"USD", "USD", "LBP"}[rng.Intn(3)]
			setExchangeTaxAndRounding(t, shop, 0, "USD")
			var sold []string
			for range 1 + rng.Intn(6) {
				sold = append(sold, codes[rng.Intn(len(codes))])
			}
			sort.Strings(sold)
			sale := shop.sell(t, sold...)
			setExchangeTaxAndRounding(t, shop, tax, currency)

			for round := 0; round < 1+rng.Intn(2); round++ {
				lines := randomExchange(rng, sold, codes, unknown.Code, unknown.Barcode)
				if len(lines) == 0 {
					break
				}
				desc := fmt.Sprintf("fallback %d, tax %v %s, sale %v, round %d, lines %+v", fallback, tax, currency, sold, round, lines)
				raw, _ := json.Marshal(lines)
				quote, err := shop.app.QuoteExchangeTotal(string(raw), sale.InvoiceID)
				if err != nil {
					// What can't be quoted can't be checked out either.
					if _, cerr := shop.app.checkoutReturnExchangeInvoice(string(raw), "[]", sale.InvoiceID, shop.salesperson.ID, "", 0, posExchangePolicy); cerr == nil {
						t.Fatalf("%s: quote failed (%v) but checkout went through", desc, err)
					}
					continue
				}
				if msg := quoteInconsistency(quote, lines); msg != "" {
					t.Fatalf("%s: %s\n%+v", desc, msg, quote)
				}
				hasReplacement := false
				for _, l := range lines {
					hasReplacement = hasReplacement || l.Qty > 0
				}
				if !hasReplacement {
					// A return with nothing bought is a refund (kashi or the
					// admin), never an exchange at the till.
					if _, cerr := shop.app.checkoutReturnExchangeInvoice(string(raw), "[]", sale.InvoiceID, shop.salesperson.ID, "", 0, posExchangePolicy); cerr == nil {
						t.Fatalf("%s: an exchange with nothing bought went through", desc)
					}
					continue
				}
				if quote.NeedsMoreReplacementValue {
					p, _ := json.Marshal([]checkoutPayment{{AccountID: shop.cashbox.ID, AmountCents: max(1, quote.GrandTotalCents)}})
					if _, cerr := shop.app.checkoutReturnExchangeInvoice(string(raw), string(p), sale.InvoiceID, shop.salesperson.ID, "", 0, posExchangePolicy); cerr == nil {
						t.Fatalf("%s: quote asks for %d more but checkout went through", desc, quote.ShortfallCents)
					}
					continue
				}
				if msg := checkoutMismatch(shop, sale.InvoiceID, lines, quote); msg != "" {
					t.Fatalf("%s: %s\n%+v", desc, msg, quote)
				}
				checkedOut++
				sold = removeReturned(sold, lines)
			}
		}
		if checkedOut < iterations/4 {
			t.Fatalf("fallback %d: only %d of %d exchanges checked out; the generator is too strict to test much", fallback, checkedOut, iterations)
		}
	}
}

// randomExchange returns some of what is left of a sale, line by line or
// merged, and buys some replacements, sometimes an unknown item.
func randomExchange(rng *rand.Rand, sold, codes []string, unknownCode, unknownBarcode string) []exchangeLine {
	if len(sold) == 0 {
		return nil
	}
	perm := rng.Perm(len(sold))
	n := 1 + rng.Intn(len(sold))
	var lines []exchangeLine
	merged := rng.Intn(2) == 0
	for _, i := range perm[:n] {
		code := sold[i]
		if merged {
			found := false
			for j := range lines {
				if lines[j].Code == code {
					lines[j].Qty--
					found = true
				}
			}
			if found {
				continue
			}
		}
		lines = append(lines, line(code, -1))
	}
	for range rng.Intn(4) {
		if rng.Intn(5) == 0 {
			price := int64(100 + rng.Intn(20000))
			lines = append(lines, exchangeLine{Code: unknownCode, Barcode: unknownBarcode, Qty: 1, UnitPricesCents: []int64{price}})
			continue
		}
		lines = append(lines, line(codes[rng.Intn(len(codes))], int64(1+rng.Intn(2))))
	}
	rng.Shuffle(len(lines), func(i, j int) { lines[i], lines[j] = lines[j], lines[i] })
	// Unknown items in a cart are lines of their own; two typed-price lines
	// of the one product can't be sent, so keep the first.
	seenUnknown := false
	out := lines[:0]
	for _, l := range lines {
		if l.Barcode == unknownBarcode {
			if seenUnknown {
				continue
			}
			seenUnknown = true
		}
		out = append(out, l)
	}
	return out
}

func removeReturned(sold []string, lines []exchangeLine) []string {
	left := append([]string(nil), sold...)
	for _, l := range lines {
		for range -l.Qty {
			for i, code := range left {
				if code == l.Code {
					left = append(left[:i], left[i+1:]...)
					break
				}
			}
		}
	}
	return left
}
