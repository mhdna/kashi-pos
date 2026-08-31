package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	db "kashi-pos/db/sqlc"
)

// outboxByType returns every pending outbox entry of one entity_type, in id
// order (so the newest settlement change is last).
func outboxByType(t *testing.T, app *App, store *db.Store, entityType string) []db.SyncOutbox {
	t.Helper()
	entries, err := store.ListDueSyncOutboxEntries(app.ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	var out []db.SyncOutbox
	for _, entry := range entries {
		if entry.EntityType == entityType {
			out = append(out, entry)
		}
	}
	return out
}

func decodeSettlement(t *testing.T, entry db.SyncOutbox) branchSettlementPayload {
	t.Helper()
	var payload branchSettlementPayload
	if err := json.Unmarshal([]byte(entry.PayloadJson), &payload); err != nil {
		t.Fatalf("decode settlement payload: %v", err)
	}
	return payload
}

// seedSaleForSettlement checks out a one-line USD sale and returns it plus
// its queued branch client_ref, so a settlement-change test can assert the
// change is linked back to the same sale.
func seedSaleForSettlement(t *testing.T, app *App, store *db.Store, cashbox db.CashboxAccount, client db.Client) (SalesInvoiceResult, string) {
	t.Helper()
	items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": 1}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 100}})
	sale, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID})
	if err != nil {
		t.Fatal(err)
	}
	saleEntries := outboxByType(t, app, store, "sales_invoice")
	if len(saleEntries) != 1 {
		t.Fatalf("expected the sale to queue 1 sales_invoice entry, got %d", len(saleEntries))
	}
	var salePayload branchInvoicePayload
	if err := json.Unmarshal([]byte(saleEntries[0].PayloadJson), &salePayload); err != nil {
		t.Fatal(err)
	}
	return sale, salePayload.ClientRef
}

