package main

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func TestListProductsPageBoundsAndSearch(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	insertTestProducts(t, database, 150)

	first, err := app.ListProductsPage(1, 5, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 5 || first.Total != 150 {
		t.Fatalf("first page has %d items and total %d, want 5 and 150", len(first.Items), first.Total)
	}
	if first.Items[0]["code"] != "SKU-000" || first.Items[4]["code"] != "SKU-004" {
		t.Fatalf("unexpected first page: %#v", first.Items)
	}

	second, err := app.ListProductsPage(2, 5, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 5 || second.Items[0]["code"] != "SKU-005" {
		t.Fatalf("unexpected second page: %#v", second.Items)
	}

	defaultSize, err := app.ListProductsPage(1, 0, "  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(defaultSize.Items) != 5 {
		t.Fatalf("default page has %d items, want 5", len(defaultSize.Items))
	}

	maxSize, err := app.ListProductsPage(1, 1_000, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(maxSize.Items) != 100 {
		t.Fatalf("capped page has %d items, want 100", len(maxSize.Items))
	}

	filtered, err := app.ListProductsPage(1, 5, " SKU-011 ")
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Items) != 1 || filtered.Total != 1 || filtered.Items[0]["code"] != "SKU-011" {
		t.Fatalf("unexpected filtered page: %#v (total %d)", filtered.Items, filtered.Total)
	}
}

func TestListProductsCursorUsesStableBatches(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	insertTestProducts(t, database, 45)

	first, err := app.ListProductsCursor(0, 20, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 20 || !first.HasMore || first.NextCursor <= 0 {
		t.Fatalf("unexpected first cursor page: %#v", first)
	}
	if first.Items[0]["code"] != "SKU-000" || first.Items[19]["code"] != "SKU-019" {
		t.Fatalf("unexpected first cursor items: %#v", first.Items)
	}

	second, err := app.ListProductsCursor(first.NextCursor, 20, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 20 || !second.HasMore || second.Items[0]["code"] != "SKU-020" {
		t.Fatalf("unexpected second cursor page: %#v", second)
	}

	last, err := app.ListProductsCursor(second.NextCursor, 20, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Items) != 5 || last.HasMore || last.Items[0]["code"] != "SKU-040" {
		t.Fatalf("unexpected final cursor page: %#v", last)
	}

	filtered, err := app.ListProductsCursor(0, 20, " SKU-011 ")
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Items) != 1 || filtered.HasMore || filtered.Items[0]["code"] != "SKU-011" {
		t.Fatalf("unexpected filtered cursor page: %#v", filtered)
	}
}

func TestProductSearchMatchesNameCodeAndBarcodeOnly(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	insertTestProducts(t, database, 5)

	if _, err := app.store.CreateProduct(app.ctx, db.CreateProductParams{
		Barcode: "BAR-VARIANT", Code: "SKU-VARIANT", Name: "Denim Jacket",
		IsActive: 1, Price: 4599, Color: "Indigo", Size: "XL", CurrencyCode: "USD",
	}); err != nil {
		t.Fatal(err)
	}
	priceProductsFromRows(t, app.store.Queries)

	matches := []struct {
		name  string
		query string
	}{
		{"name", "Denim"},
		{"code", "SKU-VARIANT"},
		{"barcode", "BAR-VARIANT"},
	}
	for _, tc := range matches {
		t.Run(tc.name, func(t *testing.T) {
			page, err := app.ListProductsPage(1, 10, tc.query)
			if err != nil {
				t.Fatal(err)
			}
			if len(page.Items) != 1 || page.Items[0]["code"] != "SKU-VARIANT" {
				t.Fatalf("search %q = %#v, want the Denim Jacket variant", tc.query, page.Items)
			}

			cursor, err := app.ListProductsCursor(0, 20, tc.query)
			if err != nil {
				t.Fatal(err)
			}
			if len(cursor.Items) != 1 || cursor.Items[0]["code"] != "SKU-VARIANT" {
				t.Fatalf("cursor search %q = %#v, want the Denim Jacket variant", tc.query, cursor.Items)
			}
		})
	}

	nonMatches := []struct {
		name  string
		query string
	}{
		{"color", "Indigo"},
		{"size", "XL"},
		{"price", "45.99"},
	}
	for _, tc := range nonMatches {
		t.Run("no_match_"+tc.name, func(t *testing.T) {
			page, err := app.ListProductsPage(1, 10, tc.query)
			if err != nil {
				t.Fatal(err)
			}
			if len(page.Items) != 0 {
				t.Fatalf("search %q = %#v, want no matches (color/size/price aren't searched)", tc.query, page.Items)
			}
		})
	}
}

func TestQuickAccessProductsPersistInDatabase(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	insertTestProducts(t, database, 2)

	product, err := app.store.GetProductByCode(app.ctx, "SKU-001")
	if err != nil {
		t.Fatal(err)
	}
	if err := app.AddQuickAccessProduct(product.ID); err != nil {
		t.Fatal(err)
	}
	items, err := app.ListQuickAccessProducts()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0]["code"] != "SKU-001" {
		t.Fatalf("quick-access items = %#v", items)
	}

	if err := app.RemoveQuickAccessProduct(product.ID); err != nil {
		t.Fatal(err)
	}
	items, err = app.ListQuickAccessProducts()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("quick-access items after removal = %#v", items)
	}
}

func TestProductVariantsHaveDistinctBarcodesAndCatalogMetadata(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	// brand/family/kind/season/product_year are still legacy columns here
	// (admin_server.go and tools/kashi-admin still write them), but the
	// main app now sources this metadata from the attribute system
	// instead (see 000073_create_product_attributes.up.sql) — so a
	// realistic test product needs matching products_attributes rows, not
	// just the legacy columns, for GetProductByBarcode to surface them.
	_, err := database.Exec(`
		INSERT INTO products (
			barcode, code, name, description, is_active, price, discount,
			color, size, currency_code, brand, family, kind, season, product_year
		) VALUES
			('VAR-RED-S', 'SHIRT-1', 'Oxford Shirt', 'Shirts', 1, 4500, 0,
			 'Red', 'S', 'USD', 'Kashi', 'Oxford', 'Shirt', 'Summer', 2026),
			('VAR-BLUE-L', 'SHIRT-1', 'Oxford Shirt', 'Shirts', 1, 4500, 0,
			 'Blue', 'L', 'USD', 'Kashi', 'Oxford', 'Shirt', 'Summer', 2026)
	`)
	if err != nil {
		t.Fatal(err)
	}
	for _, barcode := range []string{"VAR-RED-S", "VAR-BLUE-L"} {
		_, err := database.Exec(`
			INSERT INTO attributes_values (attribute_id, value)
			SELECT 2, 'Kashi' WHERE NOT EXISTS (SELECT 1 FROM attributes_values WHERE attribute_id = 2 AND value = 'Kashi')
			UNION ALL SELECT 1, 'Oxford' WHERE NOT EXISTS (SELECT 1 FROM attributes_values WHERE attribute_id = 1 AND value = 'Oxford')
			UNION ALL SELECT 3, 'Shirt' WHERE NOT EXISTS (SELECT 1 FROM attributes_values WHERE attribute_id = 3 AND value = 'Shirt')
			UNION ALL SELECT 7, 'Summer' WHERE NOT EXISTS (SELECT 1 FROM attributes_values WHERE attribute_id = 7 AND value = 'Summer')
			UNION ALL SELECT 6, '2026' WHERE NOT EXISTS (SELECT 1 FROM attributes_values WHERE attribute_id = 6 AND value = '2026')
		`)
		if err != nil {
			t.Fatal(err)
		}
		_, err = database.Exec(`
			INSERT INTO products_attributes (attribute_id, attribute_value_id, product_id)
			SELECT av.attribute_id, av.id, (SELECT id FROM products WHERE barcode = ?)
			FROM attributes_values av
			WHERE (av.attribute_id, av.value) IN ((2,'Kashi'), (1,'Oxford'), (3,'Shirt'), (7,'Summer'), (6,'2026'))
		`, barcode)
		if err != nil {
			t.Fatal(err)
		}
	}

	red, err := app.GetProductByBarcode("VAR-RED-S")
	if err != nil {
		t.Fatal(err)
	}
	blue, err := app.GetProductByBarcode("VAR-BLUE-L")
	if err != nil {
		t.Fatal(err)
	}
	if red["code"] != blue["code"] || red["color"] != "Red" || red["size"] != "S" || blue["color"] != "Blue" || blue["size"] != "L" {
		t.Fatalf("variants were not kept separate: red=%#v blue=%#v", red, blue)
	}
	// year comes back as text now (attributes_values.value is always
	// text, matching kashi's own attribute model), not the int64 the old
	// product_year column held.
	if red["brand"] != "Kashi" || red["family"] != "Oxford" || red["kind"] != "Shirt" || red["season"] != "Summer" || red["year"] != "2026" {
		t.Fatalf("catalog metadata missing from product: %#v", red)
	}

	page, err := app.ListProductsPage(1, 10, "Blue")
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0]["barcode"] != "VAR-BLUE-L" {
		t.Fatalf("color search did not select the exact variant: %#v", page)
	}
}

