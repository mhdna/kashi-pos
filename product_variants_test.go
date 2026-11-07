package main

import (
	"fmt"
	"math/rand"
	"testing"

	db "kashi-pos/db/sqlc"
)

// The product search shows the variants sharing a code as one result and
// asks ListProductVariants for the whole set when one is tapped.

func seedVariantProducts(t *testing.T, app *App, rows []db.CreateProductParams) {
	t.Helper()
	for _, r := range rows {
		if r.CurrencyCode == "" {
			r.CurrencyCode = "USD"
		}
		if _, err := app.store.CreateProduct(app.ctx, r); err != nil {
			t.Fatalf("seed %s: %v", r.Barcode, err)
		}
	}
	priceProductsFromRows(t, app.store.Queries)
}

// The case from the till: kids' casual shoes, one code in 4 colours x 2 sizes.
func kidsShoes() []db.CreateProductParams {
	var rows []db.CreateProductParams
	n := 0
	for _, size := range []string{"20-25", "25-30"} {
		for _, color := range []string{"Grey", "Beige", "White", "Black"} {
			n++
			rows = append(rows, db.CreateProductParams{
				Barcode: fmt.Sprintf("LC29-%02d", n), Code: "C6KSHE29-LC", Name: "Kids Casual shoes",
				IsActive: 1, Price: 2900, Color: color, Size: size,
			})
		}
	}
	return rows
}

func TestListProductVariantsReturnsEveryActiveVariantOfTheCode(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	rows := append(kidsShoes(),
		db.CreateProductParams{Barcode: "LC29-OLD", Code: "C6KSHE29-LC", Name: "Kids Casual shoes", IsActive: 0, Price: 2900, Color: "Red", Size: "20-25"},
		db.CreateProductParams{Barcode: "UNKNOWN", Code: "C6KSHE29-LC", Name: "Unknown", IsActive: 1, Price: 0},
		db.CreateProductParams{Barcode: "LC35-01", Code: "C6KSHE35-LC", Name: "Kids Casual shoes", IsActive: 1, Price: 3500, Color: "Grey", Size: "31-36"},
	)
	seedVariantProducts(t, app, rows)

	got, err := app.ListProductVariants("C6KSHE29-LC")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 8 {
		t.Fatalf("got %d variants, want 8: %v", len(got), got)
	}
	seen := map[string]bool{}
	for i, v := range got {
		if v["code"] != "C6KSHE29-LC" {
			t.Errorf("variant %d has code %v", i, v["code"])
		}
		if v["price"] != 29.0 {
			t.Errorf("variant %d price = %v, want 29 (priced like the search results)", i, v["price"])
		}
		key := fmt.Sprint(v["color"], "/", v["size"])
		if seen[key] {
			t.Errorf("duplicate variant %s", key)
		}
		seen[key] = true
		if v["barcode"] == "LC29-OLD" || v["barcode"] == "UNKNOWN" {
			t.Errorf("inactive or unknown product offered: %v", v["barcode"])
		}
	}
	// In the order they were made, like the search.
	if got[0]["barcode"] != "LC29-01" || got[7]["barcode"] != "LC29-08" {
		t.Errorf("order = %v .. %v, want LC29-01 .. LC29-08", got[0]["barcode"], got[7]["barcode"])
	}
}

func TestListProductVariantsEdges(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	seedVariantProducts(t, app, append(kidsShoes(),
		db.CreateProductParams{Barcode: "SOLO", Code: "SOLO-1", Name: "Single", IsActive: 1, Price: 1000},
	))

	for _, tc := range []struct {
		code string
		want int
	}{
		{"c6kshe29-lc", 8},    // any case, as a cashier types it
		{"  C6KSHE29-LC ", 8}, // stray spaces
		{"SOLO-1", 1},
		{"C6KSHE29", 0}, // a prefix isn't the code
		{"", 0},
		{"   ", 0},
		{"NOPE", 0},
	} {
		got, err := app.ListProductVariants(tc.code)
		if err != nil {
			t.Fatalf("%q: %v", tc.code, err)
		}
		if got == nil {
			t.Errorf("%q: nil, want an empty list for the frontend", tc.code)
		}
		if len(got) != tc.want {
			t.Errorf("%q: %d variants, want %d", tc.code, len(got), tc.want)
		}
	}
}

