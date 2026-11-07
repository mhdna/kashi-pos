package main

import (
	"encoding/json"
	"testing"
)

// A held exchange comes back from the hold list exactly as it was held:
// the sale it returns items from, and its returned line still negative,
// locked and at its sold price (frontend/src/lib/holdSnapshot.ts restores
// it from this).
func TestHoldInvoiceKeepsAnExchangeWhole(t *testing.T) {
	shop := setUpDealShop(t)
	snapshot := `{"version":1,
		"items":[
			{"code":"JACKET","barcode":"bc-JACKET","price":100,"qty":1},
			{"code":"A","barcode":"bc-A","price":100,"qty":-1,"locked":true,"soldUnitPrice":100,"soldUnitDiscount":0}
		],
		"client":{"id":1},
		"exchange":{"originalInvoiceId":412,"originalInvoiceCode":"SA/CT/2026/02900"}}`
	id, err := shop.app.HoldInvoice(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	sale, err := shop.app.HoldInvoice(`{"version":1,"items":[{"code":"SOCKS","barcode":"bc-SOCKS","qty":2}]}`)
	if err != nil {
		t.Fatal(err)
	}

	holds, err := shop.app.ListHoldInvoices()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[int64]map[string]any{}
	for _, h := range holds {
		byID[h["id"].(int64)] = h
	}
	held := byID[id]
	if held == nil {
		t.Fatalf("hold %d not listed: %+v", id, holds)
	}
	raw, _ := json.Marshal(held)
	var got struct {
		ItemCount int `json:"itemCount"`
		Items     []struct {
			Code          string  `json:"code"`
			Qty           float64 `json:"qty"`
			Locked        bool    `json:"locked"`
			SoldUnitPrice float64 `json:"soldUnitPrice"`
		} `json:"items"`
		Exchange struct {
			OriginalInvoiceID   int64  `json:"originalInvoiceId"`
			OriginalInvoiceCode string `json:"originalInvoiceCode"`
		} `json:"exchange"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Exchange.OriginalInvoiceID != 412 || got.Exchange.OriginalInvoiceCode != "SA/CT/2026/02900" {
		t.Fatalf("exchange = %+v", got.Exchange)
	}
	if got.ItemCount != 2 || len(got.Items) != 2 {
		t.Fatalf("items = %+v", got.Items)
	}
	if r := got.Items[1]; r.Code != "A" || r.Qty != -1 || !r.Locked || r.SoldUnitPrice != 100 {
		t.Fatalf("returned line = %+v", r)
	}
	if _, isExchange := byID[sale]["exchange"]; isExchange {
		t.Fatalf("a held sale has no exchange: %+v", byID[sale])
	}

	// Restoring deletes the hold, as for a sale.
	if err := shop.app.DeleteHoldInvoice(id); err != nil {
		t.Fatal(err)
	}
	holds, _ = shop.app.ListHoldInvoices()
	for _, h := range holds {
		if h["id"].(int64) == id {
			t.Fatal("the restored exchange is still on hold")
		}
	}
}
