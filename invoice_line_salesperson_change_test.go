package main

import (
	"encoding/json"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

// Changing who sold one line of a finished invoice: the line takes the new
// person, the report follows, 0 hands the line back to the invoice's person,
// and a line of another invoice (or a stranger) is refused.
func TestChangeInvoiceLineSalesperson(t *testing.T) {
	app, cashbox, client, _ := setupQuoteTestApp(t, 0, roundingHalfUp, "USD")
	invoicePerson, err := app.store.CreateSalesperson(app.ctx, "Invoice Person")
	if err != nil {
		t.Fatal(err)
	}
	other, err := app.store.CreateSalesperson(app.ctx, "Other Person")
	if err != nil {
		t.Fatal(err)
	}

	sell := func() (SalesInvoiceResult, int64) {
		t.Helper()
		items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": 1}})
		payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 1533, EntryType: "payment"}})
		result, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID})
		if err != nil {
			t.Fatal(err)
		}
		if err := app.store.UpdateInvoiceSalesperson(app.ctx, db.UpdateInvoiceSalespersonParams{ID: result.InvoiceID, SalespersonID: invoicePerson.ID}); err != nil {
			t.Fatal(err)
		}
		details, err := app.GetInvoiceDetails(result.InvoiceID)
		if err != nil {
			t.Fatal(err)
		}
		lineID, _ := details["items"].([]map[string]any)[0]["lineId"].(int64)
		if lineID == 0 {
			t.Fatalf("invoice detail items carry no lineId: %#v", details["items"])
		}
		return result, lineID
	}
	lineSalesperson := func(invoiceID int64) (int64, string) {
		t.Helper()
		details, err := app.GetInvoiceDetails(invoiceID)
		if err != nil {
			t.Fatal(err)
		}
		item := details["items"].([]map[string]any)[0]
		return item["salespersonId"].(int64), item["salespersonName"].(string)
	}

	first, firstLine := sell()
	second, secondLine := sell()

	if err := app.ChangeInvoiceLineSalesperson(first.InvoiceID, firstLine, other.ID); err != nil {
		t.Fatal(err)
	}
	if id, name := lineSalesperson(first.InvoiceID); id != other.ID || name != "Other Person" {
		t.Fatalf("line salesperson = %d %q, want Other Person", id, name)
	}
	// The other invoice is untouched.
	if id, _ := lineSalesperson(second.InvoiceID); id != 0 {
		t.Fatalf("second invoice's line changed to %d", id)
	}

	day := time.Now().Format("2006-01-02")
	report, err := app.ListSalespersonProductSales(day, day)
	if err != nil {
		t.Fatal(err)
	}
	byPerson := map[int64]float64{}
	for _, row := range report {
		byPerson[row.SalespersonID] += float64(row.Quantity)
	}
	if byPerson[other.ID] != 1 || byPerson[invoicePerson.ID] != 1 {
		t.Fatalf("report after the change = %#v", report)
	}

	// 0 gives the line back to the invoice's own salesperson.
	if err := app.ChangeInvoiceLineSalesperson(first.InvoiceID, firstLine, 0); err != nil {
		t.Fatal(err)
	}
	if id, _ := lineSalesperson(first.InvoiceID); id != 0 {
		t.Fatalf("cleared line salesperson = %d", id)
	}

	// A line that belongs to a different invoice, an unknown line, an unknown
	// salesperson and a negative id are all refused and change nothing.
	for name, call := range map[string]func() error{
		"another invoice's line": func() error { return app.ChangeInvoiceLineSalesperson(first.InvoiceID, secondLine, other.ID) },
		"unknown line":           func() error { return app.ChangeInvoiceLineSalesperson(first.InvoiceID, 999999, other.ID) },
		"unknown salesperson":    func() error { return app.ChangeInvoiceLineSalesperson(first.InvoiceID, firstLine, 999999) },
		"negative salesperson":   func() error { return app.ChangeInvoiceLineSalesperson(first.InvoiceID, firstLine, -1) },
	} {
		if err := call(); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if id, _ := lineSalesperson(first.InvoiceID); id != 0 {
		t.Fatalf("a refused change still moved the first line to %d", id)
	}
	if id, _ := lineSalesperson(second.InvoiceID); id != 0 {
		t.Fatalf("a refused change still moved the second line to %d", id)
	}
}
