package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

type balanceSwapFixture struct {
	app           *App
	database      *sql.DB
	shiftID       int64
	sourceID      int64
	destinationID int64
	onAccountID   int64
	onCouponID    int64
}

func newBalanceSwapFixture(t *testing.T, total int64) balanceSwapFixture {
	t.Helper()
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "swap.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	if err := store.UpsertSettings(ctx, defaultSettingsParams()); err != nil {
		t.Fatal(err)
	}
	shift, err := store.CreateShift(ctx, db.CreateShiftParams{})
	if err != nil {
		t.Fatal(err)
	}
	account := func(name string) db.CashboxAccount {
		t.Helper()
		value, createErr := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: name, CurrencyCode: "USD"})
		if createErr != nil {
			t.Fatal(createErr)
		}
		return value
	}
	source := account("Cash")
	destination := account("Card")
	onAccount := account("On Account")
	onCoupon := account("On Coupon")
	inventory, err := store.CreateInventory(ctx, "Sales")
	if err != nil {
		t.Fatal(err)
	}
	client, err := store.CreateClient(ctx, db.CreateClientParams{Name: "Walk-in", Phone: "00000000"})
	if err != nil {
		t.Fatal(err)
	}
	invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: source.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/00001",
		InvoiceIndex: 1, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		GrandTotal: total, DiscountedTotal: total, Subtotal: total,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{InvoiceID: invoice.ID, CashboxID: source.ID, Amount: total}); err != nil {
		t.Fatal(err)
	}
	return balanceSwapFixture{
		app: &App{ctx: ctx, store: store}, database: database, shiftID: shift.ID,
		sourceID: source.ID, destinationID: destination.ID, onAccountID: onAccount.ID, onCouponID: onCoupon.ID,
	}
}

func fixtureBalances(t *testing.T, fixture balanceSwapFixture) map[int64]int64 {
	t.Helper()
	rows, err := fixture.app.ListShiftBalances(fixture.shiftID)
	if err != nil {
		t.Fatal(err)
	}
	result := map[int64]int64{}
	for _, row := range rows {
		result[row["accountId"].(int64)] = databaseInt64(row["balance"])
	}
	return result
}

