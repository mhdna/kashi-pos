package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func newRemoteControlTestApp(t *testing.T) *App {
	t.Helper()
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "remote_control.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO settings (
		branch_name, api_key, printer_id, printer_size, receipt_printer,
		screen_port, receipt_width, receipt_height, theme, tax_rate,
		rounding_mode, rounding_currency, receipt_enabled, sales_inventory_id,
		receipt_font, receipt_body_font, central_api_url
	) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'light', 0,
		'half_up', 'USD', 0, 0, 'Times', 'Courier', '')`); err != nil {
		t.Fatal(err)
	}
	return &App{ctx: ctx, store: db.NewStore(database), conn: database}
}

// The settings kashi couldn't reach before: four feature flags, cash
// handling values and the default opening float, all through update_settings.
func TestExecuteUpdateSettingsCommand_RemoteControlFields(t *testing.T) {
	ctx := context.Background()
	app := newRemoteControlTestApp(t)

	payload := `{
		"hide_payment_method_currency": true,
		"auto_append_lbp_zeros": true,
		"auto_close_stale_shifts": true,
		"visitor_counter_enabled": true,
		"branch_phone": "01 234 567",
		"max_overpayment_cents": 5000,
		"default_opening_float_usd": 2500
	}`
	if err := app.executeUpdateSettingsCommand(ctx, json.RawMessage(payload)); err != nil {
		t.Fatalf("executeUpdateSettingsCommand: %v", err)
	}

	for _, flag := range []string{hidePaymentMethodCurrencyFlag, autoAppendLbpZerosFlag, autoCloseStaleShiftsFlag, visitorCounterFlag} {
		if !app.flagEnabled(ctx, flag) {
			t.Errorf("flag %s not enabled", flag)
		}
	}
	s, err := app.store.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.BranchPhone != "01 234 567" || s.MaxOverpaymentCents != 5000 {
		t.Errorf("branch_phone/max_overpayment = %q/%d", s.BranchPhone, s.MaxOverpaymentCents)
	}
	// LBP wasn't named, so it keeps its current value.
	if s.DefaultOpeningFloatUsd != 2500 || s.DefaultOpeningFloatLbp != 0 {
		t.Errorf("opening float = %d/%d, want 2500/0", s.DefaultOpeningFloatUsd, s.DefaultOpeningFloatLbp)
	}
}

// A push merges over what the screen reported and bumps the rev; the
// screen's own reports never move the rev.
func TestDisplayPrefs_PushMergesAndBumpsRev(t *testing.T) {
	ctx := context.Background()
	app := newRemoteControlTestApp(t)

	if err := app.ReportDisplayPrefs(`{"quick_access_rows":3,"animations_enabled":true}`); err != nil {
		t.Fatal(err)
	}
	if got, _ := app.GetDisplayPrefs(); got.Rev != 0 {
		t.Fatalf("rev after a local report = %d, want 0", got.Rev)
	}

	if err := app.executeUpdateSettingsCommand(ctx, json.RawMessage(`{"display_prefs":{"quick_access_rows":5,"theme":"dark"}}`)); err != nil {
		t.Fatalf("push: %v", err)
	}
	got, err := app.GetDisplayPrefs()
	if err != nil {
		t.Fatal(err)
	}
	if got.Rev != 1 {
		t.Errorf("rev = %d, want 1", got.Rev)
	}
	if got.Prefs["quick_access_rows"] != float64(5) || got.Prefs["animations_enabled"] != true {
		t.Errorf("prefs = %v, want rows 5 and animations kept", got.Prefs)
	}
	if s, _ := app.store.GetSettings(ctx); s.Theme != "dark" {
		t.Errorf("theme = %q, want dark", s.Theme)
	}
}

func TestDisplayPrefs_RejectsUnknownOrOutOfRange(t *testing.T) {
	ctx := context.Background()
	app := newRemoteControlTestApp(t)

	for _, payload := range []string{
		`{"display_prefs":{"quick_access_rows":99}}`,
		`{"display_prefs":{"wallpaper":"cats"}}`,
		`{"display_prefs":{"theme":"purple"}}`,
	} {
		if err := app.executeUpdateSettingsCommand(ctx, json.RawMessage(payload)); err == nil {
			t.Errorf("%s: expected an error", payload)
		}
	}
	if got, _ := app.GetDisplayPrefs(); got.Rev != 0 {
		t.Errorf("rev = %d after rejected pushes, want 0", got.Rev)
	}
}

func TestExecuteCommand_RejectsNegativeOpeningFloat(t *testing.T) {
	app := newRemoteControlTestApp(t)
	err := app.executeUpdateSettingsCommand(context.Background(), json.RawMessage(`{"default_opening_float_lbp":-1}`))
	if err == nil {
		t.Fatal("expected an error for a negative opening float")
	}
}

// kashi's approve/cancel reaches the till as a command and lands on the
// request it proposed; a ref the till doesn't have fails visibly.
func TestExecuteCommand_TransferDecision(t *testing.T) {
	ctx := context.Background()
	app := newRemoteControlTestApp(t)

	if _, err := app.store.CreateTransferRequest(ctx, db.CreateTransferRequestParams{
		ClientRef: "tr-1", ToBranchCode: "B2",
	}); err != nil {
		t.Fatal(err)
	}

	err := app.executeCommand(ctx, branchCommand{Type: "transfer_decision", Payload: json.RawMessage(`{"client_ref":"tr-1","status":"approved"}`)})
	if err != nil {
		t.Fatalf("transfer_decision: %v", err)
	}
	got, err := app.store.GetTransferRequestByClientRef(ctx, "tr-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "approved" {
		t.Errorf("status = %q, want approved", got.Status)
	}

	err = app.executeCommand(ctx, branchCommand{Type: "transfer_decision", Payload: json.RawMessage(`{"client_ref":"nope","status":"cancelled"}`)})
	if err == nil {
		t.Error("expected an error for a transfer this till doesn't have")
	}
}

// The end-of-shift receipt's sign-off lines print until a till or kashi
// turns them off.
func TestShiftReportSignatures_DefaultOnAndPushable(t *testing.T) {
	ctx := context.Background()
	app := newRemoteControlTestApp(t)

	if !app.shiftReportSignatures() {
		t.Fatal("signature lines off on a till that never set them")
	}
	if err := app.executeUpdateSettingsCommand(ctx, json.RawMessage(`{"shift_report_signatures":false}`)); err != nil {
		t.Fatalf("push: %v", err)
	}
	if app.shiftReportSignatures() {
		t.Error("signature lines still on after kashi turned them off")
	}
}

// The end-of-shift receipt's header stays as typed until a till or kashi
// asks for capitals.
func TestShiftReportUppercaseHeader_DefaultOffAndPushable(t *testing.T) {
	ctx := context.Background()
	app := newRemoteControlTestApp(t)

	if app.shiftReportUppercaseHeader() {
		t.Fatal("capitalised header on a till that never set it")
	}
	if err := app.executeUpdateSettingsCommand(ctx, json.RawMessage(`{"shift_report_uppercase_header":true}`)); err != nil {
		t.Fatalf("push: %v", err)
	}
	if !app.shiftReportUppercaseHeader() {
		t.Error("header still as typed after kashi turned capitals on")
	}
}
