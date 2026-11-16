package main

import (
	"context"
	"fmt"
	"testing"

	db "kashi-pos/db/sqlc"
)

// onAccountFixture sells `amounts` on account to a new client, one invoice
// each, and returns the client and the invoice ids.
type onAccountFixture struct {
	app       *App
	store     *db.Store
	shiftID   int64
	cashID    int64
	accountID int64
	invIndex  int64
}

func newOnAccountFixture(t *testing.T) *onAccountFixture {
	t.Helper()
	ctx := context.Background()
	database := openTestDatabase(t)
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	if err := store.UpsertSettings(ctx, defaultSettingsParams()); err != nil {
		t.Fatal(err)
	}
	shift, err := store.CreateShift(ctx, db.CreateShiftParams{})
	if err != nil {
		t.Fatal(err)
	}
	onAccount, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "On Account", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	cash, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash USD", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	return &onAccountFixture{
		app: &App{ctx: ctx, store: store}, store: store,
		shiftID: shift.ID, cashID: cash.ID, accountID: onAccount.ID,
	}
}

func (f *onAccountFixture) client(t *testing.T, name string) int64 {
	t.Helper()
	c, err := f.store.CreateClient(f.app.ctx, db.CreateClientParams{Name: name, Phone: "+9617" + fmt.Sprint(1000000+f.invIndex*7+int64(len(name)))})
	if err != nil {
		t.Fatal(err)
	}
	return c.ID
}

func (f *onAccountFixture) sell(t *testing.T, clientID, cents int64) int64 {
	t.Helper()
	ctx := f.app.ctx
	f.invIndex++
	inventory, _ := f.store.CreateInventory(ctx, fmt.Sprintf("Sales %d", f.invIndex))
	salesperson, _ := f.store.CreateSalesperson(ctx, fmt.Sprintf("Cashier %d", f.invIndex))
	invoice, err := f.store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: f.accountID, ShiftID: f.shiftID, InvoiceCode: fmt.Sprintf("SA/CN/2026/%05d", f.invIndex),
		InvoiceIndex: f.invIndex, Year: 2026, ClientID: clientID, InventoryID: inventory.ID,
		Subtotal: cents, DiscountedTotal: cents, GrandTotal: cents, SalespersonID: salesperson.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.CreateClientAccountEntry(ctx, db.CreateClientAccountEntryParams{ClientID: clientID, InvoiceID: invoice.ID, Amount: cents}); err != nil {
		t.Fatal(err)
	}
	return invoice.ID
}

func onAccountRows(t *testing.T, app *App) (map[int64]map[string]any, map[string]any) {
	t.Helper()
	got, err := app.ListOnAccountClients()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[int64]map[string]any{}
	for _, row := range got["clients"].([]map[string]any) {
		byID[row["id"].(int64)] = row
	}
	return byID, got
}

func TestListOnAccountClients(t *testing.T) {
	f := newOnAccountFixture(t)
	owes := f.client(t, "Owes Two")
	partly := f.client(t, "Partly Paid")
	settled := f.client(t, "Settled")
	nothing := f.client(t, "No Account")
	_ = nothing

	f.sell(t, owes, 1000)
	f.sell(t, owes, 2500)
	partlyInv := f.sell(t, partly, 4000)
	settledInv := f.sell(t, settled, 700)

	if _, err := f.app.PayClientAccount(partlyInv, f.cashID, 1500); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.PayClientAccount(settledInv, f.cashID, 700); err != nil {
		t.Fatal(err)
	}

	byID, got := onAccountRows(t, f.app)
	if len(byID) != 2 {
		t.Fatalf("clients listed = %#v, want only the two who still owe", byID)
	}
	if _, ok := byID[settled]; ok {
		t.Fatal("a fully paid client is listed")
	}
	if _, ok := byID[nothing]; ok {
		t.Fatal("a client with no account sales is listed")
	}
	check := func(id int64, count, total, paid, outstanding int64) {
		t.Helper()
		row := byID[id]
		if row["invoiceCount"].(int64) != count || row["total"].(int64) != total ||
			row["paid"].(int64) != paid || row["outstanding"].(int64) != outstanding {
			t.Fatalf("row = %#v, want count=%d total=%d paid=%d outstanding=%d", row, count, total, paid, outstanding)
		}
	}
	check(owes, 2, 3500, 0, 3500)
	check(partly, 1, 4000, 1500, 2500)
	if got["total"].(int64) != 7500 || got["paid"].(int64) != 1500 || got["outstanding"].(int64) != 6000 {
		t.Fatalf("totals = %#v", got)
	}
	first := got["clients"].([]map[string]any)[0]
	if first["id"].(int64) != owes {
		t.Fatalf("biggest debt should come first, got %#v", first)
	}
	// The number matches what the account dialog shows for each client.
	for id, row := range byID {
		details, err := f.app.GetClientAccountDetails(id)
		if err != nil || databaseInt64(details["balance"]) != row["outstanding"].(int64) {
			t.Fatalf("client %d: list says %v, account dialog says %v (err=%v)", id, row["outstanding"], details["balance"], err)
		}
	}
}

func TestListOnAccountClientsEmpty(t *testing.T) {
	f := newOnAccountFixture(t)
	_, got := onAccountRows(t, f.app)
	if len(got["clients"].([]map[string]any)) != 0 || got["outstanding"].(int64) != 0 {
		t.Fatalf("empty till = %#v", got)
	}
}

// A paid-off invoice must not hide, or double count, the client's other open one.
func TestListOnAccountClientsMixedInvoices(t *testing.T) {
	f := newOnAccountFixture(t)
	c := f.client(t, "Mixed")
	paidOff := f.sell(t, c, 1200)
	f.sell(t, c, 800)
	if _, err := f.app.PayClientAccount(paidOff, f.cashID, 1200); err != nil {
		t.Fatal(err)
	}
	byID, _ := onAccountRows(t, f.app)
	row := byID[c]
	if row == nil || row["invoiceCount"].(int64) != 1 || row["total"].(int64) != 800 || row["paid"].(int64) != 0 || row["outstanding"].(int64) != 800 {
		t.Fatalf("row = %#v", row)
	}
	details, _ := f.app.GetClientAccountDetails(c)
	if databaseInt64(details["balance"]) != 800 {
		t.Fatalf("details = %#v", details)
	}
}
