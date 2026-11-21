package main

import (
	"encoding/json"
	db "kashi-pos/db/sqlc"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnknownProductIsCreatedOnceAndPricedWithoutCustomPriceFlags(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	first, err := app.GetUnknownProduct()
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.GetUnknownProduct()
	if err != nil {
		t.Fatal(err)
	}
	if first["id"] != second["id"] || first["barcode"] != defaultUnknownBarcode {
		t.Fatalf("placeholder = %v then %v, want one product with barcode %q", first, second, defaultUnknownBarcode)
	}

	listPricing, err := loadActivePricing(app.ctx, app.store.Queries)
	if err != nil {
		t.Fatal(err)
	}
	// Custom and per-unit pricing are both off: an ordinary product would be
	// refused, the placeholder takes the price typed for each unit.
	items := []salesInvoiceItem{{Code: "UNKNOWN", Barcode: "UNKNOWN", Qty: 2, UnitPricesCents: []int64{1000, 1500}}}
	products, subtotal, _, _, err := app.validateAndPriceItems(app.store.Queries, 0, items, "half_up", false, listPricing, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 2 || subtotal != 2500 {
		t.Fatalf("priced %d lines, subtotal %d; want 2 lines, 2500", len(products), subtotal)
	}
}

func TestUnknownProductStaysOutOfProductSearch(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	if _, err := app.GetUnknownProduct(); err != nil {
		t.Fatal(err)
	}
	page, err := app.ListProductsCursor(0, 20, "")
	if err != nil {
		t.Fatal(err)
	}
	total, err := app.CountProductsForSearch("")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 || total != 0 {
		t.Fatalf("search lists %d items (total %d), want none", len(page.Items), total)
	}
}

func TestVerifyUnknownPicturesNeedsEveryUnit(t *testing.T) {
	complete := false
	central := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/branch/unknown_sessions/status") {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"complete": complete,
			"slots":    []map[string]any{{"slot_ref": "1"}, {"slot_ref": "2"}},
		})
	}))
	defer central.Close()

	app, database := newProductTestApp(t)
	defer database.Close()
	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id, central_api_url, branch_code) VALUES ('Test', 'key', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0, ?, 'B1')`, central.URL); err != nil {
		t.Fatal(err)
	}

	cart := `[{"code":"UNKNOWN","barcode":"UNKNOWN","qty":2},{"code":"A","barcode":"111","qty":1}]`
	if ref, err := app.verifyUnknownPictures(`[{"code":"A","barcode":"111","qty":1}]`); err != nil || ref != "" {
		t.Fatalf("ordinary cart = (%q, %v), want no session needed", ref, err)
	}
	if _, err := app.verifyUnknownPictures(cart); err == nil {
		t.Fatal("unknown items with no session were let through")
	}

	app.unknownSessionRef = "sess-1"
	if _, err := app.verifyUnknownPictures(cart); err == nil {
		t.Fatal("an incomplete session was let through")
	}
	complete = true
	ref, err := app.verifyUnknownPictures(cart)
	if err != nil || ref != "sess-1" {
		t.Fatalf("complete session = (%q, %v), want sess-1", ref, err)
	}
	// The sale screen sends a code and no barcode; that is still unknown.
	if _, err := app.verifyUnknownPictures(`[{"code":"UNKNOWN","qty":3}]`); err == nil {
		t.Fatal("a code-only unknown line skipped the picture check")
	}
	// A session sized for a different cart is refused (a unit was added after).
	if _, err := app.verifyUnknownPictures(`[{"code":"UNKNOWN","barcode":"UNKNOWN","qty":3}]`); err == nil {
		t.Fatal("a session with the wrong number of units was let through")
	}
}

func TestEveryUnknownItemMayBeItsOwnLine(t *testing.T) {
	if _, err := parseSalesItems(`[{"code":"UNKNOWN","barcode":"UNKNOWN","qty":1},{"code":"UNKNOWN","barcode":"UNKNOWN","qty":1}]`, "UNKNOWN", "UNKNOWN"); err != nil {
		t.Fatalf("two unknown lines were refused: %v", err)
	}
	if _, err := parseSalesItems(`[{"code":"UNKNOWN","qty":1},{"code":"UNKNOWN","qty":1}]`, "UNKNOWN", "UNKNOWN"); err != nil {
		t.Fatalf("two unknown lines sent by code were refused: %v", err)
	}
	if _, err := parseSalesItems(`[{"code":"A","barcode":"111","qty":1},{"code":"A","barcode":"111","qty":1}]`, "UNKNOWN", "UNKNOWN"); err == nil {
		t.Fatal("a repeated ordinary product was accepted")
	}
}

func TestUnknownProductCanBeANamedProduct(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if got := app.unknownBarcode(); got != defaultUnknownBarcode {
		t.Fatalf("default unknown barcode = %q", got)
	}
	if err := app.SetUnknownProductBarcode(" 8001 "); err != nil {
		t.Fatal(err)
	}
	if got := app.unknownBarcode(); got != "8001" {
		t.Fatalf("unknown barcode = %q, want 8001", got)
	}
	// Named but not in the catalog yet: refused, not silently invented.
	if _, err := app.GetUnknownProduct(); err == nil {
		t.Fatal("a named product missing from the catalog was accepted")
	}
	if _, err := app.store.CreateProduct(app.ctx, db.CreateProductParams{
		Barcode: "8001", Code: "MYSTERY", Name: "Mystery item", IsActive: 1, CurrencyCode: "USD",
	}); err != nil {
		t.Fatal(err)
	}
	product, err := app.GetUnknownProduct()
	if err != nil || product["code"] != "MYSTERY" {
		t.Fatalf("GetUnknownProduct = (%v, %v), want the named product", product, err)
	}
}


// The reported case: the phone sent the pictures, the till saw them all in,
// then its internet dropped before paying. Checkout lets the sale through on
// what central already said, but only for that session and that many units.
func TestVerifyUnknownPicturesWhenCentralDropsAfterComplete(t *testing.T) {
	complete, down := true, false
	central := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if down {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/branch/unknown_sessions/status") {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"complete": complete,
			"slots":    []map[string]any{{"slot_ref": "1", "has_image": true}, {"slot_ref": "2", "has_image": complete}},
		})
	}))
	defer central.Close()
	app, database := newProductTestApp(t)
	defer database.Close()
	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id, central_api_url, branch_code) VALUES ('Test', 'key', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0, ?, 'B1')`, central.URL); err != nil {
		t.Fatal(err)
	}
	cart := `[{"code":"UNKNOWN","barcode":"UNKNOWN","qty":2}]`
	app.unknownSessionRef = "sess-1"

	// Offline before central ever said complete: refused.
	down = true
	if _, err := app.verifyUnknownPictures(cart); err == nil {
		t.Fatal("let through with no word from central")
	}
	// The sale screen's poll sees every picture in; then the internet drops.
	down = false
	if st, err := app.UnknownItemSessionStatus("sess-1"); err != nil || st["complete"] != true {
		t.Fatalf("status = %v, %v", st, err)
	}
	down = true
	ref, err := app.verifyUnknownPictures(cart)
	if err != nil || ref != "sess-1" {
		t.Fatalf("offline after complete = (%q, %v), want sess-1", ref, err)
	}
	// Still offline, but the cart grew: the pictures central knows don't cover it.
	if _, err := app.verifyUnknownPictures(`[{"code":"UNKNOWN","barcode":"UNKNOWN","qty":3}]`); err == nil {
		t.Fatal("a bigger cart was let through on an old answer")
	}
	// Another cart's session: not trusted.
	app.unknownSessionRef = "sess-2"
	if _, err := app.verifyUnknownPictures(cart); err == nil {
		t.Fatal("another session's answer was trusted")
	}
	// Once the sale is made, the answer is forgotten.
	app.unknownSessionRef = "sess-1"
	app.clearUnknownSession("sess-1")
	app.unknownSessionRef = "sess-1"
	if _, err := app.verifyUnknownPictures(cart); err == nil {
		t.Fatal("a finished sale's answer was reused")
	}
	// Central later says the session isn't complete (a unit came back): that wins.
	down, complete = false, true
	if _, err := app.UnknownItemSessionStatus("sess-1"); err != nil {
		t.Fatal(err)
	}
	complete = false
	if _, err := app.verifyUnknownPictures(cart); err == nil {
		t.Fatal("incomplete was let through")
	}
	down = true
	if _, err := app.verifyUnknownPictures(cart); err == nil {
		t.Fatal("an answer central took back was trusted")
	}
}
