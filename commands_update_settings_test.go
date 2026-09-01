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

// Exercises the widened update_settings command: a carried field is applied
// through the same local setter the Settings page uses, and an absent
// (nil pointer) field leaves the current value untouched.
func TestExecuteUpdateSettingsCommand_WidenedFields(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "update_settings.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}

	store := db.NewStore(database)
	if _, err := database.Exec(`INSERT INTO settings (
		branch_name, api_key, printer_id, printer_size, receipt_printer,
		screen_port, receipt_width, receipt_height, theme, tax_rate,
		rounding_mode, rounding_currency, receipt_enabled, sales_inventory_id,
		receipt_font, receipt_body_font, central_api_url
	) VALUES ('Test', '', 'OLD-PRINTER', '80mm', '', 'COM3', 80, 297, 'system', 0,
		'half_up', 'USD', 0, 0, 'Times', 'Courier', 'http://kashi.test')`); err != nil {
		t.Fatal(err)
	}

	role, err := store.CreateRole(ctx, db.CreateRoleParams{Name: "admin", Description: "Administrator"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateUser(ctx, db.CreateUserParams{
		Username: "admin", PasswordHash: hashPassword("admin"), RoleID: role.ID, Active: 1,
	}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store, conn: database}

	apply := func(p updateSettingsCommandPayload) {
		t.Helper()
		raw, _ := json.Marshal(p)
		if err := app.executeUpdateSettingsCommand(ctx, raw); err != nil {
			t.Fatalf("executeUpdateSettingsCommand: %v", err)
		}
	}
	boolp := func(b bool) *bool { return &b }
	strp := func(s string) *string { return &s }
	i64p := func(i int64) *int64 { return &i }

	// 1. Carry a spread of the new fields.
	apply(updateSettingsCommandPayload{
		BranchName: "Test", RoundingMode: "half_up", RoundingCurrency: "USD", ExchangeRate: 1,

		InvoiceKeyboardMode: boolp(true),
		PageUnlockInventory: boolp(true),
		ReceiptFont:         strp("Belleza"),
		ReceiptCutoff:       boolp(true),
		ScreenPort:          strp("COM7"),
		AkuvoxIP:            strp("10.0.0.9"),
		AkuvoxPassword:      strp("s3cret"),

		AttendanceDuplicateIntervalSeconds: i64p(45),
		AttendanceApproverPassword:         strp("manager-pw"),
	})

	if !app.flagEnabled(ctx, invoiceKeyboardModeFlag) {
		t.Error("invoice_keyboard_mode flag should be on")
	}
	if !app.flagEnabled(ctx, pageUnlockFlag("inventory")) {
		t.Error("page_unlock_inventory flag should be on")
	}
	got, err := store.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.ReceiptFont != "Belleza" {
		t.Errorf("receipt_font = %q, want Belleza", got.ReceiptFont)
	}
	if got.ReceiptCutoff != 1 {
		t.Errorf("receipt_cutoff = %d, want 1", got.ReceiptCutoff)
	}
	if got.ScreenPort != "COM7" {
		t.Errorf("screen_port = %q, want COM7", got.ScreenPort)
	}
	ip, _, pw, _, dup, err := app.attendanceConfig()
	if err != nil {
		t.Fatal(err)
	}
	if ip != "10.0.0.9" {
		t.Errorf("akuvox_ip = %q, want 10.0.0.9", ip)
	}
	if pw != "s3cret" {
		t.Errorf("akuvox_password = %q, want s3cret", pw)
	}
	if dup != 45 {
		t.Errorf("duplicate_interval_seconds = %d, want 45", dup)
	}
	if err := app.verifyManagerPassword("manager-pw"); err != nil {
		t.Errorf("approver password not set: %v", err)
	}

	// admin_password force-sets the admin sign-in password (no old password).
	apply(updateSettingsCommandPayload{
		BranchName: "Test", RoundingMode: "half_up", RoundingCurrency: "USD", ExchangeRate: 1,
		AdminPassword: strp("new-admin-pw"),
	})
	if app.adminPasswordHash() != hashPassword("new-admin-pw") {
		t.Error("admin password was not force-set")
	}

	// 2. A payload that omits them must not disturb the applied values.
	apply(updateSettingsCommandPayload{
		BranchName: "Test", RoundingMode: "half_up", RoundingCurrency: "USD", ExchangeRate: 1,
	})

	if !app.flagEnabled(ctx, invoiceKeyboardModeFlag) {
		t.Error("omitted flag should have been left ON")
	}
	got, err = store.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.ReceiptFont != "Belleza" {
		t.Errorf("omitted receipt_font changed to %q", got.ReceiptFont)
	}
	if got.ScreenPort != "COM7" {
		t.Errorf("omitted screen_port changed to %q", got.ScreenPort)
	}
	if got.PrinterID != "OLD-PRINTER" {
		t.Errorf("printer_id unexpectedly changed to %q", got.PrinterID)
	}
	if _, _, pw, _, dup, _ := app.attendanceConfig(); pw != "s3cret" || dup != 45 {
		t.Errorf("omitted attendance fields changed: pw=%q dup=%d", pw, dup)
	}
}
