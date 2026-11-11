package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	db "kashi-pos/db/sqlc"
)

// Items whose barcodes aren't printed per colour/size: scanning the barcode
// has to ask which colour/size, per branch (variant_pick_required) and per
// item (products.variant_pick: 'always', 'never', '' follows the branch).

func seedPickProducts(t *testing.T, app *App, rows []db.CreateProductParams) map[string]db.Product {
	t.Helper()
	out := map[string]db.Product{}
	for _, r := range rows {
		if r.CurrencyCode == "" {
			r.CurrencyCode = "USD"
		}
		p, err := app.store.CreateProduct(app.ctx, r)
		if err != nil {
			t.Fatalf("seed %s: %v", r.Barcode, err)
		}
		out[r.Barcode] = p
	}
	priceProductsFromRows(t, app.store.Queries)
	return out
}

func scanBarcodes(t *testing.T, app *App, value string) string {
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

func setItemPick(t *testing.T, app *App, id int64, mode string) {
	t.Helper()
	if err := app.store.SetProductVariantPick(app.ctx, db.SetProductVariantPickParams{VariantPick: mode, ID: id}); err != nil {
		t.Fatal(err)
	}
}

// A shirt in two colours (plus a retired one), the plain item row that holds
// the original barcode, and an unrelated item.
func pickShirts() []db.CreateProductParams {
	return []db.CreateProductParams{
		{Barcode: "BASE", Code: "SHIRT-1", Name: "Shirt", IsActive: 1, Price: 4500},
		{Barcode: "VAR-RED-M", Code: "SHIRT-1", Name: "Shirt", IsActive: 1, Price: 4500, Color: "Red", Size: "M"},
		{Barcode: "VAR-BLUE-L", Code: "SHIRT-1", Name: "Shirt", IsActive: 1, Price: 4500, Color: "Blue", Size: "L"},
		{Barcode: "VAR-OLD", Code: "SHIRT-1", Name: "Shirt", IsActive: 0, Price: 4500, Color: "Green", Size: "M"},
		{Barcode: "HAT", Code: "HAT-1", Name: "Cap", IsActive: 1, Price: 1500},
	}
}

func TestScanBarcodeAsksWhichVariantWhenRequired(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	seedPickProducts(t, app, pickShirts())

	// Off by default: the barcode names exactly one row, as before.
	if got := scanBarcodes(t, app, "BASE"); got != "BASE" {
		t.Fatalf("default: scan(BASE) = %q, want BASE", got)
	}

	if err := app.SetVariantPickRequiredEnabled(true); err != nil {
		t.Fatal(err)
	}
	// Any barcode of the item opens the picker over the active colour/size
	// variants: not the plain row, not the retired colour.
	for _, bc := range []string{"BASE", "VAR-RED-M", "VAR-BLUE-L"} {
		if got := scanBarcodes(t, app, bc); got != "VAR-RED-M,VAR-BLUE-L" {
			t.Errorf("required: scan(%s) = %q, want both active variants", bc, got)
		}
	}
	// An item with no variants is sold as scanned.
	if got := scanBarcodes(t, app, "HAT"); got != "HAT" {
		t.Errorf("required: scan(HAT) = %q, want HAT", got)
	}
	// Typing the code is the existing path and offers every active row.
	if got := scanBarcodes(t, app, "SHIRT-1"); got != "BASE,VAR-RED-M,VAR-BLUE-L" {
		t.Errorf("required: scan(SHIRT-1) = %q", got)
	}

	if err := app.SetVariantPickRequiredEnabled(false); err != nil {
		t.Fatal(err)
	}
	if got := scanBarcodes(t, app, "VAR-RED-M"); got != "VAR-RED-M" {
		t.Errorf("turned back off: scan(VAR-RED-M) = %q, want VAR-RED-M", got)
	}
}

func TestItemVariantPickOverridesTheBranchSetting(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	items := seedPickProducts(t, app, pickShirts())

	// Branch off, item says always.
	setItemPick(t, app, items["VAR-RED-M"].ID, "always")
	if got := scanBarcodes(t, app, "VAR-RED-M"); got != "VAR-RED-M,VAR-BLUE-L" {
		t.Errorf("item always, branch off: got %q", got)
	}
	// The override is per row: another row of the same code still follows
	// the branch (kashi sends it on every variant, so in practice all agree).
	if got := scanBarcodes(t, app, "VAR-BLUE-L"); got != "VAR-BLUE-L" {
		t.Errorf("other row follows the branch: got %q", got)
	}

	// Branch on, item says never.
	if err := app.SetVariantPickRequiredEnabled(true); err != nil {
		t.Fatal(err)
	}
	setItemPick(t, app, items["VAR-RED-M"].ID, "never")
	if got := scanBarcodes(t, app, "VAR-RED-M"); got != "VAR-RED-M" {
		t.Errorf("item never, branch on: got %q", got)
	}
	// Back to following the branch.
	setItemPick(t, app, items["VAR-RED-M"].ID, "")
	if got := scanBarcodes(t, app, "VAR-RED-M"); got != "VAR-RED-M,VAR-BLUE-L" {
		t.Errorf("item follows, branch on: got %q", got)
	}
}

func TestVariantPickEdges(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	seedPickProducts(t, app, []db.CreateProductParams{
		// Only one variant, and it is the one scanned: nothing to choose.
		{Barcode: "ONE-V", Code: "ONE", Name: "One", IsActive: 1, Price: 100, Color: "Red"},
		// The plain row is scanned but the item has a single variant: ask.
		{Barcode: "TWO-BASE", Code: "TWO", Name: "Two", IsActive: 1, Price: 100},
		{Barcode: "TWO-V", Code: "TWO", Name: "Two", IsActive: 1, Price: 100, Size: "L"},
		// Size-only and colour-only variants both count.
		{Barcode: "THREE-C", Code: "THREE", Name: "Three", IsActive: 1, Price: 100, Color: "Red"},
		{Barcode: "THREE-S", Code: "THREE", Name: "Three", IsActive: 1, Price: 100, Size: "S"},
		// No code: nothing to group by.
		{Barcode: "NOCODE-A", Code: "", Name: "A", IsActive: 1, Price: 100, Color: "Red"},
		{Barcode: "NOCODE-B", Code: "", Name: "B", IsActive: 1, Price: 100, Color: "Blue"},
		// A code in another case belongs to the same item.
		{Barcode: "CASE-A", Code: "Case-1", Name: "C", IsActive: 1, Price: 100, Color: "Red"},
		{Barcode: "CASE-B", Code: "CASE-1", Name: "C", IsActive: 1, Price: 100, Color: "Blue"},
	})
	if err := app.SetVariantPickRequiredEnabled(true); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ scan, want string }{
		{"ONE-V", "ONE-V"},
		{"TWO-BASE", "TWO-V"},
		{"TWO-V", "TWO-V"},
		{"THREE-C", "THREE-C,THREE-S"},
		{"NOCODE-A", "NOCODE-A"},
		{"NOCODE-B", "NOCODE-B"},
		{"CASE-A", "CASE-A,CASE-B"},
	} {
		if got := scanBarcodes(t, app, tc.scan); got != tc.want {
			t.Errorf("scan(%s) = %q, want %q", tc.scan, got, tc.want)
		}
	}
}

