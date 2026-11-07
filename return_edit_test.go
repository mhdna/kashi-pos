package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	db "kashi-pos/db/sqlc"
)

// returnState is everything an edit touches on one return, to compare
// before and after.
type returnState struct {
	id, saleID, grandTotal int64
	notes                  string
	lines                  []db.InvoiceProduct
	payments               map[int64]int64
	entries                int64 // sum of payment entries
	balance                int64 // cashbox balance of the shift, the refund account
	lbp                    int64
	clawback               int64
	outbox                 int
}

func lastReturnID(t *testing.T, shop dealShop) int64 {
	t.Helper()
	var id int64
	if err := shop.app.conn.QueryRow(`SELECT max(invoice_id) FROM return_invoices`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func readReturn(t *testing.T, shop dealShop, id int64) returnState {
	t.Helper()
	ctx := context.Background()
	q := shop.store.Queries
	inv, err := q.GetInvoiceByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	saleID, err := q.GetReturnSaleID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	lines, err := q.ListInvoiceProductsInOrder(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	st := returnState{id: id, saleID: saleID, grandTotal: inv.GrandTotal, notes: inv.Notes, lines: lines, payments: map[int64]int64{}}
	pays, _ := q.ListInvoicePayments(ctx, id)
	for _, p := range pays {
		st.payments[p.CashboxID] += p.Amount
	}
	entries, _ := q.ListInvoicePaymentEntries(ctx, id)
	for _, e := range entries {
		st.entries += e.Amount
	}
	if bal, err := q.GetShiftAccountBalance(ctx, db.GetShiftAccountBalanceParams{AccountID: shop.cashbox.ID, ShiftID: inv.ShiftID}); err == nil {
		st.balance = bal.Balance
	}
	shop.app.conn.QueryRow(`SELECT COALESCE(lbp_total, 0) FROM invoices WHERE id = ?`, id).Scan(&st.lbp)
	adjs, _ := q.ListExchangeDealAdjustments(ctx, id)
	for _, a := range adjs {
		st.clawback += a.Clawback
	}
	shop.app.conn.QueryRow(`SELECT count(*) FROM sync_outbox WHERE entity_type = 'return_invoice_edit'`).Scan(&st.outbox)
	return st
}

func returnClientRef(t *testing.T, shop dealShop, id int64) string {
	t.Helper()
	ref, err := getOrCreateInvoiceClientRef(context.Background(), shop.store.Queries, id)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

// The case from the shop floor (RS/CT/2026/00068): a deal charged back part
// of a paid unit's credit on a cash refund; kashi gives the customer the
// rest. Sale A $100, B $80, C $50 on Buy 2 get 1 (C free). Refunding A
// leaves C without its pair: A is credited $50, $50 charged back. The edit
// makes it the full $100.
func TestEditReturnGivesTheFullCredit(t *testing.T) {
	shop := setUpDealShop(t)
	sale := shop.sell(t, "A", "B", "C")
	if got := shop.refund(t, sale.InvoiceID, "A"); got != -5000 {
		t.Fatalf("refund %d, want -5000", got)
	}
	id := lastReturnID(t, shop)
	before := readReturn(t, shop, id)
	if before.lines[0].LineTotal != -5000 || before.clawback != 5000 {
		t.Fatalf("before: line %d clawback %d", before.lines[0].LineTotal, before.clawback)
	}

	total, err := shop.app.editReturnInvoice(context.Background(), editReturnInvoicePayload{
		ClientRef: returnClientRef(t, shop, id),
		Lines:     []editReturnLine{{Position: 0, Barcode: "bc-A", LineTotal: -10000}},
		Reason:    "Customer keeps the deal",
		EditedBy:  "Mahdi",
	})
	if err != nil {
		t.Fatal(err)
	}
	after := readReturn(t, shop, id)
	switch {
	case total != -10000 || after.grandTotal != -10000:
		t.Fatalf("total %d / %d, want -10000", total, after.grandTotal)
	case after.lines[0].LineTotal != -10000:
		t.Fatalf("line %d, want -10000", after.lines[0].LineTotal)
	case after.payments[shop.cashbox.ID] != -10000 || after.entries != -10000:
		t.Fatalf("payments %v entries %d, want -10000", after.payments, after.entries)
	case after.balance-before.balance != -5000:
		t.Fatalf("drawer moved %d, want -5000", after.balance-before.balance)
	case after.lbp == before.lbp || after.lbp >= 0:
		t.Fatalf("LBP total %d (was %d) not updated", after.lbp, before.lbp)
	case after.clawback != 0:
		t.Fatalf("clawback left %d, want 0: a later exchange would charge it again", after.clawback)
	case !strings.Contains(after.notes, "Item A credit $50.00 -> $100.00") || !strings.Contains(after.notes, "Customer keeps the deal") || !strings.Contains(after.notes, "Mahdi"):
		t.Fatalf("notes %q", after.notes)
	case after.outbox != before.outbox+1:
		t.Fatalf("%d edits queued for kashi, want 1", after.outbox-before.outbox)
	}

	// What kashi is sent.
	var raw string
	if err := shop.app.conn.QueryRow(`SELECT payload_json FROM sync_outbox WHERE entity_type = 'return_invoice_edit' ORDER BY id DESC LIMIT 1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var sent branchReturnEditPayload
	if err := json.Unmarshal([]byte(raw), &sent); err != nil {
		t.Fatal(err)
	}
	if sent.ClientRef != returnClientRef(t, shop, id) || sent.GrandTotal != -10000 || len(sent.Lines) != 1 ||
		sent.Lines[0] != (branchReturnEditLine{Position: 0, LineTotal: -10000}) || sent.EditRef == "" {
		t.Fatalf("sent %+v", sent)
	}
	var paid int64
	for _, p := range sent.Payments {
		paid += p.Amount
	}
	if paid != -10000 {
		t.Fatalf("payments sent add up to %d, want -10000", paid)
	}

	// Replaying the same edit (a command retried) changes nothing.
	if _, err := shop.app.editReturnInvoice(context.Background(), editReturnInvoicePayload{
		ClientRef: returnClientRef(t, shop, id),
		Lines:     []editReturnLine{{Position: 0, Barcode: "bc-A", LineTotal: -10000}},
	}); err != nil {
		t.Fatal(err)
	}
	again := readReturn(t, shop, id)
	if fmt.Sprint(again) != fmt.Sprint(after) {
		t.Fatalf("a replay changed the return:\n%+v\n%+v", after, again)
	}
}

// What kashi's Edit refund sends goes through the command channel.
func TestEditReturnCommand(t *testing.T) {
	shop := setUpDealShop(t)
	sale := shop.sell(t, "A", "B", "C")
	shop.refund(t, sale.InvoiceID, "A")
	id := lastReturnID(t, shop)
	payload, _ := json.Marshal(editReturnInvoicePayload{
		ClientRef: returnClientRef(t, shop, id),
		Lines:     []editReturnLine{{Position: 0, Barcode: "bc-A", LineTotal: -10000}},
	})
	if err := shop.app.executeCommand(context.Background(), branchCommand{Type: "edit_return_invoice", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if got := readReturn(t, shop, id).grandTotal; got != -10000 {
		t.Fatalf("total %d, want -10000", got)
	}
}

// An exchange's returned line can be edited too; the new items stay as they
// are, and the customer's payment goes down by what the edit gives back.
func TestEditExchangeReturnedLine(t *testing.T) {
	shop := setUpDealShop(t)
	sale := shop.sell(t, "A", "B", "C")
	if pays := shop.exchange(t, sale.InvoiceID, []string{"A"}, []string{"JACKET"}); pays != 5000 {
		t.Fatalf("exchange pays %d, want 5000", pays)
	}
	id := lastReturnID(t, shop)
	before := readReturn(t, shop, id)
	if _, err := shop.app.editReturnInvoice(context.Background(), editReturnInvoicePayload{
		ClientRef: returnClientRef(t, shop, id),
		Lines:     []editReturnLine{{Position: 0, Barcode: "bc-A", LineTotal: -10000}},
	}); err != nil {
		t.Fatal(err)
	}
	after := readReturn(t, shop, id)
	if after.grandTotal != 0 || after.lines[1].LineTotal != before.lines[1].LineTotal {
		t.Fatalf("exchange total %d, jacket %d -> %d", after.grandTotal, before.lines[1].LineTotal, after.lines[1].LineTotal)
	}
	var paid int64
	for _, a := range after.payments {
		paid += a
	}
	if paid != 0 || after.balance-before.balance != -5000 {
		t.Fatalf("payments %v, drawer moved %d", after.payments, after.balance-before.balance)
	}
}

func TestEditReturnRefuses(t *testing.T) {
	shop := setUpDealShop(t)
	sale := shop.sell(t, "A", "B", "C")
	shop.exchange(t, sale.InvoiceID, []string{"A"}, []string{"JACKET"})
	id := lastReturnID(t, shop)
	ref := returnClientRef(t, shop, id)
	saleRef := returnClientRef(t, shop, sale.InvoiceID)
	cases := []struct {
		name string
		p    editReturnInvoicePayload
		want string
	}{
		{"a line past the end", editReturnInvoicePayload{ClientRef: ref, Lines: []editReturnLine{{Position: 5, LineTotal: -100}}}, "not on this return"},
		{"a negative position", editReturnInvoicePayload{ClientRef: ref, Lines: []editReturnLine{{Position: -1, LineTotal: -100}}}, "not on this return"},
		{"another product", editReturnInvoicePayload{ClientRef: ref, Lines: []editReturnLine{{Position: 0, Barcode: "bc-B", LineTotal: -10000}}}, "reload the invoice"},
		{"a new item", editReturnInvoicePayload{ClientRef: ref, Lines: []editReturnLine{{Position: 1, LineTotal: 5000}}}, "isn't a returned item"},
		{"a charge", editReturnInvoicePayload{ClientRef: ref, Lines: []editReturnLine{{Position: 0, LineTotal: 100}}}, "can't be a charge"},
		{"more than its price", editReturnInvoicePayload{ClientRef: ref, Lines: []editReturnLine{{Position: 0, LineTotal: -10001}}}, "more than its price, $100.00"},
		{"the same line twice", editReturnInvoicePayload{ClientRef: ref, Lines: []editReturnLine{{Position: 0, LineTotal: -6000}, {Position: 0, LineTotal: -7000}}}, "edited twice"},
		{"a sale", editReturnInvoicePayload{ClientRef: saleRef, Lines: []editReturnLine{{Position: 0, LineTotal: -100}}}, "only a return or an exchange"},
		{"nothing", editReturnInvoicePayload{ClientRef: ref}, "nothing to change"},
		{"no invoice", editReturnInvoicePayload{ClientRef: "nope", Lines: []editReturnLine{{Position: 0, LineTotal: -100}}}, "invoice not found"},
		{"no client ref", editReturnInvoicePayload{Lines: []editReturnLine{{Position: 0, LineTotal: -100}}}, "client_ref is required"},
	}
	before := readReturn(t, shop, id)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := shop.app.editReturnInvoice(context.Background(), tc.p)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v, want %q", err, tc.want)
			}
			if after := readReturn(t, shop, id); fmt.Sprint(after) != fmt.Sprint(before) {
				t.Fatalf("a refused edit changed the return:\n%+v\n%+v", before, after)
			}
		})
	}
	// A bad line among good ones changes nothing either.
	_, err := shop.app.editReturnInvoice(context.Background(), editReturnInvoicePayload{ClientRef: ref, Lines: []editReturnLine{
		{Position: 0, LineTotal: -9000}, {Position: 1, LineTotal: 1},
	}})
	if err == nil {
		t.Fatal("a bad line was accepted")
	}
	if after := readReturn(t, shop, id); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatal("half an edit was applied")
	}
}

// Any edit of any return keeps the books straight: the total moves by what
// the lines moved, the payments still add up to the total and so do their
// entries, the drawer moves by the same, and each edit is sent to kashi once.
func TestEditReturnKeepsTheBooksStraight(t *testing.T) {
	codes := []string{"A", "B", "C", "D", "JACKET", "SOCKS"}
	shop := setUpDealShop(t)
	rng := rand.New(rand.NewSource(7))
	edits := 0
	for it := 0; it < 120; it++ {
		var sold []string
		for range 2 + rng.Intn(4) {
			sold = append(sold, codes[rng.Intn(len(codes))])
		}
		sale := shop.sell(t, sold...)
		// A refund of one or two items, or an exchange for a jacket.
		returned := []string{sold[0]}
		if len(sold) > 2 && rng.Intn(2) == 0 {
			returned = append(returned, sold[1])
		}
		if rng.Intn(2) == 0 {
			shop.refund(t, sale.InvoiceID, returned...)
		} else if _, err := shop.tryExchange(sale.InvoiceID, returned, []string{"JACKET", "JACKET"}); err != nil {
			continue
		}
		id := lastReturnID(t, shop)
		before := readReturn(t, shop, id)
		var lines []editReturnLine
		var moved int64
		for pos, l := range before.lines {
			if l.Quantity >= 0 || rng.Intn(3) == 0 {
				continue
			}
			shelf := l.UnitPrice * l.Quantity
			newTotal := shelf + rng.Int63n(-shelf+1) // between the shelf price and 0
			lines = append(lines, editReturnLine{Position: pos, LineTotal: newTotal})
			moved += newTotal - l.LineTotal
		}
		if len(lines) == 0 {
			continue
		}
		if _, err := shop.app.editReturnInvoice(context.Background(), editReturnInvoicePayload{ClientRef: returnClientRef(t, shop, id), Lines: lines}); err != nil {
			t.Fatalf("sale %v, returned %v, lines %+v: %v", sold, returned, lines, err)
		}
		after := readReturn(t, shop, id)
		var paid int64
		for _, a := range after.payments {
			paid += a
		}
		var beforePaid int64
		for _, a := range before.payments {
			beforePaid += a
		}
		want := 0
		if moved != 0 {
			want = 1
		}
		switch {
		case after.grandTotal-before.grandTotal != moved:
			t.Fatalf("total moved %d, lines %d", after.grandTotal-before.grandTotal, moved)
		case paid-beforePaid != moved || after.entries != paid:
			t.Fatalf("payments moved %d, entries %d of %d, lines %d", paid-beforePaid, after.entries, paid, moved)
		case after.balance-before.balance != moved:
			t.Fatalf("drawer moved %d, lines %d", after.balance-before.balance, moved)
		case after.outbox-before.outbox != want:
			t.Fatalf("%d edits sent to kashi, want %d", after.outbox-before.outbox, want)
		case after.clawback < 0 || after.clawback > before.clawback:
			t.Fatalf("clawback %d -> %d", before.clawback, after.clawback)
		}
		for i, l := range after.lines {
			if l.Quantity > 0 && l.LineTotal != before.lines[i].LineTotal {
				t.Fatalf("a new item changed: %d -> %d", before.lines[i].LineTotal, l.LineTotal)
			}
		}
		edits++
	}
	if edits < 40 {
		t.Fatalf("only %d edits ran", edits)
	}
}
