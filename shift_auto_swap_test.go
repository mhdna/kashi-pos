package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	db "kashi-pos/db/sqlc"
)

// With the setting on, closing prep moves a negative Cash LL balance into
// Cash $$: LL ends at 0 and USD drops by the same value. Off, nothing moves.
func TestPrepareShiftCloseCoversNegativeLBPFromUSD(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "autoswap.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	if err := store.UpsertSettings(ctx, defaultSettingsParams()); err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, store: store}
	shift, err := store.CreateShift(ctx, db.CreateShiftParams{})
	if err != nil {
		t.Fatal(err)
	}

	cash := func(currency, name string) int64 {
		t.Helper()
		accounts, err := store.ListCashboxAccounts(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range accounts {
			if isCashAccount(a.Name, a.CurrencyCode) && strings.EqualFold(a.CurrencyCode, currency) {
				return a.ID
			}
		}
		a, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: name, CurrencyCode: currency})
		if err != nil {
			t.Fatal(err)
		}
		return a.ID
	}
	usdID, lbpID := cash("USD", "Cash $$"), cash("LBP", "Cash LL")

	inventory, err := store.CreateInventory(ctx, "Sales")
	if err != nil {
		t.Fatal(err)
	}
	client, err := store.CreateClient(ctx, db.CreateClientParams{Name: "Walk-in", Phone: "00000000"})
	if err != nil {
		t.Fatal(err)
	}
	// $100 paid in USD, $7.66 given back as LBP change.
	invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: usdID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/00001",
		InvoiceIndex: 1, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		GrandTotal: 9234, DiscountedTotal: 9234, Subtotal: 9234,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []db.CreateInvoicePaymentParams{
		{InvoiceID: invoice.ID, CashboxID: usdID, Amount: 10000},
		{InvoiceID: invoice.ID, CashboxID: lbpID, Amount: -766},
	} {
		if err := store.CreateInvoicePayment(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	balances := func() map[int64]int64 {
		t.Helper()
		rows, err := app.ListShiftBalances(shift.ID)
		if err != nil {
			t.Fatal(err)
		}
		out := map[int64]int64{}
		for _, row := range rows {
			out[row["accountId"].(int64)] = databaseInt64(row["balance"])
		}
		return out
	}

	if changed, err := app.PrepareShiftClose(shift.ID); err != nil || changed {
		t.Fatalf("setting off: changed=%v err=%v, want nothing moved", changed, err)
	}
	if err := app.SetAutoCoverNegativeLBPEnabled(true); err != nil {
		t.Fatal(err)
	}
	if changed, err := app.PrepareShiftClose(shift.ID); err != nil || !changed {
		t.Fatalf("setting on: changed=%v err=%v, want the LBP covered", changed, err)
	}
	got := balances()
	if got[lbpID] != 0 || got[usdID] != 9234 {
		t.Fatalf("balances = LL %d, $$ %d; want 0 and 9234", got[lbpID], got[usdID])
	}
	if changed, err := app.PrepareShiftClose(shift.ID); err != nil || changed {
		t.Fatalf("second run: changed=%v err=%v, want nothing left to move", changed, err)
	}
}
