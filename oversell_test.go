package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

// setUpOversellApp is a synced till (outbox on) whose product SKU-1 is
// tracked centrally, belonging to branch CT while logging in as CT2.
func setUpOversellApp(t *testing.T, mode string, onHand int64) (*App, *db.Store, db.CashboxAccount, db.Client) {
	t.Helper()
	app, store, cashbox, client := setUpTestInvoiceApp(t, "http://kashi.test")
	product, err := store.GetProductByCode(app.ctx, "SKU-1")
	if err != nil {
		t.Fatal(err)
	}
	if !product.Uid.Valid {
		t.Fatal("test product has no uid")
	}
	if err := store.SetHomeBranchCode(app.ctx, "CT"); err != nil {
		t.Fatal(err)
	}
	if err := app.SetOversellMode(mode); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertBranchStock(app.ctx, db.UpsertBranchStockParams{
		BranchCode: "CT", VariantUid: product.Uid.String, Barcode: "1", Quantity: onHand,
		UpdatedAt: time.Now().UTC().Format(sqliteTimeLayout),
	}); err != nil {
		t.Fatal(err)
	}
	return app, store, cashbox, client
}

func sellSKU1(t *testing.T, app *App, cashbox db.CashboxAccount, client db.Client, qty int) error {
	t.Helper()
	items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": qty}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: int64(100 * qty)}})
	_, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID})
	return err
}

func cartSKU1(qty int) string {
	items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": qty}})
	return string(items)
}

func TestCheckOversell_CountsUnsyncedLocalSales(t *testing.T) {
	app, _, cashbox, client := setUpOversellApp(t, oversellWarn, 3)

	check, err := app.CheckOversell(cartSKU1(3))
	if err != nil {
		t.Fatal(err)
	}
	if check.Mode != oversellWarn || len(check.Lines) != 0 {
		t.Fatalf("3 of 3 in stock should pass: %+v", check)
	}

	// A sale kashi hasn't received yet uses up 2 of the 3.
	if err := sellSKU1(t, app, cashbox, client, 2); err != nil {
		t.Fatal(err)
	}
	check, err = app.CheckOversell(cartSKU1(3))
	if err != nil {
		t.Fatal(err)
	}
	if len(check.Lines) != 1 || check.Lines[0].Available != 1 || check.Lines[0].Requested != 3 {
		t.Fatalf("expected 1 available of 3 requested, got %+v", check.Lines)
	}
}

func TestCheckoutBlockModeRefusesButWarnSells(t *testing.T) {
	app, _, cashbox, client := setUpOversellApp(t, oversellBlock, 1)
	err := sellSKU1(t, app, cashbox, client, 2)
	if err == nil || !strings.Contains(err.Error(), "not enough stock") {
		t.Fatalf("block mode should refuse, got %v", err)
	}

	app, _, cashbox, client = setUpOversellApp(t, oversellWarn, 1)
	if err := sellSKU1(t, app, cashbox, client, 2); err != nil {
		t.Fatalf("warn mode must still sell at checkout (the confirm is the sale screen's): %v", err)
	}
}

func TestCheckOversell_UnknownStockNeverBlocks(t *testing.T) {
	app, _, cashbox, client := setUpTestInvoiceApp(t, "http://kashi.test")
	if err := app.SetOversellMode(oversellBlock); err != nil {
		t.Fatal(err)
	}
	// No branch_stock rows for this branch at all: stock is unknown.
	if err := sellSKU1(t, app, cashbox, client, 50); err != nil {
		t.Fatalf("a till with no stock data must keep selling: %v", err)
	}
}

func TestTargetProgress_BranchWidePlusUnsyncedLocal(t *testing.T) {
	app, store, cashbox, client := setUpTestInvoiceApp(t, "http://kashi.test")
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	to := from.AddDate(0, 1, -1)
	if err := store.UpsertBranchTarget(app.ctx, db.UpsertBranchTargetParams{
		RemoteID: 77, DateFrom: from.Format("2006-01-02"), DateTo: to.Format("2006-01-02"),
		TargetAmountCents: 100000, Color: "#4C6EF5",
	}); err != nil {
		t.Fatal(err)
	}

	achieved := func() int64 {
		t.Helper()
		rows, err := app.ListBranchTargetsWithProgress()
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("want 1 target, got %d", len(rows))
		}
		return rows[0]["achievedCents"].(int64)
	}

	// This till's sale, not yet on kashi.
	if err := sellSKU1(t, app, cashbox, client, 1); err != nil {
		t.Fatal(err)
	}
	if got := achieved(); got != 100 {
		t.Fatalf("before any kashi figure: want local 100, got %d", got)
	}

	// kashi reports 5000 branch-wide (the other registers' sales); our sale
	// is still queued, so it's added on top.
	fetchedAt := time.Now().UTC().Add(-time.Minute).Format(sqliteTimeLayout)
	if err := store.InsertBranchTargetProgress(app.ctx, db.InsertBranchTargetProgressParams{
		TargetID: 77, AchievedCents: 5000, FetchedAt: fetchedAt,
	}); err != nil {
		t.Fatal(err)
	}
	if got := achieved(); got != 5100 {
		t.Fatalf("want 5000 + 100 unsynced, got %d", got)
	}

	// Our sale lands on kashi before the next fetch: kashi's 5000 doesn't
	// have it yet, so it still counts once.
	due, err := store.ListDueSyncOutboxEntries(app.ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range due {
		if err := store.MarkSyncOutboxSynced(app.ctx, db.MarkSyncOutboxSyncedParams{SyncedTo: "http://kashi.test", ID: e.ID}); err != nil {
			t.Fatal(err)
		}
	}
	if got := achieved(); got != 5100 {
		t.Fatalf("synced after the fetch: want 5100, got %d", got)
	}

	// A later fetch includes it: no double count.
	if err := store.DeleteBranchTargetProgress(app.ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertBranchTargetProgress(app.ctx, db.InsertBranchTargetProgressParams{
		TargetID: 77, AchievedCents: 5100, FetchedAt: time.Now().UTC().Add(time.Minute).Format(sqliteTimeLayout),
	}); err != nil {
		t.Fatal(err)
	}
	if got := achieved(); got != 5100 {
		t.Fatalf("after refetch: want 5100, got %d", got)
	}
}
