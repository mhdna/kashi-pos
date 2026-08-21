package main

import (
	"encoding/json"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

func TestInvoiceLineSalespersonPersistsAndOverridesReportAttribution(t *testing.T) {
	app, cashbox, client, _ := setupQuoteTestApp(t, 0, roundingHalfUp, "USD")
	linePerson, err := app.store.CreateSalesperson(app.ctx, "Line Person")
	if err != nil {
		t.Fatal(err)
	}
	invoicePerson, err := app.store.CreateSalesperson(app.ctx, "Invoice Person")
	if err != nil {
		t.Fatal(err)
	}

	checkout := func(salespersonID int64) SalesInvoiceResult {
		t.Helper()
		items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": 1, "salespersonId": salespersonID}})
		payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 1533, EntryType: "payment"}})
		result, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID})
		if err != nil {
			t.Fatal(err)
		}
		if err := app.store.UpdateInvoiceSalesperson(app.ctx, db.UpdateInvoiceSalespersonParams{ID: result.InvoiceID, SalespersonID: invoicePerson.ID}); err != nil {
			t.Fatal(err)
		}
		return result
	}

	assigned := checkout(linePerson.ID)
	checkout(0)
	lines, err := app.store.ListInvoiceProducts(app.ctx, assigned.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0].SalespersonID != linePerson.ID {
		t.Fatalf("persisted invoice lines = %#v", lines)
	}

	day := time.Now().Format("2006-01-02")
	report, err := app.ListSalespersonProductSales(day, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(report) != 2 {
		t.Fatalf("report rows = %#v", report)
	}
	byPerson := make(map[int64]SalespersonProductSale, len(report))
	for _, row := range report {
		byPerson[row.SalespersonID] = row
	}
	if byPerson[linePerson.ID].Quantity != 1 || byPerson[invoicePerson.ID].Quantity != 1 {
		t.Fatalf("report attribution = %#v", report)
	}
}
