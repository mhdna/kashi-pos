package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	db "kashi-pos/db/sqlc"
)

func TestSyncPullAppliesCurrenciesAndCashboxAccountsAndAdvancesCursor(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		entity := r.URL.Query().Get("entity")
		since := r.URL.Query().Get("since")
		if calls <= 8 && since != "" {
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
		case "branch_targets":
			fmt.Fprint(w, `{"entity":"branch_targets","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		case "salespersons":
			fmt.Fprint(w, `{"entity":"salespersons","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		case "expense_categories":
			fmt.Fprint(w, `{"entity":"expense_categories","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		case "loan_categories":
			fmt.Fprint(w, `{"entity":"loan_categories","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
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
	if calls != 16 {
		t.Fatalf("expected 16 total calls (8 entities x 2 ticks), got %d", calls)
	}
}

func TestSyncPullAppliesSalespersonWithoutTouchingLocalAttendanceID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("entity") {
		case "salespersons":
			fmt.Fprint(w, `{"entity":"salespersons","items":[{"id":7,"name":"Sarah M","is_active":true}],"server_time":"2026-08-24T00:00:00Z"}`)
		default:
			fmt.Fprint(w, `{"entity":"noop","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		}
	}))
	defer server.Close()

	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	client := newCentralClient(server.URL, "TEST01", "test-key")

	if err := app.pullEntityOnce(app.ctx, client, "salespersons"); err != nil {
		t.Fatal(err)
	}

	created, err := store.GetSalespersonByRemoteID(app.ctx, sql.NullInt64{Int64: 7, Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	if created.FullName != "Sarah M" || created.IsActive != 1 {
		t.Fatalf("salesperson not synced correctly: %+v", created)
	}

	if err := store.UpdateSalespersonAttendanceID(app.ctx, db.UpdateSalespersonAttendanceIDParams{
		AttendanceUserID: "DEVICE-42",
		ID:               created.ID,
	}); err != nil {
		t.Fatal(err)
	}

	// A second pull with the same name/active state must not clobber the
	// attendance id set locally in between.
	if err := app.pullEntityOnce(app.ctx, client, "salespersons"); err != nil {
		t.Fatal(err)
	}
	after, err := store.GetSalespersonByRemoteID(app.ctx, sql.NullInt64{Int64: 7, Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	if after.AttendanceUserID != "DEVICE-42" {
		t.Fatalf("attendance_user_id was overwritten by sync: got %q", after.AttendanceUserID)
	}
}

func TestSyncPullRemovesBranchTargetWhenKashiMarksItInactive(t *testing.T) {
	active := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("entity") {
		case "branch_targets":
			fmt.Fprintf(w, `{"entity":"branch_targets","items":[{"id":9,"date_from":"2026-08-01T00:00:00Z","date_to":"2026-08-31T00:00:00Z","target_amount":500000,"color":"#4C6EF5","is_active":%v}],"server_time":"2026-08-24T00:00:00Z"}`, active)
		default:
			fmt.Fprint(w, `{"entity":"noop","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		}
	}))
	defer server.Close()

	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	client := newCentralClient(server.URL, "TEST01", "test-key")

	if err := app.pullEntityOnce(app.ctx, client, "branch_targets"); err != nil {
		t.Fatal(err)
	}
	targets, err := store.ListBranchTargets(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target after initial sync, got %d", len(targets))
	}

	// Deleting a target in kashi is a soft delete (is_active flips to
	// false) precisely so this branch can see the change and remove its
	// local copy -- a hard delete on kashi's side would just vanish from
	// the "changed since" query with no way for this branch to notice.
	active = false
	if err := app.pullEntityOnce(app.ctx, client, "branch_targets"); err != nil {
		t.Fatal(err)
	}
	targets, err = store.ListBranchTargets(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 0 {
		t.Fatalf("expected target removed after kashi marked it inactive, got %d", len(targets))
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
