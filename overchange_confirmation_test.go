package main

import (
	"context"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func TestOverchangeConfirmationFlag(t *testing.T) {
	ctx := context.Background()
	database := openTestDatabase(t)
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	if err := db.NewStore(database).UpsertSettings(ctx, defaultSettingsParams()); err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, store: db.NewStore(database), conn: database}

	// Default (flag never set) is on.
	if !app.overchangeConfirmationEnabled() {
		t.Fatal("overchangeConfirmationEnabled() should default to true")
	}
	if settings, err := app.GetAppSettings(); err != nil {
		t.Fatal(err)
	} else if settings["overchangeConfirmationEnabled"] != true {
		t.Fatalf("GetAppSettings overchangeConfirmationEnabled = %v, want true", settings["overchangeConfirmationEnabled"])
	}

	// Turn it off, then back on.
	if err := app.SetOverchangeConfirmationEnabled(false); err != nil {
		t.Fatal(err)
	}
	if app.overchangeConfirmationEnabled() {
		t.Fatal("flag should be off after SetOverchangeConfirmationEnabled(false)")
	}
	if settings, err := app.GetAppSettings(); err != nil {
		t.Fatal(err)
	} else if settings["overchangeConfirmationEnabled"] != false {
		t.Fatalf("GetAppSettings overchangeConfirmationEnabled = %v, want false", settings["overchangeConfirmationEnabled"])
	}

	if err := app.SetOverchangeConfirmationEnabled(true); err != nil {
		t.Fatal(err)
	}
	if !app.overchangeConfirmationEnabled() {
		t.Fatal("flag should be on again")
	}
}