func replacementCount(t *testing.T, database *sql.DB) int64 {
	t.Helper()
	var count int64
	if err := database.QueryRow(`SELECT COUNT(*) FROM shift_balance_replacements`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestReplaceShiftBalancePreservesTotalAcrossPartialAndReverseSwaps(t *testing.T) {
	fixture := newBalanceSwapFixture(t, 1000)
	if err := fixture.app.ReplaceShiftBalance(fixture.shiftID, fixture.sourceID, fixture.destinationID, 400); err != nil {
		t.Fatal(err)
	}
	balances := fixtureBalances(t, fixture)
	if balances[fixture.sourceID] != 600 || balances[fixture.destinationID] != 400 {
		t.Fatalf("partial swap balances = source %d, destination %d", balances[fixture.sourceID], balances[fixture.destinationID])
	}
	if err := fixture.app.ReplaceShiftBalance(fixture.shiftID, fixture.destinationID, fixture.sourceID, 150); err != nil {
		t.Fatal(err)
	}
	balances = fixtureBalances(t, fixture)
	if balances[fixture.sourceID] != 750 || balances[fixture.destinationID] != 250 {
		t.Fatalf("reverse swap balances = source %d, destination %d", balances[fixture.sourceID], balances[fixture.destinationID])
	}
	if balances[fixture.sourceID]+balances[fixture.destinationID] != 1000 {
		t.Fatalf("swap changed total balance: %#v", balances)
	}
	var originalPayment int64
	if err := fixture.database.QueryRow(`SELECT amount FROM invoice_payments LIMIT 1`).Scan(&originalPayment); err != nil {
		t.Fatal(err)
	}
	if originalPayment != 1000 {
		t.Fatalf("swap changed original payment to %d", originalPayment)
	}
}

func TestReplaceShiftBalancePreservesNegativeTotalAndRejectsOverdraw(t *testing.T) {
	fixture := newBalanceSwapFixture(t, -1000)
	if err := fixture.app.ReplaceShiftBalance(fixture.shiftID, fixture.sourceID, fixture.destinationID, 400); err != nil {
		t.Fatal(err)
	}
	balances := fixtureBalances(t, fixture)
	if balances[fixture.sourceID] != -600 || balances[fixture.destinationID] != -400 {
		t.Fatalf("negative partial swap balances = source %d, destination %d", balances[fixture.sourceID], balances[fixture.destinationID])
	}
	if err := fixture.app.ReplaceShiftBalance(fixture.shiftID, fixture.sourceID, fixture.destinationID, 601); err == nil {
		t.Fatal("negative source overdraw was accepted")
	}
	if replacementCount(t, fixture.database) != 1 {
		t.Fatal("rejected overdraw wrote a replacement record")
	}
}

func TestReplaceShiftBalanceRejectsInvalidAndNonCashAccountsWithoutWriting(t *testing.T) {
	fixture := newBalanceSwapFixture(t, 1000)
	cases := []struct {
		name     string
		from, to int64
		amount   int64
	}{
		{name: "zero amount", from: fixture.sourceID, to: fixture.destinationID, amount: 0},
		{name: "negative amount", from: fixture.sourceID, to: fixture.destinationID, amount: -1},
		{name: "same account", from: fixture.sourceID, to: fixture.sourceID, amount: 1},
		{name: "above balance", from: fixture.sourceID, to: fixture.destinationID, amount: 1001},
		{name: "on account destination", from: fixture.sourceID, to: fixture.onAccountID, amount: 1},
		{name: "coupon destination", from: fixture.sourceID, to: fixture.onCouponID, amount: 1},
		{name: "missing destination", from: fixture.sourceID, to: 999999, amount: 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := fixture.app.ReplaceShiftBalance(fixture.shiftID, test.from, test.to, test.amount); err == nil {
				t.Fatal("invalid replacement was accepted")
			}
		})
	}
	if replacementCount(t, fixture.database) != 0 {
		t.Fatal("invalid replacements wrote records")
	}
	balances := fixtureBalances(t, fixture)
	if balances[fixture.sourceID] != 1000 || balances[fixture.destinationID] != 0 {
		t.Fatalf("invalid replacements changed balances: %#v", balances)
	}
}

func TestReplaceShiftBalanceMovesNegativeBalanceWithItsSign(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "balance.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}

	store := db.NewStore(database)
	if err := store.UpsertSettings(ctx, defaultSettingsParams()); err != nil {
		t.Fatal(err)
	}
	shift, err := store.CreateShift(ctx, db.CreateShiftParams{})
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	destination, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Card", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := store.CreateInventory(ctx, "Sales")
	if err != nil {
		t.Fatal(err)
	}
	client, err := store.CreateClient(ctx, db.CreateClientParams{Name: "Walk-in", Phone: "00000000"})
	if err != nil {
		t.Fatal(err)
	}
	invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: source.ID, ShiftID: shift.ID, InvoiceCode: "RS/CN/2026/00001",
		InvoiceIndex: 1, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		GrandTotal: -1000, DiscountedTotal: -1000, Subtotal: -1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{
		InvoiceID: invoice.ID, CashboxID: source.ID, Amount: -1000,
	}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store}
	if err := app.ReplaceShiftBalance(shift.ID, source.ID, destination.ID, 1000); err != nil {
		t.Fatal(err)
	}
	balances, err := app.ListShiftBalances(shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]int64{}
	for _, balance := range balances {
		got[balance["accountId"].(int64)] = databaseInt64(balance["balance"])
	}
	if got[source.ID] != 0 || got[destination.ID] != -1000 {
		t.Fatalf("balances after negative replacement = source %d, destination %d", got[source.ID], got[destination.ID])
	}
	currentShift, err := app.GetCurrentShiftIncomeByAccount()
	if err != nil {
		t.Fatal(err)
	}
	todayBalances := map[int64]int64{}
	for _, balance := range currentShift {
		todayBalances[balance["accountId"].(int64)] = int64(balance["total"].(float64))
	}
	if todayBalances[source.ID] != 0 || todayBalances[destination.ID] != -1000 {
		t.Fatalf("current shift balances after negative replacement = source %d, destination %d", todayBalances[source.ID], todayBalances[destination.ID])
	}
}

