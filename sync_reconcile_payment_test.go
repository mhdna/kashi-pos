package main

import (
	"context"
	"testing"

	db "kashi-pos/db/sqlc"
)

func activeMethodNames(t *testing.T, q *db.Queries) []string {
	t.Helper()
	accounts, err := q.ListActiveCashboxAccounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(accounts))
	for _, a := range accounts {
		names = append(names, a.Name)
	}
	return names
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// The core of the rule: a payment method kashi stops sending stops being
// offered at the till.
func TestReconcileCashboxAccountsRetiresWhatKashiNoLongerHas(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)
	q := store.Queries

	sent := []syncedCashboxAccount{
		{Name: "Cash $$", CurrencyCode: "USD"},
		{Name: "Visa", CurrencyCode: "USD"},
	}
	for _, a := range sent {
		if err := applySyncedCashboxAccount(ctx, q, a); err != nil {
			t.Fatal(err)
		}
	}
	if err := reconcileCashboxAccounts(ctx, q, sent); err != nil {
		t.Fatal(err)
	}
	if names := activeMethodNames(t, q); !contains(names, "Cash $$") || !contains(names, "Visa") {
		t.Fatalf("both sent methods should be active, got %v", names)
	}

	// kashi drops Visa.
	remaining := []syncedCashboxAccount{{Name: "Cash $$", CurrencyCode: "USD"}}
	if err := applySyncedCashboxAccount(ctx, q, remaining[0]); err != nil {
		t.Fatal(err)
	}
	if err := reconcileCashboxAccounts(ctx, q, remaining); err != nil {
		t.Fatal(err)
	}

	names := activeMethodNames(t, q)
	if contains(names, "Visa") {
		t.Fatalf("Visa should no longer be offered, got %v", names)
	}
	if !contains(names, "Cash $$") {
		t.Fatalf("Cash $$ should still be offered, got %v", names)
	}

	// Retired, not deleted -- history referencing it still has to resolve.
	if _, err := q.GetCashboxAccountByName(ctx, "Visa"); err != nil {
		t.Fatalf("Visa row must survive so past payments still resolve: %v", err)
	}
}

// "If kashi doesn't have any payment methods then kashi-pos doesn't" -- the
// empty payload is a real state, not a no-op, and the NOT IN queries it feeds
// have to handle it.
func TestReconcileCashboxAccountsEmptyPayloadRetiresEverything(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)
	q := store.Queries

	for _, a := range []syncedCashboxAccount{
		{Name: "Cash $$", CurrencyCode: "USD"},
		{Name: "Visa", CurrencyCode: "USD"},
	} {
		if err := applySyncedCashboxAccount(ctx, q, a); err != nil {
			t.Fatal(err)
		}
	}
	if err := reconcileCashboxAccounts(ctx, q, nil); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"Cash $$", "Visa"} {
		if contains(activeMethodNames(t, q), name) {
			t.Fatalf("%q should be retired when kashi sends nothing", name)
		}
	}
}

// The two internal placeholders are not kashi's to retire -- the coupon and
// client-account settlement paths look them up by exact name.
func TestReconcileCashboxAccountsKeepsInternalPlaceholders(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)
	q := store.Queries

	for _, name := range []string{"On Account", "On Coupon"} {
		if _, err := q.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{
			Name: name, CurrencyCode: "USD",
		}); err != nil {
			t.Fatal(err)
		}
	}

	// kashi sends nothing at all -- the strongest case.
	if err := reconcileCashboxAccounts(ctx, q, nil); err != nil {
		t.Fatal(err)
	}

	names := activeMethodNames(t, q)
	for _, name := range []string{"On Account", "On Coupon"} {
		if !contains(names, name) {
			t.Fatalf("%q is an internal placeholder and must survive, got %v", name, names)
		}
	}
}

// A method kashi re-adds comes back rather than staying retired.
func TestReconcileCashboxAccountsReactivatesOnReturn(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)
	q := store.Queries

	visa := []syncedCashboxAccount{{Name: "Visa", CurrencyCode: "USD"}}
	if err := applySyncedCashboxAccount(ctx, q, visa[0]); err != nil {
		t.Fatal(err)
	}
	if err := reconcileCashboxAccounts(ctx, q, nil); err != nil {
		t.Fatal(err)
	}
	if contains(activeMethodNames(t, q), "Visa") {
		t.Fatal("precondition: Visa should be retired")
	}

	if err := applySyncedCashboxAccount(ctx, q, visa[0]); err != nil {
		t.Fatal(err)
	}
	if err := reconcileCashboxAccounts(ctx, q, visa); err != nil {
		t.Fatal(err)
	}
	if !contains(activeMethodNames(t, q), "Visa") {
		t.Fatalf("Visa should be offered again once kashi re-adds it, got %v", activeMethodNames(t, q))
	}
}

// Same rule for currencies, except the default is the accounting anchor every
// amount is stored in and is not kashi's to retire.
func TestReconcileCurrenciesRetiresWhatKashiLacksButKeepsTheAnchor(t *testing.T) {
	ctx := context.Background()
	store := setUpTestClientDB(t)
	q := store.Queries

	eur := syncedCurrency{
		Code: "EUR", Name: "Euro", Symbol: "€", IsActive: true,
		UnitsPerUsdMicros: 920000, CashRoundingUnit: 5,
	}
	if err := applySyncedCurrency(ctx, q, eur); err != nil {
		t.Fatal(err)
	}

	before, err := q.GetDefaultCurrency(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// kashi sends nothing.
	if err := reconcileCurrencies(ctx, q, nil); err != nil {
		t.Fatal(err)
	}

	retired, err := q.GetCurrencyByCode(ctx, "EUR")
	if err != nil {
		t.Fatal(err)
	}
	if retired.IsActive {
		t.Fatal("EUR should be retired once kashi no longer has it")
	}

	// The anchor (USC, the accounting cent) is untouched -- whatever state it
	// was in, the reconcile does not get a vote. Asserting "unchanged" rather
	// than "active" on purpose: USC ships inactive because it is an internal
	// accounting unit, not a tender anyone pays with.
	after, err := q.GetCurrencyByCode(ctx, before.Code)
	if err != nil {
		t.Fatal(err)
	}
	if after.IsActive != before.IsActive || !after.IsDefault {
		t.Fatalf("the accounting anchor %q was changed by the reconcile: before=%v after=%v",
			before.Code, before.IsActive, after.IsActive)
	}
}

// A retired payment method must not be offered at checkout.
func TestListPaymentMethodsHidesRetiredAccounts(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	addPaymentMethod(t, app, "Cash $$", "USD")
	addPaymentMethod(t, app, "Visa", "USD")

	if err := reconcileCashboxAccounts(app.ctx, app.store.Queries,
		[]syncedCashboxAccount{{Name: "Cash $$", CurrencyCode: "USD"}}); err != nil {
		t.Fatal(err)
	}

	methods, err := app.ListPaymentMethods()
	if err != nil {
		t.Fatal(err)
	}
	names := paymentMethodNames(t, methods)
	if contains(names, "Visa") {
		t.Fatalf("a retired method must not be offered at checkout, got %v", names)
	}
	if !contains(names, "Cash $$") {
		t.Fatalf("the surviving method should still be offered, got %v", names)
	}
}
