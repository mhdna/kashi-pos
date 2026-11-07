package main

import (
	"context"
	"strings"
	"testing"
)

// The real Centro sale SA/CT/2026/02923 on Buy 1 Get 1 with each item's own
// "otherwise" percent: outlet (CA, EOC5) 30%, Bangladesh (BOC5, BNV) 20%.
func mixedFallbackShop(t *testing.T) (dealShop, int64) {
	t.Helper()
	ctx := context.Background()
	app, store, cashbox, client := setUpTestInvoiceApp(t, "http://localhost:9999")
	q := store.Queries
	var prices []syncedPriceListItem
	var discounts []syncedDiscountListItem
	for _, p := range []struct {
		code  string
		price int64
		deal  bool
	}{
		{"BOC5KCTSH12", 1200, true}, {"BOC5KBSWT12", 1200, true}, {"BOC5KBJOP9", 900, true}, {"BOC5KBH15", 1500, true},
		{"CA24KGJK29", 2900, true}, {"BNV5KGSWT15", 1500, true}, {"EOC5KGJ9", 900, true}, {"EOC5KGSWT12", 1200, true},
		{"BNV5MH19", 1900, true}, {"L6LSKE35", 3500, false},
	} {
		if err := applySyncedProductVariant(ctx, q, syncedProductVariant{UID: "var-" + p.code, Barcode: "bc-" + p.code, IsActive: true, Code: p.code, Name: p.code}); err != nil {
			t.Fatal(err)
		}
		prices = append(prices, syncedPriceListItem{ProductVariantUID: "var-" + p.code, Price: p.price})
		if p.deal {
			fb := int64(20)
			if strings.HasPrefix(p.code, "CA") || strings.HasPrefix(p.code, "EOC5") {
				fb = 30
			}
			discounts = append(discounts, syncedDiscountListItem{ProductVariantUID: "var-" + p.code, DealUID: "b1g1", DealFallbackPercent: &fb})
		}
	}
	if err := applySyncedPriceList(ctx, q, syncedPriceList{UID: "p", Name: "P", IsActive: true, Items: prices}); err != nil {
		t.Fatal(err)
	}
	if err := applySyncedDiscountList(ctx, q, syncedDiscountList{UID: "d", Name: "D", IsActive: true, Items: discounts,
		Deals: []syncedDeal{{UID: "b1g1", Name: "Buy 1 Get 1", Kind: dealBuyGet, BuyQty: 1, GetQty: 1, Percent: 100}}}); err != nil {
		t.Fatal(err)
	}
	sp, _ := store.CreateSalesperson(ctx, "C")
	shop := dealShop{app: app, store: store, cashbox: cashbox, client: client, salesperson: sp}
	sale := shop.sell(t, "BOC5KCTSH12", "BOC5KBSWT12", "BOC5KBJOP9", "BOC5KBH15", "CA24KGJK29", "BNV5KGSWT15", "EOC5KGJ9", "EOC5KGSWT12")
	if sale.GrandTotal != 7100 {
		t.Fatalf("sale %d, want 7100", sale.GrandTotal)
	}
	return shop, sale.InvoiceID
}

// The guide's cases, with each item's own percent on: an even number of
// items kept stays in sets (no percent applies); an odd one out keeps its
// percent, so less is charged back.
func TestExchangeMixedFallbackGuideCases(t *testing.T) {
	shop, saleID := mixedFallbackShop(t)
	lines := []exchangeLine{line("CA24KGJK29", -1), line("L6LSKE35", 1)}
	q := quoteExchangeLines(t, shop, saleID, lines)
	if q.Lines[0].ChargedBackCents != 900 || q.GrandTotalCents != 1500 {
		t.Fatalf("charged back %d, pays %d; want 900 and 1500", q.Lines[0].ChargedBackCents, q.GrandTotalCents)
	}
	checkoutQuoted(t, shop, saleID, lines, q)

	shop, saleID = mixedFallbackShop(t)
	lines = []exchangeLine{line("CA24KGJK29", -1), line("BOC5KBH15", -1), line("BNV5MH19", 1), line("CA24KGJK29", 1)}
	q = quoteExchangeLines(t, shop, saleID, lines)
	if q.GrandTotalCents != 400 || q.Lines[0].ChargedBackCents != 0 {
		t.Fatalf("deal swap: pays %d, charged back %d; want 400 and 0", q.GrandTotalCents, q.Lines[0].ChargedBackCents)
	}
	checkoutQuoted(t, shop, saleID, lines, q)

	shop, saleID = mixedFallbackShop(t)
	lines = []exchangeLine{line("CA24KGJK29", -1), line("BOC5KBH15", -1), line("BOC5KBJOP9", -1)}
	q = quoteExchangeLines(t, shop, saleID, lines)
	if q.NetCents != -4100 {
		t.Fatalf("RS/CT/2026/00068: lines %d, want -4100", q.NetCents)
	}
}
