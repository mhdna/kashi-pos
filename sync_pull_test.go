package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSyncPullAppliesCurrenciesAndCashboxAccountsAndAdvancesCursor(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		entity := r.URL.Query().Get("entity")
		since := r.URL.Query().Get("since")
		if calls <= 4 && since != "" {
			t.Fatalf("expected first pull for %s to have no since, got %q", entity, since)
		}
		w.Header().Set("Content-Type", "application/json")
		switch entity {
		case "currencies":
			fmt.Fprint(w, `{"entity":"currencies","items":[{"code":"LBP","name":"Lebanese Pound","symbol":"LL","is_default":false,"is_active":true,"units_per_usd_micros":92000000000,"cash_rounding_unit":10000}],"server_time":"2026-08-24T00:00:00Z"}`)
		case "cashbox_accounts":
			fmt.Fprint(w, `{"entity":"cashbox_accounts","items":[{"name":"Wire Transfer","currency_code":"USD","sort_order":9,"color":"#00ff00"}],"server_time":"2026-08-24T00:00:00Z"}`)
		case "products":
			fmt.Fprint(w, `{"entity":"products","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		case "clients":
			fmt.Fprint(w, `{"entity":"clients","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		default:
			t.Fatalf("unexpected entity %q", entity)
		}
	}))
	defer server.Close()

	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	client := newCentralClient(server.URL, "TEST01", "test-key")

	app.runSyncPullOnce(app.ctx, client)

	lbp, err := store.GetCurrencyByCode(app.ctx, "LBP")
	if err != nil {
		t.Fatal(err)
	}
	if lbp.UnitsPerUsdMicros != 92000000000 || lbp.CashRoundingUnit != 10000 {
		t.Fatalf("LBP not updated from pull: %+v", lbp)
	}

	wire, err := store.GetCashboxAccountByName(app.ctx, "Wire Transfer")
	if err != nil {
		t.Fatal(err)
	}
	if wire.CurrencyCode != "USD" || wire.Color != "#00ff00" {
		t.Fatalf("Wire Transfer cashbox account not created correctly: %+v", wire)
	}

	cursor, err := store.GetSyncCursor(app.ctx, "currencies")
	if err != nil {
		t.Fatal(err)
	}
	if cursor != "2026-08-24T00:00:00Z" {
		t.Fatalf("currencies cursor = %q, want 2026-08-24T00:00:00Z", cursor)
	}

	// Second pull should send the saved cursor as `since`, not fetch
	// everything all over again.
	app.runSyncPullOnce(app.ctx, client)
	if calls != 8 {
		t.Fatalf("expected 8 total calls (4 entities x 2 ticks), got %d", calls)
	}
}

func TestSyncPullProtectsLocalDefaultCurrency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("entity") {
		case "currencies":
			fmt.Fprint(w, `{"entity":"currencies","items":[{"code":"USC","name":"Renamed By Kashi","symbol":"X","is_default":true,"is_active":true,"units_per_usd_micros":1,"cash_rounding_unit":1}],"server_time":"2026-08-24T00:00:00Z"}`)
		default:
			fmt.Fprint(w, `{"entity":"cashbox_accounts","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		}
	}))
	defer server.Close()

	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	client := newCentralClient(server.URL, "TEST01", "test-key")

	app.runSyncPullOnce(app.ctx, client)

	usc, err := store.GetCurrencyByCode(app.ctx, "USC")
	if err != nil {
		t.Fatal(err)
	}
	if usc.Name == "Renamed By Kashi" {
		t.Fatal("local default currency (USC) must not be overwritten by a pulled change")
	}
}