func TestChangeInvoiceSettlementQueuesSettlementSyncLinkedToSale(t *testing.T) {
	app, store, cashbox, client := setUpTestInvoiceApp(t, "http://localhost:9999")
	cash2, err := store.CreateCashboxAccount(app.ctx, db.CreateCashboxAccountParams{Name: "Cash 2", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	sale, saleRef := seedSaleForSettlement(t, app, store, cashbox, client)

	if err := app.ChangeInvoiceSettlement(sale.InvoiceID, cash2.ID); err != nil {
		t.Fatal(err)
	}

	entries := outboxByType(t, app, store, "invoice_settlement")
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 invoice_settlement entry, got %d", len(entries))
	}
	payload := decodeSettlement(t, entries[0])
	if payload.SaleClientRef != saleRef {
		t.Fatalf("settlement sale_client_ref = %q, want the sale's ref %q", payload.SaleClientRef, saleRef)
	}
	if payload.SettlementRef == "" || payload.SettlementRef == saleRef {
		t.Fatalf("settlement_ref = %q, want a fresh non-empty ref distinct from the sale", payload.SettlementRef)
	}
	if payload.GrandTotal != 100 {
		t.Fatalf("settlement grand_total = %d, want 100 (frozen from the sale)", payload.GrandTotal)
	}
	if len(payload.Payments) != 1 || payload.Payments[0].AccountName != "Cash 2" || payload.Payments[0].Amount != 100 {
		t.Fatalf("settlement payments = %+v, want the full 100 moved to Cash 2", payload.Payments)
	}
}

func TestChangeInvoiceSettlementAmountQueuesPartialSplit(t *testing.T) {
	app, store, cashbox, client := setUpTestInvoiceApp(t, "http://localhost:9999")
	cash2, err := store.CreateCashboxAccount(app.ctx, db.CreateCashboxAccountParams{Name: "Cash 2", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	sale, saleRef := seedSaleForSettlement(t, app, store, cashbox, client)

	if err := app.ChangeInvoiceSettlementAmount(sale.InvoiceID, cashbox.ID, cash2.ID, 40); err != nil {
		t.Fatal(err)
	}

	entries := outboxByType(t, app, store, "invoice_settlement")
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 invoice_settlement entry, got %d", len(entries))
	}
	payload := decodeSettlement(t, entries[0])
	if payload.SaleClientRef != saleRef {
		t.Fatalf("settlement sale_client_ref = %q, want %q", payload.SaleClientRef, saleRef)
	}
	byName := map[string]int64{}
	var total int64
	for _, p := range payload.Payments {
		byName[p.AccountName] = p.Amount
		total += p.Amount
	}
	if total != 100 {
		t.Fatalf("settlement payments sum to %d, want 100 (must reconcile to grand_total)", total)
	}
	if byName["Cash"] != 60 || byName["Cash 2"] != 40 {
		t.Fatalf("settlement split = %+v, want Cash=60 Cash 2=40", byName)
	}
}

func TestSettleInvoicePaymentsQueuesSettlementSync(t *testing.T) {
	app, store, cashbox, client := setUpTestInvoiceApp(t, "http://localhost:9999")
	cash2, err := store.CreateCashboxAccount(app.ctx, db.CreateCashboxAccountParams{Name: "Cash 2", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	sale, saleRef := seedSaleForSettlement(t, app, store, cashbox, client)

	newPayments, _ := json.Marshal([]checkoutPayment{{AccountID: cash2.ID, AmountCents: 100, EntryType: "payment"}})
	if _, err := app.SettleInvoicePayments(sale.InvoiceID, string(newPayments)); err != nil {
		t.Fatal(err)
	}

	entries := outboxByType(t, app, store, "invoice_settlement")
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 invoice_settlement entry, got %d", len(entries))
	}
	payload := decodeSettlement(t, entries[0])
	if payload.SaleClientRef != saleRef {
		t.Fatalf("settlement sale_client_ref = %q, want %q", payload.SaleClientRef, saleRef)
	}
	if len(payload.Payments) != 1 || payload.Payments[0].AccountName != "Cash 2" || payload.Payments[0].Amount != 100 {
		t.Fatalf("settlement payments = %+v, want a single Cash 2 = 100 line", payload.Payments)
	}
}

func TestSettlementChangeDoesNotQueueWhenSyncNotConfigured(t *testing.T) {
	app, store, cashbox, client := setUpTestInvoiceApp(t, "")

	items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": 1}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 100}})
	sale, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID})
	if err != nil {
		t.Fatal(err)
	}
	cash2, err := store.CreateCashboxAccount(app.ctx, db.CreateCashboxAccountParams{Name: "Cash 2", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}

	if err := app.ChangeInvoiceSettlement(sale.InvoiceID, cash2.ID); err != nil {
		t.Fatal(err)
	}

	count, err := store.CountPendingSyncOutbox(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected no outbox entries on an unconfigured install, got %d", count)
	}
}

func TestSyncWorkerRoutesSettlementChangeToItsEndpoint(t *testing.T) {
	var mu sync.Mutex
	paths := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths[r.URL.Path]++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	app, store, cashbox, client := setUpTestInvoiceApp(t, server.URL)
	cash2, err := store.CreateCashboxAccount(app.ctx, db.CreateCashboxAccountParams{Name: "Cash 2", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	sale, _ := seedSaleForSettlement(t, app, store, cashbox, client)
	if err := app.ChangeInvoiceSettlement(sale.InvoiceID, cash2.ID); err != nil {
		t.Fatal(err)
	}

	if err := app.runSyncOutboxOnce(app.ctx, newCentralClient(server.URL, "TEST01", "test-key")); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if paths["/branch/sales_invoices"] != 1 {
		t.Fatalf("POSTs to /branch/sales_invoices = %d, want 1", paths["/branch/sales_invoices"])
	}
	if paths["/branch/invoice_settlements"] != 1 {
		t.Fatalf("POSTs to /branch/invoice_settlements = %d, want 1", paths["/branch/invoice_settlements"])
	}
	count, err := store.CountPendingSyncOutbox(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 pending after a successful drain, got %d", count)
	}
}
