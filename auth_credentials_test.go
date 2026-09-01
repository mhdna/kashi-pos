package main

import (
	"context"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func TestAuthCredentialsChangeAndVerify(t *testing.T) {
	ctx := context.Background()
	database := openTestDatabase(t)
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, store: db.NewStore(database), conn: database}

	// Seeded defaults from migration 000083.
	if _, err := app.VerifyAdminPassword("1963"); err != nil {
		t.Fatalf("default admin password should work: %v", err)
	}
	if role, err := app.VerifyPIN("1478"); err != nil || role["role"] != "cashier" {
		t.Fatalf("default cashier PIN: role %v err %v", role["role"], err)
	}

	// Change the admin password - wrong current is rejected, right one sticks.
	if err := app.ChangeAdminPassword("nope", "s3cret!!"); err == nil {
		t.Fatal("ChangeAdminPassword accepted a wrong current password")
	}
	if err := app.ChangeAdminPassword("1963", "s3cret!!"); err != nil {
		t.Fatalf("ChangeAdminPassword: %v", err)
	}
	if _, err := app.VerifyAdminPassword("1963"); err == nil {
		t.Fatal("old admin password still works after change")
	}
	if _, err := app.VerifyAdminPassword("s3cret!!"); err != nil {
		t.Fatalf("new admin password should work: %v", err)
	}

	// Cashier PIN must be exactly 4 digits.
	for _, bad := range []string{"123", "12345", "12ab", ""} {
		if err := app.SetCashierPIN(bad); err == nil {
			t.Fatalf("SetCashierPIN accepted invalid PIN %q", bad)
		}
	}
	if err := app.SetCashierPIN("2468"); err != nil {
		t.Fatalf("SetCashierPIN: %v", err)
	}
	if _, err := app.VerifyPIN("1478"); err == nil {
		t.Fatal("old cashier PIN still works after change")
	}
	if role, err := app.VerifyPIN("2468"); err != nil || role["role"] != "cashier" {
		t.Fatalf("new cashier PIN: role %v err %v", role["role"], err)
	}
	if got, _ := app.GetCashierPIN(); got != "2468" {
		t.Fatalf("GetCashierPIN = %q, want 2468", got)
	}

	// ForceSetAdminPassword bypasses the current-password check (kashi command path).
	if err := app.ForceSetAdminPassword("from-kashi"); err != nil {
		t.Fatalf("ForceSetAdminPassword: %v", err)
	}
	if _, err := app.VerifyAdminPassword("from-kashi"); err != nil {
		t.Fatalf("password set by ForceSetAdminPassword should work: %v", err)
	}
}