// Over many random catalogues the picker's answer always obeys the same
// rules, whatever the mix of variants, plain rows, retired rows and settings.
func TestVariantPickRandomCatalogues(t *testing.T) {
	rng := rand.New(rand.NewSource(20261111))
	for round := 0; round < 150; round++ {
		app, database := newProductTestApp(t)
		var rows []db.CreateProductParams
		n := 0
		for c := 0; c < 1+rng.Intn(3); c++ {
			for r := 0; r < 1+rng.Intn(5); r++ {
				n++
				active := int64(1)
				if rng.Intn(4) == 0 { // a retired row now and then
					active = 0
				}
				row := db.CreateProductParams{
					Barcode: fmt.Sprintf("B%d-%d", round, n), Code: fmt.Sprintf("C%d", c),
					Name: "x", IsActive: active, Price: 100,
				}
				switch rng.Intn(4) {
				case 0: // plain row
				case 1:
					row.Color = "Red"
				case 2:
					row.Size = "M"
				default:
					row.Color, row.Size = "Blue", "L"
				}
				rows = append(rows, row)
			}
		}
		items := seedPickProducts(t, app, rows)
		branchOn := rng.Intn(2) == 0
		if err := app.SetVariantPickRequiredEnabled(branchOn); err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			mode := []string{"", "always", "never"}[rng.Intn(3)]
			setItemPick(t, app, items[r.Barcode].ID, mode)
			items[r.Barcode] = db.Product{ID: items[r.Barcode].ID, VariantPick: mode}
		}
		for _, r := range rows {
			scanned := items[r.Barcode]
			ask := branchOn
			if scanned.VariantPick == "always" {
				ask = true
			} else if scanned.VariantPick == "never" {
				ask = false
			}
			got := strings.Split(scanBarcodes(t, app, r.Barcode), ",")
			if !ask {
				if len(got) != 1 || got[0] != r.Barcode {
					t.Fatalf("round %d: %s not asking, got %v", round, r.Barcode, got)
				}
				continue
			}
			var variants []string
			for _, s := range rows {
				if s.Code == r.Code && s.IsActive == 1 && (s.Color != "" || s.Size != "") {
					variants = append(variants, s.Barcode)
				}
			}
			want := []string{r.Barcode}
			if len(variants) > 1 || (len(variants) == 1 && variants[0] != r.Barcode) {
				want = variants
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("round %d: scan(%s) = %v, want %v (rows %+v)", round, r.Barcode, got, want, rows)
			}
		}
		database.Close()
	}
}

