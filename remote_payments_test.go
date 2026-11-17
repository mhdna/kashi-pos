package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	db "kashi-pos/db/sqlc"
)

// remotePaymentsFixture: the on-account fixture plus a Card method. sourceID
// is a $20 cash sale (2 x $10, walk-in); saleID a $20 sale On Account (Maya).
type remotePaymentsFixture struct {
	onAccountReturnFixture
	card db.CashboxAccount
}

func newRemotePaymentsFixture(t *testing.T) remotePaymentsFixture {
	t.Helper()
	f := newOnAccountReturnFixture(t)
	card, err := f.store.CreateCashboxAccount(context.Background(), db.CreateCashboxAccountParams{Name: "Card", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	return remotePaymentsFixture{onAccountReturnFixture: f, card: card}
}

func (f remotePaymentsFixture) ref(t *testing.T, invoiceID int64) string {
	t.Helper()
	ref, err := f.store.GetInvoiceClientRef(context.Background(), invoiceID)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func (f remotePaymentsFixture) run(t *testing.T, kind string, payload any) error {
	t.Helper()
	raw, _ := json.Marshal(payload)
	return f.app.executeCommand(context.Background(), branchCommand{Type: kind, Payload: raw})
}

func (f remotePaymentsFixture) settlement(t *testing.T, invoiceID int64) map[string]int64 {
	t.Helper()
	rows, err := f.database.Query(`SELECT ca.name, ip.amount FROM invoice_payments ip JOIN cashbox_accounts ca ON ca.id = ip.cashbox_id WHERE ip.invoice_id = ?`, invoiceID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]int64{}
	for rows.Next() {
		var name string
		var amount int64
		if err := rows.Scan(&name, &amount); err != nil {
			t.Fatal(err)
		}
		got[name] = amount
	}
	return got
}

func (f remotePaymentsFixture) lastInvoiceID(t *testing.T) int64 {
	t.Helper()
	var id int64
	if err := f.database.QueryRow(`SELECT MAX(id) FROM invoices`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (f remotePaymentsFixture) returnPayload(t *testing.T, qty int64, payments []remotePayment) remoteReturnInvoiceCommandPayload {
	return remoteReturnInvoiceCommandPayload{
		ClientRef: f.clientRef,
		Items:     []remoteReturnInvoiceItemPayload{{BranchProductID: f.product.ID, Quantity: qty}},
		Payments:  payments,
	}
}

func TestRemoteReturnRefundSplitBetweenMethods(t *testing.T) {
	f := newRemotePaymentsFixture(t)
	cash, card := f.shiftBalance(t, f.cashbox.ID), f.shiftBalance(t, f.card.ID)
	err := f.run(t, "remote_return_invoice", f.returnPayload(t, 2, []remotePayment{
		{AccountName: "cash", AmountCents: -1500}, // names match case-insensitively
		{AccountName: "Card", AmountCents: -500},
	}))
	if err != nil {
		t.Fatal(err)
	}
	got := f.settlement(t, f.lastInvoiceID(t))
	if got["Cash"] != -1500 || got["Card"] != -500 || len(got) != 2 {
		t.Fatalf("return settlement = %v", got)
	}
	if f.shiftBalance(t, f.cashbox.ID)-cash != -1500 || f.shiftBalance(t, f.card.ID)-card != -500 {
		t.Fatal("shift balances did not move by the split")
	}
}

func TestRemoteReturnPaymentsMustMatchTheTillsTotal(t *testing.T) {
	f := newRemotePaymentsFixture(t)
	before := f.lastInvoiceID(t)
	err := f.run(t, "remote_return_invoice", f.returnPayload(t, 1, []remotePayment{{AccountName: "Cash", AmountCents: -1500}}))
	if err == nil || !strings.Contains(err.Error(), "-$10.00") || !strings.Contains(err.Error(), "-$15.00") {
		t.Fatalf("err = %v, want the till's -$10.00 and the entered -$15.00", err)
	}
	if f.lastInvoiceID(t) != before {
		t.Fatal("a refused return must record nothing")
	}
	for _, bad := range [][]remotePayment{
		{{AccountName: "Wire", AmountCents: -1000}},
		{{AccountName: "Cash", AmountCents: 0}},
	} {
		if err := f.run(t, "remote_return_invoice", f.returnPayload(t, 1, bad)); err == nil {
			t.Fatalf("payments %v should be refused", bad)
		}
	}
	// No payments: the till settles it as before.
	if err := f.run(t, "remote_return_invoice", f.returnPayload(t, 1, nil)); err != nil {
		t.Fatal(err)
	}
}

// Swapping one $10 unit for a $15 one: the customer pays $5, split as entered.
func TestRemoteExchangeDifferenceSplitBetweenMethods(t *testing.T) {
	f := newRemotePaymentsFixture(t)
	ctx := context.Background()
	dearer, err := f.store.CreateProduct(ctx, db.CreateProductParams{Barcode: "777", Code: "DEARER", Name: "Dearer", IsActive: 1, Price: 1500})
	if err != nil {
		t.Fatal(err)
	}
	priceProductsFromRows(t, f.store.Queries)
	dearer, _ = f.store.GetProductByBarcode(ctx, "777")
	payload := f.returnPayload(t, 1, []remotePayment{{AccountName: "Cash", AmountCents: 300}, {AccountName: "Card", AmountCents: 200}})
	payload.Replacements = []remoteReplacementPayload{{ProductUID: dearer.Uid.String, Barcode: "777", Quantity: 1}}
	if err := f.run(t, "remote_return_invoice", payload); err != nil {
		t.Fatal(err)
	}
	got := f.settlement(t, f.lastInvoiceID(t))
	if got["Cash"] != 300 || got["Card"] != 200 {
		t.Fatalf("exchange settlement = %v", got)
	}
}

func TestChangeInvoiceSettlementFromCentral(t *testing.T) {
	f := newRemotePaymentsFixture(t)
	cash, card := f.shiftBalance(t, f.cashbox.ID), f.shiftBalance(t, f.card.ID)
	change := func(payments ...remotePayment) error {
		return f.run(t, "change_invoice_settlement", changeInvoiceSettlementPayload{ClientRef: f.clientRef, Payments: payments})
	}
	if err := change(remotePayment{AccountName: "Cash", AmountCents: 1200}, remotePayment{AccountName: "Card", AmountCents: 800}); err != nil {
		t.Fatal(err)
	}
	if got := f.settlement(t, f.sourceID); got["Cash"] != 1200 || got["Card"] != 800 || len(got) != 2 {
		t.Fatalf("settlement = %v", got)
	}
	if f.shiftBalance(t, f.cashbox.ID)-cash != -800 || f.shiftBalance(t, f.card.ID)-card != 800 {
		t.Fatal("shift balances: cash should drop 800 and card gain it")
	}
	var queued int
	if err := f.database.QueryRow(`SELECT COUNT(*) FROM sync_outbox WHERE entity_type = 'invoice_settlement'`).Scan(&queued); err != nil || queued == 0 {
		t.Fatalf("the change should be queued for central (count %d, err %v)", queued, err)
	}
	// Doing it again with the same split changes nothing more.
	if err := change(remotePayment{AccountName: "Cash", AmountCents: 1200}, remotePayment{AccountName: "Card", AmountCents: 800}); err != nil {
		t.Fatal(err)
	}
	if f.shiftBalance(t, f.cashbox.ID)-cash != -800 {
		t.Fatal("a repeat moved the cash again")
	}
	if err := change(remotePayment{AccountName: "Cash", AmountCents: 1900}); err == nil || !strings.Contains(err.Error(), "$20.00") {
		t.Fatalf("a split under the total: err = %v", err)
	}
	if err := change(); err == nil {
		t.Fatal("no payments should be refused")
	}
	// A return isn't a sale.
	if err := f.run(t, "remote_return_invoice", f.returnPayload(t, 1, nil)); err != nil {
		t.Fatal(err)
	}
	if err := f.run(t, "change_invoice_settlement", changeInvoiceSettlementPayload{
		ClientRef: f.ref(t, f.lastInvoiceID(t)), Payments: []remotePayment{{AccountName: "Cash", AmountCents: -1000}},
	}); err == nil {
		t.Fatal("a return's settlement isn't changed this way")
	}
}

func TestChangeSettlementOnAndOffAccount(t *testing.T) {
	f := newRemotePaymentsFixture(t)
	saleRef := f.ref(t, f.saleID)
	change := func(payments ...remotePayment) error {
		return f.run(t, "change_invoice_settlement", changeInvoiceSettlementPayload{ClientRef: saleRef, Payments: payments})
	}
	// Off the account: the debt goes.
	if err := change(remotePayment{AccountName: "Cash", AmountCents: 2000}); err != nil {
		t.Fatal(err)
	}
	if got := f.owed(t); got != 0 {
		t.Fatalf("owed after moving to cash = %d", got)
	}
	// Back on it, half: half is owed.
	if err := change(remotePayment{AccountName: "Cash", AmountCents: 1000}, remotePayment{AccountName: "On Account", AmountCents: 1000}); err != nil {
		t.Fatal(err)
	}
	if got := f.owed(t); got != 1000 {
		t.Fatalf("owed = %d, want 1000", got)
	}
	// $600 paid back: the On Account part can't drop under it.
	if _, err := f.app.PayClientAccount(f.saleID, f.cashbox.ID, 600); err != nil {
		t.Fatal(err)
	}
	if err := change(remotePayment{AccountName: "Cash", AmountCents: 1500}, remotePayment{AccountName: "On Account", AmountCents: 500}); err == nil {
		t.Fatal("On Account under what was paid back should be refused")
	}
	if err := change(remotePayment{AccountName: "Card", AmountCents: 1200}, remotePayment{AccountName: "On Account", AmountCents: 800}); err != nil {
		t.Fatal(err)
	}
	if got := f.owed(t); got != 200 {
		t.Fatalf("owed = %d, want 800 - 600 = 200", got)
	}
	// A walk-in can't be put On Account.
	if err := f.run(t, "change_invoice_settlement", changeInvoiceSettlementPayload{
		ClientRef: f.clientRef, Payments: []remotePayment{{AccountName: "On Account", AmountCents: 2000}},
	}); err == nil {
		t.Fatal("walk-in sale moved On Account")
	}
}

func TestChangeSettlementRefusesCoupons(t *testing.T) {
	f := newRemotePaymentsFixture(t)
	if _, err := f.store.CreateCashboxAccount(context.Background(), db.CreateCashboxAccountParams{Name: "On Coupon", CurrencyCode: "USD"}); err != nil {
		t.Fatal(err)
	}
	err := f.run(t, "change_invoice_settlement", changeInvoiceSettlementPayload{
		ClientRef: f.clientRef, Payments: []remotePayment{{AccountName: "On Coupon", AmountCents: 2000}},
	})
	if err == nil || !strings.Contains(err.Error(), "coupon") {
		t.Fatalf("err = %v", err)
	}
}
