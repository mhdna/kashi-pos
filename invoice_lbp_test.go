package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"
)

// Checkout stamps the LBP total the receipt prints, and the backfill
// payload reads the stamped one back rather than recomputing it.
func TestStampInvoiceLBPTotal(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "lbp.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	lbp, err := store.GetCurrencyByCode(ctx, "LBP")
	if err != nil {
		t.Fatal(err)
	}

	shift, err := store.CreateShift(ctx, db.CreateShiftParams{})
	if err != nil {
		t.Fatal(err)
	}
	usd, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash USD", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := store.CreateInventory(ctx, "Sales")
	if err != nil {
		t.Fatal(err)
	}
	client, err := store.CreateClient(ctx, db.CreateClientParams{Name: "Walk-in", Phone: "00000000"})
	if err != nil {
		t.Fatal(err)
	}
	invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: usd.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/00001",
		InvoiceIndex: 1, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		Subtotal: 111, DiscountedTotal: 111, GrandTotal: 111,
	})
	if err != nil {
		t.Fatal(err)
	}

	total, rate, err := stampInvoiceLBPTotal(ctx, store.Queries, invoice.ID, 111, "half_up")
	if err != nil {
		t.Fatal(err)
	}
	want, err := roundCurrencyUnitsFromCents(111, lbp.UnitsPerUsdMicros, lbp.CashRoundingUnit, "half_up")
	if err != nil {
		t.Fatal(err)
	}
	if total == nil || *total != want || *rate != lbp.UnitsPerUsdMicros/unitsPerUsdMicroScale {
		t.Fatalf("stamped %v at %v, want %d at %d", total, rate, want, lbp.UnitsPerUsdMicros/unitsPerUsdMicroScale)
	}

	stored, err := store.GetInvoiceByID(ctx, invoice.ID)
	if err != nil {
		t.Fatal(err)
	}
	gotTotal, gotRate := storedInvoiceLBPTotal(stored)
	if gotTotal == nil || *gotTotal != want || *gotRate != *rate {
		t.Fatalf("stored %v at %v, want %d at %d", gotTotal, gotRate, want, *rate)
	}

	refund, _, ok := invoiceLBPTotal(ctx, store.Queries, -111, "half_up")
	if !ok || refund != -want {
		t.Fatalf("refund %d, want %d", refund, -want)
	}
}