// kashi's per-item value lands on the till's product row on pull; an older
// kashi that doesn't send it leaves what is there alone.
func TestSyncPullAppliesVariantPick(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "http://localhost:9999")
	pick := func(s string) *string { return &s }
	v := syncedProductVariant{Barcode: "9990001", Code: "VP-1", Name: "Shirt", IsActive: true, ColorName: sql.NullString{String: "Red", Valid: true}}
	get := func() string {
		p, err := store.GetProductByBarcode(app.ctx, v.Barcode)
		if err != nil {
			t.Fatal(err)
		}
		return p.VariantPick
	}
	apply := func() {
		t.Helper()
		if err := applySyncedProductVariant(app.ctx, store.Queries, v); err != nil {
			t.Fatal(err)
		}
	}

	apply()
	if got := get(); got != "" {
		t.Fatalf("a new product follows the branch, got %q", got)
	}
	v.VariantPick = pick("always")
	apply()
	if got := get(); got != "always" {
		t.Fatalf("after always: %q", got)
	}
	v.VariantPick = nil // an older kashi
	v.Name = "Shirt (renamed)"
	apply()
	if got := get(); got != "always" {
		t.Fatalf("an absent value must leave it alone, got %q", got)
	}
	v.VariantPick = pick("never")
	apply()
	if got := get(); got != "never" {
		t.Fatalf("after never: %q", got)
	}
	v.VariantPick = pick("")
	apply()
	if got := get(); got != "" {
		t.Fatalf("an empty value clears it, got %q", got)
	}

	// Straight from kashi's JSON, as the feed sends it.
	var fromJSON syncedProductVariant
	if err := json.Unmarshal([]byte(`{"barcode":"x","variant_pick":"always"}`), &fromJSON); err != nil || fromJSON.VariantPick == nil || *fromJSON.VariantPick != "always" {
		t.Fatalf("decode: %v %+v", err, fromJSON.VariantPick)
	}
	if err := json.Unmarshal([]byte(`{"barcode":"x"}`), &fromJSON); err != nil {
		t.Fatal(err)
	}
}

// The setting travels both ways: kashi's update_settings command turns it
// on, and the till reports it back in its settings push.
func TestVariantPickRequiredSettingRoundTrips(t *testing.T) {
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/branch/settings" {
			body, _ = io.ReadAll(r.Body)
		}
		w.Write([]byte(`{}`))
	}))
	defer server.Close()
	app, _, _, _ := setUpTestInvoiceApp(t, server.URL)

	report := func() branchSettingsPayload {
		t.Helper()
		if err := app.pushSettings(app.ctx, newCentralClient(server.URL, "TEST01", "test-key")); err != nil {
			t.Fatal(err)
		}
		var got branchSettingsPayload
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	if report().VariantPickRequired {
		t.Fatal("a till that never had the setting reports it off")
	}

	on := true
	str := func(v string) *string { return &v }
	rate := int64(1)
	// Every command carries kashi's core settings alongside what changed.
	command := func(p updateSettingsCommandPayload) []byte {
		p.BranchName, p.RoundingMode, p.RoundingCurrency, p.ExchangeRate = str("Test"), str("half_up"), str("USD"), &rate
		raw, _ := json.Marshal(p)
		return raw
	}
	raw := command(updateSettingsCommandPayload{VariantPickRequired: &on})
	if err := app.executeUpdateSettingsCommand(app.ctx, raw); err != nil {
		t.Fatal(err)
	}
	if !report().VariantPickRequired {
		t.Fatal("update_settings turned it on, but the till reports it off")
	}
	// A command that carries something else leaves it alone.
	raw = command(updateSettingsCommandPayload{ScanByCodeEnabled: &on})
	if err := app.executeUpdateSettingsCommand(app.ctx, raw); err != nil {
		t.Fatal(err)
	}
	if !report().VariantPickRequired {
		t.Fatal("a command without the field must not change it")
	}
	settings, err := app.GetAppSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings["variantPickRequired"] != true {
		t.Fatalf("the settings screen says %v, want true", settings["variantPickRequired"])
	}
}