func BenchmarkListProductsPage(b *testing.B) {
	app, database := newProductBenchmarkApp(b)
	defer database.Close()
	insertBenchmarkProducts(b, database, 10_000)

	b.Run("unfiltered", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := app.ListProductsPage(1, 5, ""); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("search", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := app.ListProductsPage(1, 5, "Product 09999"); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func newProductTestApp(t *testing.T) (*App, *sql.DB) {
	t.Helper()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "products.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(context.Background(), database, "", false); err != nil {
		database.Close()
		t.Fatal(err)
	}
	return &App{ctx: context.Background(), store: db.NewStore(database)}, database
}

func newProductBenchmarkApp(b *testing.B) (*App, *sql.DB) {
	b.Helper()
	database, err := sql.Open("sqlite", filepath.Join(b.TempDir(), "products.db"))
	if err != nil {
		b.Fatal(err)
	}
	if err := applyMigrations(context.Background(), database, "", false); err != nil {
		database.Close()
		b.Fatal(err)
	}
	return &App{ctx: context.Background(), store: db.NewStore(database)}, database
}

func insertTestProducts(t *testing.T, database *sql.DB, count int) {
	t.Helper()
	if err := insertProducts(database, count); err != nil {
		t.Fatal(err)
	}
}

func insertBenchmarkProducts(b *testing.B, database *sql.DB, count int) {
	b.Helper()
	if err := insertProducts(database, count); err != nil {
		b.Fatal(err)
	}
}

func insertProducts(database *sql.DB, count int) error {
	tx, err := database.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO products (barcode, code, name, description, is_active, price, discount)
		VALUES (?, ?, ?, '', 1, 100, 0)
	`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for i := 0; i < count; i++ {
		if _, err := stmt.Exec(
			fmt.Sprintf("BAR-%06d", i),
			fmt.Sprintf("SKU-%03d", i),
			fmt.Sprintf("Product %05d", i),
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// A product kashi has retired (is_active = 0) must not surface in a search.
// The cursor-paged path has always filtered it out; the offset-paged path did
// not, so the same query returned different catalogs depending on which one
// the screen happened to use, and a cashier could add an unsellable product
// to the cart from the search dialog.
func TestProductSearchExcludesInactiveProducts(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	if _, err := app.store.CreateProduct(app.ctx, db.CreateProductParams{
		Barcode: "BAR-LIVE", Code: "SKU-LIVE", Name: "Denim Jacket",
		IsActive: 1, Price: 4599, CurrencyCode: "USD",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.store.CreateProduct(app.ctx, db.CreateProductParams{
		Barcode: "BAR-DEAD", Code: "SKU-DEAD", Name: "Denim Jacket Retired",
		IsActive: 0, Price: 4599, CurrencyCode: "USD",
	}); err != nil {
		t.Fatal(err)
	}
	priceProductsFromRows(t, app.store.Queries)

	page, err := app.ListProductsPage(1, 50, "Denim")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range page.Items {
		if item["code"] == "SKU-DEAD" {
			t.Fatalf("inactive product appeared in search results: %#v", item)
		}
	}
	if len(page.Items) != 1 || page.Items[0]["code"] != "SKU-LIVE" {
		t.Fatalf("expected only the active product, got %#v", page.Items)
	}
	// The count drives the pager, so it has to agree with the rows.
	if page.Total != 1 {
		t.Fatalf("total = %d, want 1 -- the count must match the rows returned", page.Total)
	}

	// And the unfiltered listing (no search term) too.
	all, err := app.ListProductsPage(1, 50, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range all.Items {
		if item["code"] == "SKU-DEAD" {
			t.Fatalf("inactive product appeared in the unfiltered listing: %#v", item)
		}
	}
	if all.Total != 1 {
		t.Fatalf("unfiltered total = %d, want 1", all.Total)
	}
}

// The Products page shows what a sale would charge -- the active price
// list's price -- and "No price" (priced:false) for a product no list prices
// yet, not the product row's frozen price column (usually 0 for a product
// synced from kashi, which is how a priced product used to show 0.00).
func TestListManagedProductsShowsListPrice(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "")
	// SKU-1 is priced at 100 cents by setUpTestInvoiceApp's price list; give
	// its row a different, stale value to prove the list wins.
	if _, err := app.conn.Exec(`UPDATE products SET price = 999 WHERE code = 'SKU-1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateProduct(app.ctx, db.CreateProductParams{
		Barcode: "2", Code: "SKU-2", Name: "Unpriced", IsActive: 1, Price: 0, CurrencyCode: "USD",
	}); err != nil {
		t.Fatal(err)
	}

	page, err := app.ListManagedProducts(0, 50, "")
	if err != nil {
		t.Fatal(err)
	}
	byCode := map[string]map[string]any{}
	for _, item := range page.Items {
		byCode[item["code"].(string)] = item
	}
	if p := byCode["SKU-1"]; p["priced"] != true || p["priceCents"] != int64(100) {
		t.Fatalf("SKU-1: priced=%v priceCents=%v, want true and 100 (the list price)", p["priced"], p["priceCents"])
	}
	if p := byCode["SKU-2"]; p["priced"] != false {
		t.Fatalf("SKU-2 has no list price but priced=%v", p["priced"])
	}
}

