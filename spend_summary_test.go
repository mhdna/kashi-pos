package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

// The month totals add up per currency, and the shift lists hold what was
// entered during the open shift.
func TestGetSpendSummary(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "spend.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	if _, err := database.Exec(`INSERT INTO settings (
		branch_name, api_key, printer_id, printer_size, receipt_printer,
		screen_port, receipt_width, receipt_height, theme, tax_rate,
		rounding_mode, receipt_enabled, sales_inventory_id, central_api_url
	) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0, '')`); err != nil {
		t.Fatal(err)
	}
	shift, err := store.CreateShift(ctx, db.CreateShiftParams{})
	if err != nil {
		t.Fatal(err)
	}
	usd, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash $$", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	lbp, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash LL", CurrencyCode: "LBP"})
	if err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, store: store, conn: database}
	for _, e := range []struct {
		amount   int64
		currency string
		account  int64
	}{{10000, "USD", usd.ID}, {2500, "USD", usd.ID}, {20000000, "LBP", lbp.ID}} {
		if _, err := app.CreateExpense("Supplies", "", e.amount, e.currency, e.account); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := app.CreateLoan("Advance", "Staff", 5000, "USD", usd.ID); err != nil {
		t.Fatal(err)
	}

	got, err := app.GetSpendSummary(shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	totals := map[string]int64{}
	for _, row := range got["monthExpenses"].([]db.SumExpensesByCurrencySinceRow) {
		totals[row.CurrencyCode] = row.Total
	}
	if totals["USD"] != 12500 || totals["LBP"] != 20000000 {
		t.Fatalf("month expenses = %v, want USD 12500 and LBP 20000000", totals)
	}
	if loans := got["monthLoans"].([]db.SumLoansByCurrencySinceRow); len(loans) != 1 || loans[0].Total != 5000 {
		t.Fatalf("month loans = %+v, want one USD 5000", loans)
	}
	if n := len(got["shiftExpenses"].([]map[string]any)); n != 3 {
		t.Fatalf("shift expenses = %d, want 3", n)
	}
	if n := len(got["shiftLoans"].([]map[string]any)); n != 1 {
		t.Fatalf("shift loans = %d, want 1", n)
	}
}
