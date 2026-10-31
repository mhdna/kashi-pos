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

func setUpDealShop(t *testing.T) dealShop { return setUpDealShopWithFallback(t, 0) }

// setUpDealShopWithFallback is the same shop with "Buy 2 get 1" giving
// fallback percent to the units outside a set.
func setUpDealShopWithFallback(t *testing.T, fallback int64) dealShop {
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
		Deals: []syncedDeal{{UID: "deal-b2g1", Name: "Buy 2 get 1", Kind: dealBuyGet, BuyQty: 2, GetQty: 1, Percent: 100, FallbackPercent: fallback}},
	}); err != nil {
		t.Fatal(err)
	}
	salesperson, err := store.CreateSalesperson(ctx, "Cashier")
	if err != nil {
		t.Fatal(err)
	}
	return dealShop{app: app, store: store, cashbox: cashbox, client: client, salesperson: salesperson}
}

// sell checks out codes paying exactly the total; a code given twice is one
// line of two units, as the cart sends it.
func (s dealShop) sell(t *testing.T, codes ...string) SalesInvoiceResult {
	t.Helper()
	var items []salesInvoiceItem
	at := map[string]int{}
	for _, code := range codes {
		if i, ok := at[code]; ok {
			items[i].Qty++
			continue
		}
		at[code] = len(items)
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
	if freeReceipt.Name != "Item C (FREE)" || freeReceipt.DealDiscount != 50 {
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

// "Buy 2 get 1, otherwise 30%": a lone shirt, or two, are 30% off; three are
// the deal, and a fourth unit beside the set is 30% off on its own. A jacket
// on no deal is no part of it.
func TestCheckoutWithDealFallback(t *testing.T) {
	ctx := context.Background()
	shop := setUpDealShopWithFallback(t, 30)

	for _, tc := range []struct {
		name  string
		codes []string
		want  int64
	}{
		{"one shirt", []string{"A"}, 7000},
		{"two shirts", []string{"A", "B"}, 7000 + 5600},
		{"a shirt beside a jacket off the deal", []string{"A", "JACKET"}, 7000 + 10000},
		{"three shirts are the deal", []string{"A", "B", "C"}, 18000},
		{"four: the set, and the dearest left over at 30%", []string{"A", "B", "C", "D"}, 32000 - 5000 - 3000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := shop.sell(t, tc.codes...).GrandTotal; got != tc.want {
				t.Fatalf("total = %d, want %d", got, tc.want)
			}
		})
	}

	sale := shop.sell(t, "A")
	deals, err := shop.store.ListInvoiceDeals(ctx, sale.InvoiceID)
	if err != nil || len(deals) != 1 || deals[0].FallbackPercent != 30 {
		t.Fatalf("invoice deals = %+v, %v", deals, err)
	}
}

// One product sold as Summer and as Winter is two cart lines on the same
// deal; each quote line names its option, so the cart gives each line its
// own share of the deal rather than the whole of it twice. Four $15 jeans on
// "Buy 2 get 1, otherwise 30%": the Summer one is free, and of the three
// Winter ones one gets the 30%.
func TestDealQuoteLinesNameTheirSaleOption(t *testing.T) {
	ctx := context.Background()
	app, store, _, _ := setUpTestInvoiceApp(t, "http://localhost:9999")
	q := store.Queries
	if err := applySyncedProductVariant(ctx, q, syncedProductVariant{
		UID: "var-jeans", Barcode: "bc-jeans", IsActive: true, Code: "CA23KGJE15", Name: "Kids Girl Jeans",
		SaleOptions: []string{"Summer", "Winter"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := applySyncedPriceList(ctx, q, syncedPriceList{
		UID: "prices", Name: "Prices", IsActive: true,
		Items: []syncedPriceListItem{{ProductVariantUID: "var-jeans", Price: 1500}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := applySyncedDiscountList(ctx, q, syncedDiscountList{
		UID: "disc", Name: "Deals", IsActive: true,
		Items: []syncedDiscountListItem{{ProductVariantUID: "var-jeans", DealUID: "deal-b2g1"}},
		Deals: []syncedDeal{{UID: "deal-b2g1", Name: "Buy 2 get 1", Kind: dealBuyGet, BuyQty: 2, GetQty: 1, Percent: 100, FallbackPercent: 30}},
	}); err != nil {
		t.Fatal(err)
	}

	itemsJSON, _ := json.Marshal([]salesInvoiceItem{
		{Barcode: "bc-jeans", Qty: 1, SaleOption: "Summer"},
		{Barcode: "bc-jeans", Qty: 3, SaleOption: "Winter"},
	})
	quote, err := app.QuoteInvoiceTotal(string(itemsJSON))
	if err != nil {
		t.Fatal(err)
	}
	dealOff := map[string]int64{}
	for _, line := range quote.Lines {
		if line.Barcode == "bc-jeans" {
			dealOff[line.SaleOption] += line.DealDiscountCents
		}
	}
	if dealOff["Summer"] != 1500 || dealOff["Winter"] != 450 || len(dealOff) != 2 {
		t.Fatalf("deal off by option = %v, want Summer 1500 (free), Winter 450 (30%% of one)", dealOff)
	}
	if quote.SubtotalCents-quote.DiscountedSubtotalCents != 1950 {
		t.Fatalf("quote = %+v, want 1950 off in all", quote)
	}
}

// One "Buy 2 get 1" for the whole shop, CA codes' leftover units at 30%
// (their rows' own fallback) and the rest at the deal's 20%: a mixed cart
// makes one set across both, and a unit left outside it gets its own
// fallback.
func TestDealPoolsUnitsWithTheirOwnFallback(t *testing.T) {
	ctx := context.Background()
	app, store, _, _ := setUpTestInvoiceApp(t, "http://localhost:9999")
	q := store.Queries
	thirty := int64(30)
	products := []struct {
		code     string
		price    int64
		fallback *int64
	}{{"CA23KGJE15", 1500, &thirty}, {"CA23KGJE25", 2500, &thirty}, {"BOC5KBJE15", 1500, nil}, {"BOC5KBJE20", 2000, nil}}
	var prices []syncedPriceListItem
	var rows []syncedDiscountListItem
	for _, p := range products {
		if err := applySyncedProductVariant(ctx, q, syncedProductVariant{
			UID: "var-" + p.code, Barcode: "bc-" + p.code, IsActive: true, Code: p.code, Name: p.code,
		}); err != nil {
			t.Fatal(err)
		}
		prices = append(prices, syncedPriceListItem{ProductVariantUID: "var-" + p.code, Price: p.price})
		rows = append(rows, syncedDiscountListItem{ProductVariantUID: "var-" + p.code, DealUID: "deal-b2g1", DealFallbackPercent: p.fallback})
	}
	if err := applySyncedPriceList(ctx, q, syncedPriceList{UID: "prices", Name: "Prices", IsActive: true, Items: prices}); err != nil {
		t.Fatal(err)
	}
	if err := applySyncedDiscountList(ctx, q, syncedDiscountList{
		UID: "disc", Name: "Deals", IsActive: true, Items: rows,
		Deals: []syncedDeal{{UID: "deal-b2g1", Name: "Buy 2 get 1", Kind: dealBuyGet, BuyQty: 2, GetQty: 1, Percent: 100, FallbackPercent: 20}},
	}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		cart  []salesInvoiceItem
		off   int64             // the deal's whole saving
		label map[string]string // the chip of each code's lines
	}{
		{
			// Your till's cart: 1 boy's + 2 girl's jeans at $15, one free.
			name: "mixed set", off: 1500,
			cart:  []salesInvoiceItem{{Barcode: "bc-BOC5KBJE15", Qty: 1}, {Barcode: "bc-CA23KGJE15", Qty: 2}},
			label: map[string]string{"BOC5KBJE15": "B2G1 else 20%", "CA23KGJE15": "B2G1 else 30%"},
		},
		{
			// $15 x3 boy's + $25 CA: a $15 free, the $25 CA left over at 30%.
			name: "CA unit left over", off: 1500 + 750,
			cart: []salesInvoiceItem{{Barcode: "bc-BOC5KBJE15", Qty: 3}, {Barcode: "bc-CA23KGJE25", Qty: 1}},
		},
		{
			// $15 x2 CA + $20 x2 boy's: a $15 free, a $20 boy's left over at 20%.
			name: "other unit left over", off: 1500 + 400,
			cart: []salesInvoiceItem{{Barcode: "bc-CA23KGJE15", Qty: 2}, {Barcode: "bc-BOC5KBJE20", Qty: 2}},
		},
		{
			// No set: each unit its own fallback.
			name: "no set", off: 450 + 300,
			cart: []salesInvoiceItem{{Barcode: "bc-CA23KGJE15", Qty: 1}, {Barcode: "bc-BOC5KBJE15", Qty: 1}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			itemsJSON, _ := json.Marshal(tc.cart)
			quote, err := app.QuoteInvoiceTotal(string(itemsJSON))
			if err != nil {
				t.Fatal(err)
			}
			var off int64
			for _, line := range quote.Lines {
				off += line.DealDiscountCents
				if want, ok := tc.label[line.Code]; ok && line.DealLabel != want {
					t.Errorf("%s chip = %q, want %q", line.Code, line.DealLabel, want)
				}
			}
			if off != tc.off {
				t.Fatalf("deal took %d off, want %d: %+v", off, tc.off, quote.Lines)
			}
		})
	}
}
