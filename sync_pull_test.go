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
		if calls <= 17 && since != "" {
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
		case "price_lists":
			fmt.Fprint(w, `{"entity":"price_lists","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		case "discount_lists":
			fmt.Fprint(w, `{"entity":"discount_lists","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		case "clients":
			fmt.Fprint(w, `{"entity":"clients","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		case "branch_targets":
			fmt.Fprint(w, `{"entity":"branch_targets","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		case "salespersons":
			fmt.Fprint(w, `{"entity":"salespersons","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		case "branch_users":
			fmt.Fprint(w, `{"entity":"branch_users","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		case "expense_categories":
			fmt.Fprint(w, `{"entity":"expense_categories","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		case "loan_categories":
			fmt.Fprint(w, `{"entity":"loan_categories","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		case "coupon_types":
			fmt.Fprint(w, `{"entity":"coupon_types","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		case "coupons":
			fmt.Fprint(w, `{"entity":"coupons","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		case "branches":
			fmt.Fprint(w, `{"entity":"branches","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		case "transfer_requests":
			fmt.Fprint(w, `{"entity":"transfer_requests","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		case "incoming_transfer_requests":
			fmt.Fprint(w, `{"entity":"incoming_transfer_requests","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		case "branch_stock":
			fmt.Fprint(w, `{"entity":"branch_stock","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
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
	if calls != 34 {
		t.Fatalf("expected 34 total calls (17 entities x 2 ticks), got %d", calls)
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

func TestSyncPullAdoptsLocalSalespersonByNameAndAppliesColor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("entity") {
		case "salespersons":
			fmt.Fprint(w, `{"entity":"salespersons","items":[{"id":7,"name":"Sarah M","is_active":true,"color":"#1E88E5"},{"id":8,"name":"New Person","is_active":true,"color":"#43A047"}],"server_time":"2026-08-24T00:00:00Z"}`)
		default:
			fmt.Fprint(w, `{"entity":"noop","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		}
	}))
	defer server.Close()

	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	client := newCentralClient(server.URL, "TEST01", "test-key")

	// Made on the till before it was connected; kashi's name differs only
	// in case and spacing.
	local, err := store.CreateSalesperson(app.ctx, "  sarah m ")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateSalespersonAttendanceID(app.ctx, db.UpdateSalespersonAttendanceIDParams{
		AttendanceUserID: "DEVICE-42",
		ID:               local.ID,
	}); err != nil {
		t.Fatal(err)
	}

	if err := app.pullEntityOnce(app.ctx, client, "salespersons"); err != nil {
		t.Fatal(err)
	}

	adopted, err := store.GetSalespersonByRemoteID(app.ctx, sql.NullInt64{Int64: 7, Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	if adopted.ID != local.ID || adopted.FullName != "Sarah M" || adopted.Color != "#1E88E5" || adopted.AttendanceUserID != "DEVICE-42" {
		t.Fatalf("local salesperson not adopted: %+v", adopted)
	}
	created, err := store.GetSalespersonByRemoteID(app.ctx, sql.NullInt64{Int64: 8, Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	if created.Color != "#43A047" {
		t.Fatalf("new salesperson color = %q", created.Color)
	}
	all, err := store.ListSalespersons(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 salespersons (no duplicate), got %d: %+v", len(all), all)
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

// kashi hard-deletes categories (and renames / moves them to central), none
// of which a since-delta can carry. The till pulls the whole branch-scoped
// set every time and drops anything kashi no longer reports.
func TestSyncPullPrunesCategoriesKashiNoLongerReports(t *testing.T) {
	items := `[{"name":"Electricity","is_active":true,"icon":"mdi-flash","color":"red"},{"name":"test","is_active":true,"icon":"mdi-tag-outline","color":"blue-grey"}]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if since := r.URL.Query().Get("since"); since != "" {
			t.Fatalf("category pull must always ask for the whole set, got since=%q", since)
		}
		w.Header().Set("Content-Type", "application/json")
		entity := r.URL.Query().Get("entity")
		fmt.Fprintf(w, `{"entity":%q,"items":%s,"server_time":"2026-08-24T00:00:00Z"}`, entity, items)
	}))
	defer server.Close()

	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	client := newCentralClient(server.URL, "TEST01", "test-key")

	names := func(entity string) []string {
		t.Helper()
		var out []string
		switch entity {
		case "expense_categories":
			rows, err := store.ListExpenseCategories(app.ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range rows {
				out = append(out, r.Name)
			}
		case "loan_categories":
			rows, err := store.ListLoanCategories(app.ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range rows {
				out = append(out, r.Name)
			}
		}
		return out
	}

	for _, entity := range []string{"expense_categories", "loan_categories"} {
		items = `[{"name":"Electricity","is_active":true,"icon":"mdi-flash","color":"red"},{"name":"test","is_active":true,"icon":"mdi-tag-outline","color":"blue-grey"}]`
		if changed, err := app.pullEntityLocked(app.ctx, client, entity); err != nil || !changed {
			t.Fatalf("first pull of %s: changed=%v err=%v", entity, changed, err)
		}
		if got := names(entity); len(got) != 2 {
			t.Fatalf("%s after first pull: %v", entity, got)
		}
		if changed, err := app.pullEntityLocked(app.ctx, client, entity); err != nil || changed {
			t.Fatalf("identical re-pull of %s: changed=%v err=%v", entity, changed, err)
		}

		items = `[{"name":"Electricity","is_active":false,"icon":"mdi-flash","color":"red"}]`
		if changed, err := app.pullEntityLocked(app.ctx, client, entity); err != nil || !changed {
			t.Fatalf("pull of %s after delete: changed=%v err=%v", entity, changed, err)
		}
		if got := names(entity); len(got) != 1 || got[0] != "Electricity" {
			t.Fatalf("%s after kashi deleted \"test\": %v", entity, got)
		}
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

// kashi resends currencies and payment methods whole on every pull, so the
// item count is never zero. Only a real difference in the local table may
// count as a change -- otherwise sync_applied fires every 15s and the
// Balance page (which listens for both) reloads under the cashier.
func TestSyncPullWholeSetReportsChangeOnlyWhenTableDiffers(t *testing.T) {
	lbpRate := "92000000000"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("entity") {
		case "currencies":
			fmt.Fprintf(w, `{"entity":"currencies","items":[{"code":"LBP","name":"Lebanese Pound","symbol":"LL","is_default":false,"is_active":true,"units_per_usd_micros":%s,"cash_rounding_unit":10000}],"server_time":"2026-08-24T00:00:00Z"}`, lbpRate)
		case "cashbox_accounts":
			fmt.Fprint(w, `{"entity":"cashbox_accounts","items":[{"name":"Wire Transfer","currency_code":"USD","sort_order":9,"color":"#00ff00"}],"server_time":"2026-08-24T00:00:00Z"}`)
		default:
			t.Fatalf("unexpected entity %q", r.URL.Query().Get("entity"))
		}
	}))
	defer server.Close()

	app, _, _, _ := setUpTestInvoiceApp(t, server.URL)
	client := newCentralClient(server.URL, "TEST01", "test-key")

	pull := func(entity string) bool {
		t.Helper()
		changed, err := app.pullEntityLocked(app.ctx, client, entity)
		if err != nil {
			t.Fatalf("pull %s: %v", entity, err)
		}
		return changed
	}

	for _, entity := range []string{"currencies", "cashbox_accounts"} {
		if !pull(entity) {
			t.Fatalf("first pull of %s applied new data but reported no change", entity)
		}
		if pull(entity) {
			t.Fatalf("second pull of %s got the identical set but reported a change", entity)
		}
	}

	lbpRate = "89500000000"
	if !pull("currencies") {
		t.Fatal("LBP rate changed in kashi but the pull reported no change")
	}
}

// Once kashi decides a salesperson's device number (attendance_user_id_set),
// the pull replaces the one typed in here -- an empty one too, which clears
// it. Until then the till's own number is what it reports up to kashi.
func TestSyncPullTakesKashiSetAttendanceID(t *testing.T) {
	item := `{"id":7,"name":"Sarah M","is_active":true}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("entity") {
		case "salespersons":
			fmt.Fprintf(w, `{"entity":"salespersons","items":[%s],"server_time":"2026-08-24T00:00:00Z"}`, item)
		default:
			fmt.Fprint(w, `{"entity":"noop","items":[],"server_time":"2026-08-24T00:00:00Z"}`)
		}
	}))
	defer server.Close()

	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	client := newCentralClient(server.URL, "TEST01", "test-key")
	pull := func() db.Salesperson {
		t.Helper()
		if err := app.pullEntityOnce(app.ctx, client, "salespersons"); err != nil {
			t.Fatal(err)
		}
		s, err := store.GetSalespersonByRemoteID(app.ctx, sql.NullInt64{Int64: 7, Valid: true})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	local := pull()
	if err := store.UpdateSalespersonAttendanceID(app.ctx, db.UpdateSalespersonAttendanceIDParams{
		AttendanceUserID: "DEVICE-42", ID: local.ID,
	}); err != nil {
		t.Fatal(err)
	}
	if got := salespersonAttendanceIDsForReport(app.ctx, store.Queries); got["7"] != "DEVICE-42" {
		t.Fatalf("report = %v, want kashi id 7 -> DEVICE-42", got)
	}

	item = `{"id":7,"name":"Sarah M","is_active":true,"attendance_user_id":"15","attendance_user_id_set":true}`
	if got := pull().AttendanceUserID; got != "15" {
		t.Fatalf("after kashi set it: attendance id = %q, want 15", got)
	}

	item = `{"id":7,"name":"Sarah M","is_active":true,"attendance_user_id":"","attendance_user_id_set":true}`
	if got := pull().AttendanceUserID; got != "" {
		t.Fatalf("after kashi cleared it: attendance id = %q, want empty", got)
	}

	// Staff who only punch attendance: still on the roster, no longer sells.
	if got := pull().IsSalesperson; got != 1 {
		t.Fatalf("is_salesperson before kashi says otherwise = %d, want 1", got)
	}
	item = `{"id":7,"name":"Sarah M","is_active":true,"is_salesperson":false}`
	if got := pull().IsSalesperson; got != 0 {
		t.Fatalf("after kashi turned selling off: is_salesperson = %d, want 0", got)
	}
}
