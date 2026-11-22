package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

// exchangeOpeningFixture is a till with one sale of three units, rung up
// daysAgo days ago, under the default 3-day exchange window.
func exchangeOpeningFixture(t *testing.T, daysAgo int) (app *App, database *sql.DB, code, clientRef string, invoiceID int64) {
	t.Helper()
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "opening.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, rounding_currency, receipt_enabled, sales_inventory_id, exchange_window_hours) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 'USD', 0, 0, 72)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateShift(ctx, db.CreateShiftParams{}); err != nil {
		t.Fatal(err)
	}
	cashbox, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	client, err := store.CreateClient(ctx, db.CreateClientParams{Name: "Walk-in", Phone: "00000000"})
	if err != nil {
		t.Fatal(err)
	}
	product, err := store.CreateProduct(ctx, db.CreateProductParams{Barcode: "O1", Code: "OPEN-1", Name: "Jacket", IsActive: 1, Price: 1000})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := store.CreateInventory(ctx, "Sales")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertInventoryItem(ctx, db.UpsertInventoryItemParams{InventoryID: inventory.ID, ProductID: product.ID, Quantity: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE settings SET sales_inventory_id = ?`, inventory.ID); err != nil {
		t.Fatal(err)
	}
	priceProductsFromRows(t, store.Queries)
	app = &App{ctx: ctx, store: store, conn: database}
	items, _ := json.Marshal([]salesInvoiceItem{{Code: product.Code, Qty: 3}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 3000}})
	sale, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID})
	if err != nil {
		t.Fatal(err)
	}
	created := time.Now().UTC().AddDate(0, 0, -daysAgo).Format("2006-01-02 15:04:05")
	if _, err := database.Exec(`UPDATE invoices SET created_at = ? WHERE id = ?`, created, sale.InvoiceID); err != nil {
		t.Fatal(err)
	}
	clientRef, err = getOrCreateInvoiceClientRef(ctx, store.Queries, sale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	return app, database, sale.InvoiceCode, clientRef, sale.InvoiceID
}

func openExchange(t *testing.T, app *App, clientRef, untilDate string) error {
	t.Helper()
	payload, _ := json.Marshal(openInvoiceExchangePayload{ClientRef: clientRef, UntilDate: untilDate, Reason: "customer abroad", OpenedBy: "admin"})
	return app.executeCommand(context.Background(), branchCommand{Type: "open_invoice_exchange", Payload: payload})
}

func localDate(offsetDays int) string {
	return time.Now().AddDate(0, 0, offsetDays).Format("2006-01-02")
}

// The reported case: a sale older than 3 days, opened by central for today,
// can be exchanged on the till straight away.
func TestOpenInvoiceExchangeLetsAnOldSaleBeExchangedToday(t *testing.T) {
	app, _, code, ref, _ := exchangeOpeningFixture(t, 10)
	if _, err := app.GetExchangeInvoice(code); err == nil || !strings.Contains(err.Error(), "3 days") {
		t.Fatalf("a 10-day-old sale should be past the window, got %v", err)
	}
	if err := openExchange(t, app, ref, localDate(0)); err != nil {
		t.Fatal(err)
	}
	loaded, err := app.GetExchangeInvoice(code)
	if err != nil {
		t.Fatalf("an opened sale should load for exchange: %v", err)
	}
	if items := loaded["items"].([]map[string]any); len(items) != 1 || items[0]["available"].(int64) != 3 {
		t.Fatalf("all three units should be returnable, got %v", items)
	}
	if _, err := app.GetExchangeInvoiceByID(loaded["id"].(int64)); err != nil {
		t.Fatalf("an exchange in progress should reload the opened sale: %v", err)
	}
}

// Opened for several days, the sale stays exchangeable for a later visit.
func TestOpenInvoiceExchangeForLaterDays(t *testing.T) {
	app, _, code, ref, id := exchangeOpeningFixture(t, 30)
	if err := openExchange(t, app, ref, localDate(5)); err != nil {
		t.Fatal(err)
	}
	until, ok, err := app.exchangeOpening(context.Background(), id)
	if err != nil || !ok {
		t.Fatalf("opening not stored: %v %v", ok, err)
	}
	wantEnd := time.Now().AddDate(0, 0, 5)
	if y, m, d := until.In(time.Local).Date(); y != wantEnd.Year() || m != wantEnd.Month() || d != wantEnd.Day() {
		t.Fatalf("open until %v, want the end of %s", until, localDate(5))
	}
	if h, mi, s := until.In(time.Local).Clock(); h != 23 || mi != 59 || s != 59 {
		t.Fatalf("open until %v, want 23:59:59 local", until)
	}
	// Four days on it is still open; the day after its last day it is not.
	settings, _ := app.store.GetSettings(context.Background())
	sale, _ := app.store.GetSalesInvoiceByCode(context.Background(), code)
	window := exchangeWindowDuration(settings.ExchangeWindowHours)
	if err := app.checkExchangeAllowed(context.Background(), id, sale.CreatedAt, time.Now().AddDate(0, 0, 4), window); err != nil {
		t.Fatalf("still inside the opening: %v", err)
	}
	if err := app.checkExchangeAllowed(context.Background(), id, sale.CreatedAt, until.Add(time.Second), window); err == nil {
		t.Fatal("a second after the opening ends the sale should be refused again")
	}
}

// A newer opening replaces the old one, shorter or longer.
func TestOpenInvoiceExchangeAgainReplacesTheOpening(t *testing.T) {
	app, database, _, ref, id := exchangeOpeningFixture(t, 8)
	for _, offset := range []int{7, 0, 2} {
		if err := openExchange(t, app, ref, localDate(offset)); err != nil {
			t.Fatal(err)
		}
	}
	var rows int
	if err := database.QueryRow(`SELECT count(*) FROM exchange_openings`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("want one opening row, got %d (%v)", rows, err)
	}
	until, _, _ := app.exchangeOpening(context.Background(), id)
	if got := until.In(time.Local).Format("2006-01-02"); got != localDate(2) {
		t.Fatalf("the last opening wins: open until %s, want %s", got, localDate(2))
	}
}

// An opening that has ended leaves the sale refused like any old sale.
func TestExpiredOpeningRefusesTheExchange(t *testing.T) {
	app, database, code, _, id := exchangeOpeningFixture(t, 10)
	past := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	if _, err := database.Exec(`INSERT INTO exchange_openings (invoice_id, open_until) VALUES (?, ?)`, id, past); err != nil {
		t.Fatal(err)
	}
	if _, err := app.GetExchangeInvoice(code); err == nil {
		t.Fatal("an ended opening should not let the exchange through")
	}
}

// A sale still inside its window needs no opening, and one opened for a
// sale inside the window changes nothing.
func TestOpeningDoesNotNarrowTheWindow(t *testing.T) {
	app, database, code, _, id := exchangeOpeningFixture(t, 1)
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	if _, err := database.Exec(`INSERT INTO exchange_openings (invoice_id, open_until) VALUES (?, ?)`, id, past); err != nil {
		t.Fatal(err)
	}
	if _, err := app.GetExchangeInvoice(code); err != nil {
		t.Fatalf("a 1-day-old sale is inside the window: %v", err)
	}
}

func TestOpenInvoiceExchangeErrors(t *testing.T) {
	app, _, _, ref, _ := exchangeOpeningFixture(t, 10)
	cases := map[string]struct{ ref, date, want string }{
		"no client ref": {"", localDate(0), "client_ref is required"},
		"unknown sale":  {"nope", localDate(0), "sale not found"},
		"bad date":      {ref, "22/11/2026", "until_date must be a date"},
		"date gone by":  {ref, localDate(-1), "already passed"},
		"empty date":    {ref, "", "until_date must be a date"},
	}
	for name, c := range cases {
		err := openExchange(t, app, c.ref, c.date)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want %q", name, err, c.want)
		}
	}
	var rows int
	app.conn.QueryRow(`SELECT count(*) FROM exchange_openings`).Scan(&rows)
	if rows != 0 {
		t.Fatalf("a refused command opened %d sales", rows)
	}
}

func TestOpenInvoiceExchangeRefusesACancelledSale(t *testing.T) {
	app, database, _, ref, id := exchangeOpeningFixture(t, 10)
	if _, err := database.Exec(`UPDATE invoices SET cancelled_at = datetime('now') WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if err := openExchange(t, app, ref, localDate(0)); err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("got %v", err)
	}
}

// Over many random days, times and zones the opening ends exactly at the
// last second of the chosen day, never before now, and a date before today
// is always refused.
func TestExchangeOpenUntilRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	zones := []string{"UTC", "Asia/Beirut", "America/New_York", "Pacific/Kiritimati", "Pacific/Pago_Pago"}
	for i := 0; i < 500; i++ {
		loc, err := time.LoadLocation(zones[rng.Intn(len(zones))])
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, time.Month(1+rng.Intn(12)), 1+rng.Intn(28), rng.Intn(24), rng.Intn(60), rng.Intn(60), 0, loc)
		offset := rng.Intn(40) - 5
		date := now.AddDate(0, 0, offset).Format("2006-01-02")
		until, err := exchangeOpenUntil(date, now, loc)
		if offset < 0 {
			if err == nil {
				t.Fatalf("%s at %v: a past date was accepted", date, now)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s at %v: %v", date, now, err)
		}
		local := until.In(loc)
		if local.Format("2006-01-02") != date || local.Add(time.Second).Format("2006-01-02") == date {
			t.Fatalf("%s at %v: open until %v is not the day's last second", date, now, local)
		}
		if until.Before(now) {
			t.Fatalf("%s at %v: opening already over at %v", date, now, until)
		}
	}
}
