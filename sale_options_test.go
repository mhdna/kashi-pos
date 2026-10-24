package main

import (
	"context"
	"testing"
)

// A product that asks Summer/Winter, a list giving it 20% plain and 50% as
// Summer: the option picks the discount, the receipt names it, and an option
// the product doesn't ask falls back to the plain discount.
func TestSaleOptionPicksTheDiscount(t *testing.T) {
	ctx := context.Background()
	app, store, _, _ := setUpTestInvoiceApp(t, "http://localhost:9999")
	q := store.Queries

	v := syncedProductVariant{
		UID: "var-eoc5", Barcode: "8000000777001", IsActive: true,
		Code: "EOC5LAN19", Name: "Lady Anorak",
		SaleOptions: []string{"Summer", "Winter"},
	}
	if err := applySyncedProductVariant(ctx, q, v); err != nil {
		t.Fatal(err)
	}
	if err := applySyncedPriceList(ctx, q, syncedPriceList{
		UID: "price-test", Name: "Test prices", IsActive: true,
		Items: []syncedPriceListItem{{ProductVariantUID: "var-eoc5", Price: 1900}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := applySyncedDiscountList(ctx, q, syncedDiscountList{
		UID: "disc-centro", Name: "Centro", IsActive: true,
		Items: []syncedDiscountListItem{{ProductVariantUID: "var-eoc5", Discount: 20}},
		OptionItems: []syncedDiscountListOptionItem{
			{ProductVariantUID: "var-eoc5", SaleOption: "Summer", Discount: 50},
		},
	}); err != nil {
		t.Fatal(err)
	}

	pricing, err := loadActivePricing(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	product, err := q.GetProductByBarcode(ctx, v.Barcode)
	if err != nil {
		t.Fatal(err)
	}
	options := saleOptionsMap(product, pricing)
	if len(options) != 2 || options[0]["name"] != "Summer" || options[0]["discount"] != int64(50) ||
		options[1]["name"] != "Winter" || options[1]["discount"] != int64(20) {
		t.Fatalf("scan should offer Summer 50%% then Winter 20%% (plain): %+v", options)
	}

	cases := []struct {
		option, receipt string
		discount        int64
	}{
		{"Summer", "Lady Anorak (Summer)", 50},
		{"Winter", "Lady Anorak (Winter)", 20},
		{"Spring", "Lady Anorak", 20}, // not an option of this product
		{"", "Lady Anorak", 20},
	}
	for _, c := range cases {
		priced, _, _, receipt, err := app.validateAndPriceItems(q, 0,
			[]salesInvoiceItem{{Barcode: v.Barcode, Qty: 1, SaleOption: c.option}}, "half_up", false, pricing, false)
		if err != nil {
			t.Fatalf("%q: %v", c.option, err)
		}
		if priced[0].discount != c.discount || receipt[0].Name != c.receipt {
			t.Fatalf("%q: got %d%% %q, want %d%% %q", c.option, priced[0].discount, receipt[0].Name, c.discount, c.receipt)
		}
	}

	// kashi takes the options away: an empty array clears them.
	v.SaleOptions = []string{}
	if err := applySyncedProductVariant(ctx, q, v); err != nil {
		t.Fatal(err)
	}
	pricing, err = loadActivePricing(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if got := saleOptionsMap(product, pricing); len(got) != 0 {
		t.Fatalf("options not cleared: %+v", got)
	}

	// A list re-sent without its option rows drops them.
	if err := applySyncedDiscountList(ctx, q, syncedDiscountList{
		UID: "disc-centro", Name: "Centro", IsActive: true,
		Items: []syncedDiscountListItem{{ProductVariantUID: "var-eoc5", Discount: 20}},
	}); err != nil {
		t.Fatal(err)
	}
	pricing, err = loadActivePricing(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if got := pricing.optionDiscounts["var-eoc5"]; len(got) != 0 {
		t.Fatalf("option rows survived a re-send without them: %+v", got)
	}
}