func TestCountProductGroupsForSearchCountsCodes(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	rows := append(kidsShoes(),
		db.CreateProductParams{Barcode: "LC35-01", Code: "C6KSHE35-LC", Name: "Kids Casual shoes", IsActive: 1, Price: 3500, Color: "Grey", Size: "31-36"},
		db.CreateProductParams{Barcode: "LC35-02", Code: "C6KSHE35-LC", Name: "Kids Casual shoes", IsActive: 1, Price: 3500, Color: "Black", Size: "31-36"},
		db.CreateProductParams{Barcode: "NC-1", Code: "", Name: "No code A", IsActive: 1, Price: 100},
		db.CreateProductParams{Barcode: "NC-2", Code: "", Name: "No code B", IsActive: 1, Price: 100},
		db.CreateProductParams{Barcode: "OFF-1", Code: "OFF", Name: "Kids gone", IsActive: 0, Price: 100},
	)
	seedVariantProducts(t, app, rows)

	for _, tc := range []struct {
		term     string
		codeOnly bool
		want     int64
	}{
		{"", false, 4},        // two shoe codes + two products with no code
		{"kids", false, 2},    // indexed (3+ chars), by name
		{"ki", false, 2},      // short term, LIKE
		{"C6KSHE", false, 2},  // by code
		{"LC29-0", false, 1},  // by barcode: one code, many rows
		{"no code", false, 2}, // each codeless product is its own group
		{"kids", true, 0},     // code-only search ignores names
		{"C6KSHE35", true, 1}, // indexed, code-only
		{"35", true, 1},       // short, code-only
		{"zzzz", false, 0},
	} {
		if err := app.SetProductSearchCodeOnly(tc.codeOnly); err != nil {
			t.Fatal(err)
		}
		got, err := app.CountProductGroupsForSearch(tc.term)
		if err != nil {
			t.Fatalf("%q: %v", tc.term, err)
		}
		if got != tc.want {
			t.Errorf("CountProductGroupsForSearch(%q, codeOnly=%v) = %d, want %d", tc.term, tc.codeOnly, got, tc.want)
		}
	}
}

// Over a random catalogue and many searches, the "N products" label must
// equal the distinct groups the paged search actually returns, and every
// variant ListProductVariants offers for a shown code must exist in the
// catalogue as an active variant of that code.
func TestCountProductGroupsMatchesPagedSearchRandomized(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	rng := rand.New(rand.NewSource(7))
	codes := []string{"AB-100", "AB-101", "CD-200", "kid-29", "KID-35", "ZX9", "Q1", ""}
	colors := []string{"", "Black", "White", "Red/Tra", "Grey"}
	sizes := []string{"", "S", "M", "20-25", "38"}
	names := []string{"Shirt", "Kids shoes", "Scarf", "Jacket"}
	var rows []db.CreateProductParams
	for i := 0; i < 300; i++ {
		active := int64(1)
		if rng.Intn(6) == 0 {
			active = 0
		}
		rows = append(rows, db.CreateProductParams{
			Barcode: fmt.Sprintf("R%04d", i), Code: codes[rng.Intn(len(codes))], Name: names[rng.Intn(len(names))],
			IsActive: active, Price: int64(100 + rng.Intn(5000)),
			Color: colors[rng.Intn(len(colors))], Size: sizes[rng.Intn(len(sizes))],
		})
	}
	seedVariantProducts(t, app, rows)

	terms := []string{"", "a", "ab", "AB-", "ab-10", "kid", "KID-3", "shoes", "sh", "r00", "R01", "zx9", "q", "none", "-"}
	for _, codeOnly := range []bool{false, true} {
		if err := app.SetProductSearchCodeOnly(codeOnly); err != nil {
			t.Fatal(err)
		}
		for _, term := range terms {
			groups := map[string]bool{}
			after := int64(0)
			for {
				page, err := app.ListProductsCursor(after, 17, term)
				if err != nil {
					t.Fatal(err)
				}
				for _, item := range page.Items {
					key := item["code"].(string)
					if key == "" {
						key = fmt.Sprint("id:", item["id"])
					}
					groups[key] = true
				}
				if !page.HasMore {
					break
				}
				after = page.NextCursor
			}
			got, err := app.CountProductGroupsForSearch(term)
			if err != nil {
				t.Fatal(err)
			}
			if int(got) != len(groups) {
				t.Errorf("codeOnly=%v term %q: label says %d products, search shows %d", codeOnly, term, got, len(groups))
			}
			for key := range groups {
				if len(key) > 3 && key[:3] == "id:" {
					continue
				}
				variants, err := app.ListProductVariants(key)
				if err != nil {
					t.Fatal(err)
				}
				want := 0
				for _, r := range rows {
					if r.IsActive == 1 && equalFoldASCII(r.Code, key) {
						want++
					}
				}
				if len(variants) != want {
					t.Errorf("ListProductVariants(%q) = %d, want %d active variants", key, len(variants), want)
				}
			}
		}
	}
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}