// The scan field resolves a barcode to its one product; failing that, a
// product code -- shared by colour/size variants -- returns every active
// variant so the screen can ask which, rather than silently selling the
// oldest one. Turning scan_by_code off limits the field to barcodes.
func TestFindProductsForScan(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	for _, p := range []db.CreateProductParams{
		{Barcode: "BAR-RED", Code: "SHIRT-1", Name: "Oxford Shirt", Color: "Red", IsActive: 1, Price: 4500, CurrencyCode: "USD"},
		{Barcode: "BAR-BLUE", Code: "SHIRT-1", Name: "Oxford Shirt", Color: "Blue", IsActive: 1, Price: 4500, CurrencyCode: "USD"},
		{Barcode: "BAR-OLD", Code: "SHIRT-1", Name: "Oxford Shirt", Color: "Green", IsActive: 0, Price: 4500, CurrencyCode: "USD"},
		{Barcode: "BAR-HAT", Code: "HAT-1", Name: "Cap", IsActive: 1, Price: 1500, CurrencyCode: "USD"},
		// A code that is also another product's barcode: the barcode wins.
		{Barcode: "BAR-SOCK", Code: "BAR-HAT", Name: "Socks", IsActive: 1, Price: 500, CurrencyCode: "USD"},
	} {
		if _, err := app.store.CreateProduct(app.ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	priceProductsFromRows(t, app.store.Queries)

	scan := func(value string) string {
		t.Helper()
		found, err := app.FindProductsForScan(value)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, len(found))
		for i, p := range found {
			out[i], _ = p["barcode"].(string)
		}
		return strings.Join(out, ",")
	}

	cases := []struct{ value, want string }{
		{"BAR-RED", "BAR-RED"},
		{" BAR-RED ", "BAR-RED"},
		{"SHIRT-1", "BAR-RED,BAR-BLUE"}, // inactive variant left out
		{"shirt-1", "BAR-RED,BAR-BLUE"}, // codes match in any case
		{"Hat-1", "BAR-HAT"},
		{"bar-red", ""}, // barcodes stay exact
		{"HAT-1", "BAR-HAT"},
		{"BAR-HAT", "BAR-HAT"},
		{"NOPE", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := scan(tc.value); got != tc.want {
			t.Errorf("scan(%q) = %q, want %q", tc.value, got, tc.want)
		}
	}

	if err := app.SetScanByCodeEnabled(false); err != nil {
		t.Fatal(err)
	}
	if got := scan("SHIRT-1"); got != "" {
		t.Errorf("scan by code while disabled = %q, want nothing", got)
	}
	if got := scan("BAR-BLUE"); got != "BAR-BLUE" {
		t.Errorf("scan by barcode while code disabled = %q, want BAR-BLUE", got)
	}
}

func TestProductSearchCodeOnlyIgnoresNames(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	if _, err := app.store.CreateProduct(app.ctx, db.CreateProductParams{
		Barcode: "BAR-VARIANT", Code: "SKU-VARIANT", Name: "Denim Jacket",
		IsActive: 1, Price: 4599, CurrencyCode: "USD",
	}); err != nil {
		t.Fatal(err)
	}
	priceProductsFromRows(t, app.store.Queries)

	if err := app.SetProductSearchCodeOnly(true); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"Denim", "jacket"} {
		page, err := app.ListProductsCursor(0, 10, query)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 0 || page.Total != 0 {
			t.Fatalf("code-only search %q = %#v (total %d), want nothing", query, page.Items, page.Total)
		}
	}
	for _, query := range []string{"SKU-VARIANT", "BAR-VARIANT"} {
		page, err := app.ListProductsCursor(0, 10, query)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 || page.Total != 1 {
			t.Fatalf("code-only search %q = %#v (total %d), want the variant", query, page.Items, page.Total)
		}
	}
}

func TestNormalizePriceEditCodes(t *testing.T) {
	got := normalizePriceEditCodes(" A1, b2\nA1;C3 \r\n")
	if got != "A1,b2,C3" {
		t.Fatalf("normalizePriceEditCodes = %q, want %q", got, "A1,b2,C3")
	}
}

func TestSetPriceEditCodesRoundTrips(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if err := app.SetPriceEditCodes("A1\nb2"); err != nil {
		t.Fatal(err)
	}
	if got := app.priceEditCodes(); got != "A1,b2" {
		t.Fatalf("priceEditCodes = %q, want A1,b2", got)
	}
}

func TestRefreshCartPricingFollowsTheLists(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	product, err := app.store.CreateProduct(app.ctx, db.CreateProductParams{
		Barcode: "B1", Code: "C1", Name: "Jacket", IsActive: 1, Price: 1200, CurrencyCode: "USD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE products SET uid = 'uid-1' WHERE id = ?`, product.ID); err != nil {
		t.Fatal(err)
	}
	priceProductsFromRows(t, app.store.Queries)

	rows, err := app.RefreshCartPricing(`["B1","missing"]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["price"] != 12.0 || rows[0]["discount"] != int64(0) {
		t.Fatalf("before the discount = %v, want one row at $12 with none", rows)
	}

	// kashi puts the product on the discount list: the next read has it.
	if _, err := database.Exec(`INSERT INTO discount_list_items (discount_list_id, product_uid, discount) SELECT id, 'uid-1', 25 FROM discount_lists WHERE uid = 'test-default-discount-list'`); err != nil {
		t.Fatal(err)
	}
	rows, err = app.RefreshCartPricing(`["B1"]`)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0]["discount"] != int64(25) {
		t.Fatalf("after the discount = %v, want 25%%", rows[0])
	}
}
