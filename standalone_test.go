package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	db "kashi-pos/db/sqlc"
)

func TestStandaloneProductLifecycle(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "")

	created, err := app.CreateProduct(ProductInput{Barcode: " NEW-1 ", Name: "Scarf", PriceCents: 1500, IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	id := created["id"].(int64)
	if created["uid"] == "" || created["code"] != "NEW-1" {
		t.Fatalf("expected a uid and code defaulted to barcode, got %+v", created)
	}
	if _, err := app.CreateProduct(ProductInput{Barcode: "NEW-1", Name: "Dup", PriceCents: 100}); err == nil {
		t.Fatal("duplicate barcode must be rejected")
	}

	if _, err := app.UpdateProduct(id, ProductInput{Barcode: "NEW-1", Name: "Wool Scarf", PriceCents: 1800, IsActive: true}); err != nil {
		t.Fatal(err)
	}
	if p, _ := store.GetProductByID(app.ctx, id); p.Name != "Wool Scarf" || p.Price != 1800 {
		t.Fatalf("update not applied: %+v", p)
	}

	outcome, err := app.DeleteProduct(id)
	if err != nil || outcome != "deleted" {
		t.Fatalf("unreferenced product should be deleted, got %q err=%v", outcome, err)
	}
	if _, err := store.GetProductByID(app.ctx, id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("product should be gone, err=%v", err)
	}
}

// A product or client that history refers to is deactivated, not deleted.
func TestStandaloneDeleteDeactivatesReferencedRows(t *testing.T) {
	app, store, cashbox, client := setUpTestInvoiceApp(t, "")
	items, _ := json.Marshal([]map[string]any{{"code": "SKU-1", "qty": 1}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: 100}})
	if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: client.ID}); err != nil {
		t.Fatal(err)
	}
	product, err := store.GetProductByBarcode(app.ctx, "1")
	if err != nil {
		t.Fatal(err)
	}

	if outcome, err := app.DeleteProduct(product.ID); err != nil || outcome != "deactivated" {
		t.Fatalf("sold product should be deactivated, got %q err=%v", outcome, err)
	}
	if p, _ := store.GetProductByID(app.ctx, product.ID); p.IsActive != 0 {
		t.Fatal("expected product inactive")
	}

	if outcome, err := app.DeleteClient(client.ID); err != nil || outcome != "deactivated" {
		t.Fatalf("client with invoices should be deactivated, got %q err=%v", outcome, err)
	}
	if c, _ := store.GetClientByID(app.ctx, client.ID); c.IsActive != 0 {
		t.Fatal("expected client inactive")
	}
	if err := app.SetClientActive(client.ID, true); err != nil {
		t.Fatal(err)
	}
	if c, _ := store.GetClientByID(app.ctx, client.ID); c.IsActive != 1 {
		t.Fatal("expected client reactivated")
	}

	fresh, err := store.CreateClient(app.ctx, db.CreateClientParams{Name: "Never Bought", Phone: "70111222"})
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := app.DeleteClient(fresh.ID); err != nil || outcome != "deleted" {
		t.Fatalf("client with no history should be deleted, got %q err=%v", outcome, err)
	}
}

// While connected, kashi owns products and client removal.
func TestConnectedModeRefusesLocalCatalogEdits(t *testing.T) {
	app, _, _, client := setUpTestInvoiceApp(t, "http://kashi.invalid")

	if _, err := app.CreateProduct(ProductInput{Barcode: "X", Name: "X", PriceCents: 100}); !errors.Is(err, errManagedByKashi) {
		t.Fatalf("CreateProduct: expected errManagedByKashi, got %v", err)
	}
	if _, err := app.DeleteClient(client.ID); !errors.Is(err, errManagedByKashi) {
		t.Fatalf("DeleteClient: expected errManagedByKashi, got %v", err)
	}
	if standalone, _ := app.IsStandaloneMode(); standalone {
		t.Fatal("a till with a central URL is not standalone")
	}
}
