package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// An unknown item the cashier named: the name is what the invoice detail, the
// list and the receipt call it, instead of "Unknown item". Any other product
// keeps its own name whatever the cart sends.
func TestUnknownItemNameShowsOnTheInvoiceAndReceipt(t *testing.T) {
	app, cashbox, client, _ := setupQuoteTestApp(t, 0, roundingHalfUp, "USD")
	if _, err := app.GetUnknownProduct(); err != nil {
		t.Fatal(err)
	}

	sell := func(items []map[string]any, cents int64) (SalesInvoiceResult, error) {
		t.Helper()
		itemsJSON, _ := json.Marshal(items)
		payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: cents, EntryType: "payment"}})
		return app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(itemsJSON), PaymentsJSON: string(payments), ClientID: client.ID})
	}
	unknown := func(name string) map[string]any {
		return map[string]any{"code": "UNKNOWN", "barcode": "UNKNOWN", "qty": 1, "unitPricesCents": []int64{2500}, "name": name}
	}
	detailNames := func(id int64) []string {
		t.Helper()
		details, err := app.GetInvoiceDetails(id)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, item := range details["items"].([]map[string]any) {
			names = append(names, item["name"].(string))
		}
		return names
	}

	// Named: the receipt items, the stored line and the detail all say so.
	result, err := sell([]map[string]any{unknown("  Red scarf  ")}, 2500)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].Name != "Red scarf" {
		t.Fatalf("receipt items = %#v, want one named Red scarf (trimmed)", result.Items)
	}
	lines, err := app.store.ListInvoiceProducts(app.ctx, result.InvoiceID)
	if err != nil || len(lines) != 1 || lines[0].ItemName != "Red scarf" {
		t.Fatalf("stored lines = %#v, %v", lines, err)
	}
	if got := detailNames(result.InvoiceID); len(got) != 1 || got[0] != "Red scarf" {
		t.Fatalf("detail names = %v", got)
	}

	// An exchange lists what can be returned by the same name.
	loaded, err := app.GetExchangeInvoiceByID(result.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	if returnable, ok := loaded["items"].([]map[string]any); !ok || len(returnable) != 1 || returnable[0]["name"] != "Red scarf" {
		t.Fatalf("exchange items = %#v, want one named Red scarf", loaded["items"])
	}

	// Not named (an old cart, central's live cart): still the placeholder's name.
	plain, err := sell([]map[string]any{unknown("   ")}, 2500)
	if err != nil {
		t.Fatal(err)
	}
	if got := detailNames(plain.InvoiceID); len(got) != 1 || got[0] != unknownProductName {
		t.Fatalf("an unnamed unknown item reads %v, want %q", got, unknownProductName)
	}

	// A normal product ignores a name in the cart: its name is its own.
	normal, err := sell([]map[string]any{{"code": "SKU-1", "qty": 1, "name": "Hacked name"}}, 1533)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range detailNames(normal.InvoiceID) {
		if strings.Contains(name, "Hacked") {
			t.Fatalf("a product line took the cart's name: %q", name)
		}
	}
	if strings.Contains(normal.Items[0].Name, "Hacked") {
		t.Fatalf("a product's receipt line took the cart's name: %q", normal.Items[0].Name)
	}

	// Two unknown items on one sale each keep their own name.
	two, err := sell([]map[string]any{unknown("Red scarf"), unknown("Blue hat")}, 5000)
	if err != nil {
		t.Fatal(err)
	}
	names := detailNames(two.InvoiceID)
	if len(names) != 2 || !((names[0] == "Red scarf" && names[1] == "Blue hat") || (names[0] == "Blue hat" && names[1] == "Red scarf")) {
		t.Fatalf("two unknown items read %v", names)
	}
}

func TestUnknownItemNameIsLimited(t *testing.T) {
	app, cashbox, client, _ := setupQuoteTestApp(t, 0, roundingHalfUp, "USD")
	if _, err := app.GetUnknownProduct(); err != nil {
		t.Fatal(err)
	}
	try := func(name string) error {
		items, _ := json.Marshal([]map[string]any{{"code": "UNKNOWN", "barcode": "UNKNOWN", "qty": 1, "unitPricesCents": []int64{2500}, "name": name}})
		payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 2500, EntryType: "payment"}})
		_, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID})
		return err
	}
	if err := try(strings.Repeat("ا", maxItemNameRunes)); err != nil { // the limit counts letters, not bytes
		t.Fatalf("a name of exactly %d letters: %v", maxItemNameRunes, err)
	}
	if err := try(strings.Repeat("a", maxItemNameRunes+1)); err == nil {
		t.Fatal("a name one letter over the limit was accepted")
	}
}
