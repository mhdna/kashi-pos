package main

import (
	"testing"

	db "kashi-pos/db/sqlc"
)

func seedSearchProducts(t *testing.T, app *App) {
	t.Helper()
	rows := []db.CreateProductParams{
		{Barcode: "B-001", Code: "AB12", Name: "Blue Shirt", IsActive: 1, Price: 100, CurrencyCode: "USD"},
		{Barcode: "B-002", Code: "AB13", Name: "Red SHIRT", IsActive: 1, Price: 100, CurrencyCode: "USD"},
		{Barcode: "B-003", Code: "CD20", Name: "Denim Jacket", IsActive: 1, Price: 100, CurrencyCode: "USD"},
		{Barcode: "B-004", Code: "CD21", Name: "Old Shirt", IsActive: 0, Price: 100, CurrencyCode: "USD"},
		{Barcode: "UNKNOWN", Code: "ZZ99", Name: "Unknown Shirt", IsActive: 1, Price: 100, CurrencyCode: "USD"},
		{Barcode: "7290001", Code: "EF30", Name: "Scarf 50% wool", IsActive: 1, Price: 100, CurrencyCode: "USD"},
	}
	for _, r := range rows {
		if _, err := app.store.CreateProduct(app.ctx, r); err != nil {
			t.Fatal(err)
		}
	}
}

func ids(products []db.Product) []int64 {
	out := make([]int64, len(products))
	for i, p := range products {
		out[i] = p.ID
	}
	return out
}

func sameIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The index must return exactly what the LIKE queries return.
func TestIndexedSearchMatchesLikeQueries(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	seedSearchProducts(t, app)

	terms := []string{
		"shirt", "SHIRT", "Blue", "ab1", "AB12", "shirt ab", // spans name and code
		"B-00", "7290", "50%", "denim j", "nothing-like-this", "scarf 50",
	}
	for _, term := range terms {
		pattern := "%" + term + "%"
		for _, codeOnly := range []bool{false, true} {
			var want []db.Product
			var err error
			if codeOnly {
				want, err = app.store.ListProductsAfterByCode(app.ctx, db.ListProductsAfterByCodeParams{AfterID: 0, SearchPattern: pattern, Limit: 50})
			} else {
				want, err = app.store.ListProductsAfter(app.ctx, db.ListProductsAfterParams{AfterID: 0, SearchPattern: pattern, Limit: 50})
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := app.listProductsAfterIndexed(app.ctx, 0, pattern, 50, codeOnly)
			if err != nil {
				t.Fatal(err)
			}
			if !sameIDs(ids(got), ids(want)) {
				t.Errorf("term %q codeOnly=%v: indexed %v, LIKE %v", term, codeOnly, ids(got), ids(want))
			}

			var wantCount int64
			if codeOnly {
				wantCount, err = app.store.CountProductsByCode(app.ctx, pattern)
			} else {
				wantCount, err = app.store.CountProducts(app.ctx, pattern)
			}
			if err != nil {
				t.Fatal(err)
			}
			gotCount, err := app.countProductsIndexed(app.ctx, pattern, codeOnly)
			if err != nil {
				t.Fatal(err)
			}
			if gotCount != wantCount {
				t.Errorf("term %q codeOnly=%v: indexed count %d, LIKE count %d", term, codeOnly, gotCount, wantCount)
			}
		}
	}
}

func TestIndexedSearchSkipsInactiveAndUnknownAndPages(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	seedSearchProducts(t, app)

	first, err := app.listProductsAfterIndexed(app.ctx, 0, "%shirt%", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 { // Blue Shirt, Red SHIRT; not the inactive or UNKNOWN ones
		t.Fatalf("got %d shirts, want 2: %v", len(first), ids(first))
	}
	rest, err := app.listProductsAfterIndexed(app.ctx, first[0].ID, "%shirt%", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rest) != 1 || rest[0].ID != first[1].ID {
		t.Fatalf("paging after the first shirt = %v, want only the second", ids(rest))
	}
}

func TestCodeOnlySearchNeverMatchesTheName(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	seedSearchProducts(t, app)

	byName, err := app.listProductsAfterIndexed(app.ctx, 0, "%denim%", 50, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(byName) != 0 {
		t.Fatalf("code-only search matched a name: %v", ids(byName))
	}
	byCode, err := app.listProductsAfterIndexed(app.ctx, 0, "%CD20%", 50, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(byCode) != 1 {
		t.Fatalf("code-only search by code = %v, want 1", ids(byCode))
	}
}

// The triggers keep the index in step with every writer.
func TestSearchIndexFollowsInsertUpdateAndDelete(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	count := func(term string) int64 {
		n, err := app.countProductsIndexed(app.ctx, "%"+term+"%", false)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	if _, err := database.Exec(`INSERT INTO products (barcode, code, name, description, is_active, price, discount) VALUES ('B-9', 'XY77', 'Wool Coat', '', 1, 100, 0)`); err != nil {
		t.Fatal(err)
	}
	if count("wool") != 1 || count("xy77") != 1 {
		t.Fatal("a new product is not in the index")
	}
	if _, err := database.Exec(`UPDATE products SET name = 'Linen Coat' WHERE barcode = 'B-9'`); err != nil {
		t.Fatal(err)
	}
	if count("wool") != 0 || count("linen") != 1 {
		t.Fatal("a rename did not reach the index")
	}
	if _, err := database.Exec(`UPDATE products SET barcode = 'B-10', code = 'XY78' WHERE barcode = 'B-9'`); err != nil {
		t.Fatal(err)
	}
	if count("xy77") != 0 || count("xy78") != 1 || count("b-10") != 1 {
		t.Fatal("a code/barcode change did not reach the index")
	}
	if _, err := database.Exec(`DELETE FROM products WHERE barcode = 'B-10'`); err != nil {
		t.Fatal(err)
	}
	if count("linen") != 0 {
		t.Fatal("a deleted product is still in the index")
	}
}

// Through the real entry points: 3+ characters use the index, 1-2 the LIKE
// fallback, and both give the right rows.
func TestCursorSearchShortAndLongTerms(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	seedSearchProducts(t, app)
	priceProductsFromRows(t, app.store.Queries)

	for _, tc := range []struct {
		term string
		want int
	}{
		{"shirt", 2}, {"sh", 2}, {"ab", 2}, {"b-001", 1}, {"zzzz", 0},
	} {
		page, err := app.ListProductsCursor(0, 20, tc.term)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != tc.want {
			t.Errorf("cursor %q = %d items, want %d", tc.term, len(page.Items), tc.want)
		}
		total, err := app.CountProductsForSearch(tc.term)
		if err != nil {
			t.Fatal(err)
		}
		if int(total) != tc.want {
			t.Errorf("count %q = %d, want %d", tc.term, total, tc.want)
		}
	}
}