// TestReplaceShiftBalanceEmptiesRoundedCashSourceDespiteConversionOvershoot
// covers the swap dialog's autofill path: it rounds a physically-rounded cash
// balance (LBP) up to the nearest cash unit and converts that back to cents,
// which can exceed the exact source magnitude by up to one cash unit. Moving
// the whole balance of such an account - here a return, so a negative balance
// swapped into USD - must be accepted and clamped, not rejected.
func TestReplaceShiftBalanceEmptiesRoundedCashSourceDespiteConversionOvershoot(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "lbp-swap.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	if err := store.UpsertSettings(ctx, defaultSettingsParams()); err != nil {
		t.Fatal(err)
	}
	shift, err := store.CreateShift(ctx, db.CreateShiftParams{})
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash LBP", CurrencyCode: "LBP"})
	if err != nil {
		t.Fatal(err)
	}
	destination, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash $$", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := store.CreateInventory(ctx, "Sales")
	if err != nil {
		t.Fatal(err)
	}
	client, err := store.CreateClient(ctx, db.CreateClientParams{Name: "Walk-in", Phone: "00000000"})
	if err != nil {
		t.Fatal(err)
	}
	// A return booked to the LBP cash account: -2220 accounting cents, whose
	// magnitude (2220) sits just below the 2,000,000 LBP the dialog autofills.
	const sourceCents = -2220
	invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: source.ID, ShiftID: shift.ID, InvoiceCode: "RS/CN/2026/00002",
		InvoiceIndex: 1, Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
		GrandTotal: sourceCents, DiscountedTotal: sourceCents, Subtotal: sourceCents,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{
		InvoiceID: invoice.ID, CashboxID: source.ID, Amount: sourceCents,
	}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store}
	// What the dialog sends: 2,000,000 LBP (autofilled from the -2220c
	// balance) converted back to cents = 2222, two cents over the magnitude.
	entryCents, err := app.ConvertCashEntryToCents("LBP", 2_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if entryCents <= 2220 {
		t.Fatalf("expected conversion to overshoot the source magnitude, got %d", entryCents)
	}

	preview, err := app.PreviewShiftBalanceReplacement(shift.ID, source.ID, destination.ID, entryCents)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Valid {
		t.Fatalf("preview rejected emptying a rounded cash source: %q", preview.Reason)
	}
	if preview.SourceRemainingCents != 0 || preview.DestinationReceivesCents != sourceCents {
		t.Fatalf("preview = %#v, want remaining 0 and destination %d", preview, sourceCents)
	}
	if err := app.ReplaceShiftBalance(shift.ID, source.ID, destination.ID, entryCents); err != nil {
		t.Fatalf("submit rejected a swap the preview accepted: %v", err)
	}
	balances, err := app.ListShiftBalances(shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]int64{}
	for _, balance := range balances {
		got[balance["accountId"].(int64)] = databaseInt64(balance["balance"])
	}
	if got[source.ID] != 0 || got[destination.ID] != sourceCents {
		t.Fatalf("balances after swap = source %d, destination %d; want 0 and %d", got[source.ID], got[destination.ID], sourceCents)
	}
}

// TestPreviewShiftBalanceReplacementNeverDivergesFromActualSubmit is the
// regression guard for the swap-balance dialog: whatever
// PreviewShiftBalanceReplacement predicts must be exactly what
// ReplaceShiftBalance actually does, for a valid swap, an over-the-limit
// swap, and a swap through a blocked (on-account) balance.
func TestPreviewShiftBalanceReplacementNeverDivergesFromActualSubmit(t *testing.T) {
	fixture := newBalanceSwapFixture(t, 5000)
	defer fixture.database.Close()

	// A valid partial swap: preview must match what actually happens.
	preview, err := fixture.app.PreviewShiftBalanceReplacement(fixture.shiftID, fixture.sourceID, fixture.destinationID, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Valid || preview.SourceRemainingCents != 3000 || preview.DestinationReceivesCents != 2000 {
		t.Fatalf("preview = %#v, want valid with remaining 3000 and destination 2000", preview)
	}
	if err := fixture.app.ReplaceShiftBalance(fixture.shiftID, fixture.sourceID, fixture.destinationID, 2000); err != nil {
		t.Fatalf("submit rejected a swap the preview said was valid: %v", err)
	}
	balances := fixtureBalances(t, fixture)
	if balances[fixture.sourceID] != preview.SourceRemainingCents {
		t.Fatalf("actual source remaining = %d, preview said %d", balances[fixture.sourceID], preview.SourceRemainingCents)
	}
	if balances[fixture.destinationID] != preview.DestinationReceivesCents {
		t.Fatalf("actual destination balance = %d, preview said %d", balances[fixture.destinationID], preview.DestinationReceivesCents)
	}

	// Over the remaining source balance (3000 left): preview must say
	// invalid, and an actual submit must also be rejected.
	overPreview, err := fixture.app.PreviewShiftBalanceReplacement(fixture.shiftID, fixture.sourceID, fixture.destinationID, 4000)
	if err != nil {
		t.Fatal(err)
	}
	if overPreview.Valid || overPreview.Reason == "" {
		t.Fatalf("preview accepted a swap larger than the remaining source balance: %#v", overPreview)
	}
	if err := fixture.app.ReplaceShiftBalance(fixture.shiftID, fixture.sourceID, fixture.destinationID, 4000); err == nil {
		t.Fatal("submit accepted a swap the preview said was invalid")
	}

	// A blocked account (On Account): preview must say invalid, matching
	// the actual submit's rejection.
	blockedPreview, err := fixture.app.PreviewShiftBalanceReplacement(fixture.shiftID, fixture.onAccountID, fixture.destinationID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if blockedPreview.Valid {
		t.Fatal("preview accepted a swap through a blocked On Account balance")
	}
	if err := fixture.app.ReplaceShiftBalance(fixture.shiftID, fixture.onAccountID, fixture.destinationID, 100); err == nil {
		t.Fatal("submit accepted a swap through a blocked On Account balance")
	}
}
