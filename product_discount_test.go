package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func TestUpdateProductDiscountRequiresFeatureAndPersists(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "discount.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}

	store := db.NewStore(database)
	product, err := store.CreateProduct(ctx, db.CreateProductParams{
		Barcode: "123", Code: "SHIRT-1", Name: "Shirt", IsActive: 1,
		Price: 2500, Discount: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, store: store}

	if err := app.UpdateProductDiscount(product.Code, 25); err == nil {
		t.Fatal("expected disabled custom discounts to reject updates")
	}
	if err := app.SetCustomItemDiscountsEnabled(true); err != nil {
		t.Fatal(err)
	}
	if err := app.UpdateProductDiscount(product.Code, 25); err != nil {
		t.Fatal(err)
	}
	updated, err := store.GetProductByCode(ctx, product.Code)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Discount != 25 {
		t.Fatalf("discount = %d, want 25", updated.Discount)
	}
	if err := app.UpdateProductDiscount(product.Code, 101); err == nil {
		t.Fatal("expected discount above 100 to fail")
	}
}

func TestItemPricingSettingsAreIndependent(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "pricing-settings.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(
		"INSERT INTO settings (branch_name, api_key, printer_id, printer_size) VALUES ('Store', '', '', '80mm')",
	); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: db.NewStore(database)}
	if err := app.SetCustomItemPricesEnabled(true); err != nil {
		t.Fatal(err)
	}
	if err := app.SetPerUnitItemPricesEnabled(true); err != nil {
		t.Fatal(err)
	}
	settings, err := app.GetAppSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings["customItemPricesEnabled"] != true {
		t.Fatalf("custom item prices = %#v, want true", settings["customItemPricesEnabled"])
	}
	if settings["perUnitItemPricesEnabled"] != true {
		t.Fatalf("per-unit item prices = %#v, want true", settings["perUnitItemPricesEnabled"])
	}
	if settings["customItemDiscountsEnabled"] != false {
		t.Fatalf("custom item discounts = %#v, want false", settings["customItemDiscountsEnabled"])
	}
}
