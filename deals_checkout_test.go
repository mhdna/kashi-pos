package main

import (
	"context"
	"encoding/json"
	"testing"

	db "kashi-pos/db/sqlc"
)

// dealShop is a till with shirts A ($100), B ($80) and C ($50) on "Buy 2 get
// 1 free", a jacket ($100) and socks ($20) on no deal, and a salesperson for
// exchanges.
type dealShop struct {
	app         *App
	store       *db.Store
	cashbox     db.CashboxAccount
	client      db.Client
	salesperson db.Salesperson
}

func setUpDealShop(t *testing.T) dealShop {
	t.Helper()
	ctx := context.Background()
	app, store, cashbox, client := setUpTestInvoiceApp(t, "http://localhost:9999")
	q := store.Queries
	products := []struct {
		code  string
		price int64
		deal  bool
	}{
		{"A", 10000, true}, {"B", 8000, true}, {"C", 5000, true}, {"D", 9000, true},
		{"JACKET", 10000, false}, {"SOCKS", 2000, false},
	}
	var prices []syncedPriceListItem
	var discounts []syncedDiscountListItem
	for _, p := range products {
		if err := applySyncedProductVariant(ctx, q, syncedProductVariant{
			UID: "var-" + p.code, Barcode: "bc-" + p.code, IsActive: true, Code: p.code, Name: "Item " + p.code,
		}); err != nil {
			t.Fatal(err)
		}
		prices = append(prices, syncedPriceListItem{ProductVariantUID: "var-" + p.code, Price: p.price})
		if p.deal {
			discounts = append(discounts, syncedDiscountListItem{ProductVariantUID: "var-" + p.code, DealUID: "deal-b2g1"})
		}
	}
	if err := applySyncedPriceList(ctx, q, syncedPriceList{UID: "prices", Name: "Prices", IsActive: true, Items: prices}); err != nil {
		t.Fatal(err)
	}
	if err := applySyncedDiscountList(ctx, q, syncedDiscountList{
		UID: "disc", Name: "Deals", IsActive: true, Items: discounts,
		Deals: []syncedDeal{{UID: "deal-b2g1", Name: "Buy 2 get 1", Kind: dealBuyGet, BuyQty: 2, GetQty: 1, Percent: 100}},
	}); err != nil {
		t.Fatal(err)
	}
	salesperson, err := store.CreateSalesperson(ctx, "Cashier")
	if err != nil {
		t.Fatal(err)
	}
	return dealShop{app: app, store: store, cashbox: cashbox, client: client, salesperson: salesperson}
}

// sell checks out codes (one unit each) paying exactly the total.
func (s dealShop) sell(t *testing.T, codes ...string) SalesInvoiceResult {
	t.Helper()
	items := make([]salesInvoiceItem, 0, len(codes))
	for _, code := range codes {
		items = append(items, salesInvoiceItem{Barcode: "bc-" + code, Qty: 1})
	}
	itemsJSON, _ := json.Marshal(items)
	quote, err := s.app.QuoteInvoiceTotal(string(itemsJSON))
	if err != nil {
		t.Fatal(err)
	}
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: s.cashbox.ID, AmountCents: quote.GrandTotalCents}})
	result, err := s.app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(itemsJSON), PaymentsJSON: string(payments), ClientID: s.client.ID})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// The cheapest of three is free: the sale charges $180, the free unit is a
// line of its own with the deal's saving, the invoice keeps the deal's terms,
// and kashi is told.
func TestCheckoutWithDeal(t *testing.T) {
	ctx := context.Background()
	shop := setUpDealShop(t)
	sale := shop.sell(t, "A", "B", "C", "SOCKS")
	if sale.GrandTotal != 18000+2000 {
		t.Fatalf("total = %d, want 20000", sale.GrandTotal)
	}

	lines, err := shop.store.ListInvoiceProducts(ctx, sale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	byPrice := map[int64]db.InvoiceProduct{}
	for _, l := range lines {
		byPrice[l.UnitPrice] = l
	}
	free := byPrice[5000]
	if free.DealUid != "deal-b2g1" || free.DealDiscount != 5000 || free.LineTotal != 0 {
		t.Fatalf("free line = %+v", free)
	}
	if paid := byPrice[10000]; paid.DealUid != "deal-b2g1" || paid.DealDiscount != 0 || paid.LineTotal != 10000 {
		t.Fatalf("paid deal line = %+v", paid)
	}
	if socks := byPrice[2000]; socks.DealUid != "" {
		t.Fatalf("socks are on no deal: %+v", socks)
	}

	deals, err := shop.store.ListInvoiceDeals(ctx, sale.InvoiceID)
	if err != nil || len(deals) != 1 || deals[0].Name != "Buy 2 get 1" || deals[0].BuyQty != 2 {
		t.Fatalf("invoice deals = %+v, %v", deals, err)
	}

	var freeReceipt invoiceItem
	for _, item := range sale.Items {
		if item.Free {
			freeReceipt = item
		}
	}
	if freeReceipt.Name != "Item C (FREE · Buy 2 get 1)" || freeReceipt.DealDiscount != 50 {
		t.Fatalf("receipt free line = %+v", freeReceipt)
	}
	if _, _, net := itemAmounts(freeReceipt); net != 0 {
		t.Fatalf("receipt free line nets %v", net)
	}

	entries, err := shop.store.ListDueSyncOutboxEntries(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	var payload branchInvoicePayload
	if err := json.Unmarshal([]byte(entries[0].PayloadJson), &payload); err != nil {
		t.Fatal(err)
	}
	var sawFree bool
	for _, item := range payload.Items {
		if item.Deal == "Buy 2 get 1" && item.DealDiscount == 5000 && item.LineTotal == 0 {
			sawFree = true
		}
	}
	if !sawFree {
		t.Fatalf("payload items carry no free deal line: %+v", payload.Items)
	}

	// The details and the reprint mark it too.
	details, err := shop.app.GetInvoiceDetails(sale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	var detailFree int
	for _, item := range details["items"].([]map[string]any) {
		if item["free"] == true && item["deal"] == "Buy 2 get 1" {
			detailFree++
		}
	}
	if detailFree != 1 {
		t.Fatalf("details items = %+v", details["items"])
	}
}

// A line priced by hand stays out of the deal.
func TestDealSkipsHandPricedLines(t *testing.T) {
	ctx := context.Background()
	shop := setUpDealShop(t)
	pricing, err := loadActivePricing(ctx, shop.store.Queries)
	if err != nil {
		t.Fatal(err)
	}
	custom := int64(9000)
	priced, _, discounted, _, err := shop.app.validateAndPriceItems(shop.store.Queries, 0, []salesInvoiceItem{
		{Barcode: "bc-A", Qty: 1}, {Barcode: "bc-B", Qty: 1}, {Barcode: "bc-C", Qty: 1, CustomPriceCents: &custom},
	}, "half_up", true, pricing, true)
	if err != nil {
		t.Fatal(err)
	}
	if discounted != 10000+8000+9000 {
		t.Fatalf("discounted = %d: two deal units make no set", discounted)
	}
	for _, p := range priced {
		if p.product.Code == "C" && p.dealUID != "" {
			t.Fatalf("hand-priced line kept its deal: %+v", p)
		}
	}
}
