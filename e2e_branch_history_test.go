//go:build e2e

package main

// Manual e2e harness for the branch-history importer, exercised against a
// real running kashi (see api/branch_history_sync.go there) rather than a
// mock. Usage:
//
//	E2E_URL=http://localhost:8090 E2E_CODE=E2ETB2 E2E_KEY=... \
//	  go test -tags e2e -run TestE2EBranchHistoryImport -v -timeout 5m .
//
// Calls the branchHistoryImporter's import* methods directly rather than
// going through PullBranchHistory/CreateBranch: those call ResyncAll/
// runSyncPullLocked, which emit wails runtime.EventsEmit events that
// hard-exit the process (log.Fatal) outside a real Wails frontend context --
// a constraint of testing App methods headlessly, not a bug in this
// feature. This still exercises the actual, unmodified import logic
// (resolveSalesperson/resolveCashboxAccount/resolveHistoricalProduct/
// importShifts/importInvoices/importExpenses/importLoans) against real
// JSON kashi returns, which is what actually needed verifying.
import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func TestE2EBranchHistoryImport(t *testing.T) {
	url := os.Getenv("E2E_URL")
	code := os.Getenv("E2E_CODE")
	key := os.Getenv("E2E_KEY")
	if url == "" || code == "" || key == "" {
		t.Skip("E2E_URL/E2E_CODE/E2E_KEY not set")
	}

	path := filepath.Join(t.TempDir(), "e2e_history.db")
	conn, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := applyMigrations(context.Background(), conn, path, true); err != nil {
		t.Fatal(err)
	}

	store := db.NewStore(conn)
	ctx := context.Background()

	// Minimum local state PullBranchHistory's own catalog pull would
	// otherwise have brought down: a matching cashbox account name, a
	// currency, and one inventory/client -- resolveInventory/resolveClient
	// fall back to whatever exists locally.
	if _, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{
		Name: "Cash $$", CurrencyCode: "USD",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateInventory(ctx, "Default"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateClient(ctx, db.CreateClientParams{Name: "Walk-in", Phone: "0"}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store, conn: conn}
	client := newCentralClient(url, code, key)
	if !client.enabled() {
		t.Fatal("central client not enabled -- check E2E_URL/E2E_CODE/E2E_KEY")
	}

	imp := &branchHistoryImporter{
		app:               app,
		client:            client,
		shiftIDByBranch:   map[int64]int64{},
		salespersonByName: map[string]int64{},
		cashboxByName:     map[string]int64{},
		invoiceIDByRef:    map[string]int64{},
	}

	shiftCount, err := imp.importShifts(ctx)
	if err != nil {
		t.Fatalf("importShifts: %v", err)
	}
	t.Logf("imported %d shift close(s)", shiftCount)
	if shiftCount == 0 {
		t.Fatal("expected at least one imported shift")
	}

	invoiceCount, unresolvedItems, err := imp.importInvoices(ctx)
	if err != nil {
		t.Fatalf("importInvoices: %v", err)
	}
	t.Logf("imported %d invoice(s), %d unresolved item(s)", invoiceCount, unresolvedItems)
	if invoiceCount == 0 {
		t.Fatal("expected at least one imported invoice")
	}

	expenseCount, err := imp.importExpenses(ctx)
	if err != nil {
		t.Fatalf("importExpenses: %v", err)
	}
	t.Logf("imported %d expense(s)", expenseCount)
	if expenseCount == 0 {
		t.Fatal("expected at least one imported expense")
	}

	loanCount, err := imp.importLoans(ctx)
	if err != nil {
		t.Fatalf("importLoans: %v", err)
	}
	t.Logf("imported %d loan(s)", loanCount)
	if loanCount == 0 {
		t.Fatal("expected at least one imported loan")
	}

	// Verify the actual data, not just counts.
	invoices, err := store.ListInvoices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, inv := range invoices {
		items, _ := store.ListInvoiceProducts(ctx, inv.ID)
		payments, _ := store.ListInvoicePayments(ctx, inv.ID)
		raw, _ := json.MarshalIndent(map[string]any{"invoice": inv, "items": items, "payments": payments}, "", "  ")
		t.Logf("invoice:\n%s", raw)
		if inv.GrandTotal != 1800 {
			t.Errorf("invoice grand_total = %d, want 1800", inv.GrandTotal)
		}
		if len(payments) != 1 || payments[0].Amount != 1800 {
			t.Errorf("invoice payments = %+v, want one payment of 1800", payments)
		}
		if len(items) != 1 || items[0].Quantity != 2 {
			t.Errorf("invoice items = %+v, want one item with quantity 2", items)
		}
	}

	expenses, err := store.ListAllExpenses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range expenses {
		raw, _ := json.MarshalIndent(e, "", "  ")
		t.Logf("expense:\n%s", raw)
		if e.Amount != 500 {
			t.Errorf("expense amount = %d, want 500", e.Amount)
		}
	}

	loans, err := store.ListAllLoans(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range loans {
		raw, _ := json.MarshalIndent(l, "", "  ")
		t.Logf("loan:\n%s", raw)
		if l.Amount != 1000 {
			t.Errorf("loan amount = %d, want 1000", l.Amount)
		}
	}

	shifts, err := store.ListClosedShifts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range shifts {
		raw, _ := json.MarshalIndent(s, "", "  ")
		t.Logf("shift:\n%s", raw)
	}
}
