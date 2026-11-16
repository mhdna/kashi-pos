package main

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"

	db "kashi-pos/db/sqlc"
)

type negativeCountFixture struct {
	db                    *sql.DB
	app                   *App
	store                 *db.Store
	shiftID, personID     int64
	usdID, lbpID, visaID  int64
	invoiceN              int64
	inventoryID, clientID int64
}

func newNegativeCountFixture(t *testing.T) *negativeCountFixture {
	t.Helper()
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "negative.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	params := defaultSettingsParams()
	params.ReceiptEnabled = 0
	if err := store.UpsertSettings(ctx, params); err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, store: store}
	shift, err := store.CreateShift(ctx, db.CreateShiftParams{})
	if err != nil {
		t.Fatal(err)
	}
	person, err := store.CreateSalesperson(ctx, "Rami")
	if err != nil {
		t.Fatal(err)
	}
	account := func(currency, name string, cash bool) int64 {
		t.Helper()
		accounts, err := store.ListCashboxAccounts(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range accounts {
			if cash && isCashAccount(a.Name, a.CurrencyCode) && strings.EqualFold(a.CurrencyCode, currency) {
				return a.ID
			}
			if !cash && strings.EqualFold(a.Name, name) {
				return a.ID
			}
		}
		a, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: name, CurrencyCode: currency})
		if err != nil {
			t.Fatal(err)
		}
		return a.ID
	}
	inventory, err := store.CreateInventory(ctx, "Sales")
	if err != nil {
		t.Fatal(err)
	}
	client, err := store.CreateClient(ctx, db.CreateClientParams{Name: "Walk-in", Phone: "00000000"})
	if err != nil {
		t.Fatal(err)
	}
	return &negativeCountFixture{
		db: database, app: app, store: store, shiftID: shift.ID, personID: person.ID,
		usdID: account("USD", "Cash $$", true), lbpID: account("LBP", "Cash LL", true),
		visaID:      account("USD", "Visa", false),
		inventoryID: inventory.ID, clientID: client.ID,
	}
}

// sale records an invoice of grandTotal cents paid by the given account
// amounts (a negative amount is change handed back).
func (f *negativeCountFixture) sale(t *testing.T, grandTotal int64, payments map[int64]int64) {
	t.Helper()
	ctx := context.Background()
	f.invoiceN++
	invoice, err := f.store.CreateInvoice(ctx, db.CreateInvoiceParams{
		CashboxID: f.usdID, ShiftID: f.shiftID, InvoiceCode: fmt.Sprintf("SA/CN/2026/%05d", f.invoiceN),
		InvoiceIndex: f.invoiceN, Year: 2026, ClientID: f.clientID, InventoryID: f.inventoryID,
		GrandTotal: grandTotal, DiscountedTotal: grandTotal, Subtotal: grandTotal,
	})
	if err != nil {
		t.Fatal(err)
	}
	for accountID, amount := range payments {
		if amount == 0 {
			continue
		}
		if err := f.store.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{InvoiceID: invoice.ID, CashboxID: accountID, Amount: amount}); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *negativeCountFixture) expected(t *testing.T) (usd, lbp, visa, whish int64, bills string) {
	t.Helper()
	totals, err := f.app.GetShiftExpectedTotals(f.shiftID)
	if err != nil {
		t.Fatal(err)
	}
	counts := totals["usdBillCounts"].(map[string]int64)
	parts := make([]string, 0, len(counts))
	for _, d := range usdBillDenominations {
		key := formatInt(d)
		parts = append(parts, `"`+key+`":`+formatInt(counts[key]))
	}
	return totals["expectedUsd"].(int64), totals["expectedLbp"].(int64),
		totals["expectedVisa"].(int64), totals["expectedWhish"].(int64), "{" + strings.Join(parts, ",") + "}"
}

func (f *negativeCountFixture) swapRows(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM shift_balance_replacements`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// The user's case: $100 paid in USD, $7.66 handed back in LBP, so the LBP
// drawer is short of what came in. Autofill suggests the negative LBP, the
// cashier closes with it, and the shift balances to the cent: nothing is
// swapped into USD and the difference is zero.
func TestCloseShiftCountsNegativeLBPAsIs(t *testing.T) {
	f := newNegativeCountFixture(t)
	f.sale(t, 9234, map[int64]int64{f.usdID: 10000, f.lbpID: -766})

	usd, lbp, visa, whish, bills := f.expected(t)
	if usd != 10000 {
		t.Fatalf("expected USD = %d, want 10000 (no swap into USD)", usd)
	}
	if lbp >= 0 {
		t.Fatalf("expected LBP = %d, want negative", lbp)
	}

	result, err := f.app.CloseShiftByID(f.shiftID, f.personID, lbp, visa, whish, bills)
	if err != nil {
		t.Fatalf("close with negative LBP: %v", err)
	}
	if f.swapRows(t) != 0 {
		t.Fatal("closing moved a balance; negative LBP must stay where it is")
	}
	closed, err := f.store.GetShiftByID(context.Background(), f.shiftID)
	if err != nil {
		t.Fatal(err)
	}
	if closed.CountedLbp != lbp || closed.ExpectedLbp != lbp {
		t.Fatalf("stored LBP counted %d expected %d, want both %d", closed.CountedLbp, closed.ExpectedLbp, lbp)
	}
	if closed.CountedUsd != 10000 || closed.ExpectedUsd != 10000 {
		t.Fatalf("stored USD counted %d expected %d, want 10000", closed.CountedUsd, closed.ExpectedUsd)
	}
	if printed, _ := result["receiptPrinted"].(bool); printed {
		t.Fatal("receipt printed while printing is off")
	}
}

// Negative Visa and Whish (a card refund bigger than the shift's card
// sales) are accepted and stored as counted.
func TestCloseShiftAcceptsNegativeVisaAndWhish(t *testing.T) {
	f := newNegativeCountFixture(t)
	if _, err := f.app.CloseShiftByID(f.shiftID, f.personID, -150000, -1250, -300, `{}`); err != nil {
		t.Fatalf("close with negative counts: %v", err)
	}
	closed, err := f.store.GetShiftByID(context.Background(), f.shiftID)
	if err != nil {
		t.Fatal(err)
	}
	if closed.CountedLbp != -150000 || closed.CountedVisa != -1250 || closed.CountedWhish != -300 {
		t.Fatalf("stored counts = LBP %d Visa %d Whish %d", closed.CountedLbp, closed.CountedVisa, closed.CountedWhish)
	}
}

// Hundreds of random shifts: whatever mix of LBP change, Visa refunds and
// USD comes in, counting exactly what autofill suggests (negatives
// included) closes with a zero difference and a counted total equal to
// what was paid; counting one LBP unit or one cent off anywhere shows up
// in the difference.
func TestNegativeCountsReconcileRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(20261116))
	f := newNegativeCountFixture(t)
	for run := 0; run < 300; run++ {
		if run > 0 {
			shift, err := f.store.CreateShift(context.Background(), db.CreateShiftParams{})
			if err != nil {
				t.Fatal(err)
			}
			f.shiftID = shift.ID
		}
		sales := 1 + rng.Intn(4)
		for i := 0; i < sales; i++ {
			usdPaid := int64(rng.Intn(20)) * 500
			lbp := int64(rng.Intn(4000)) - 3000 // mostly change going out
			visa := int64(rng.Intn(3000)) - 1500
			f.sale(t, usdPaid+lbp+visa, map[int64]int64{f.usdID: usdPaid, f.lbpID: lbp, f.visaID: visa})
		}
		usd, lbp, visa, whish, bills := f.expected(t)
		usdCounts, countedUSD, _, err := parseUSDBillCounts(bills)
		if err != nil {
			t.Fatal(err)
		}
		if usd >= 0 && countedUSD != usd {
			t.Fatalf("run %d: bill suggestion %d != expected USD %d", run, countedUSD, usd)
		}
		data, _, err := f.app.buildShiftClosePrintData(f.shiftID, f.personID, countedUSD, lbp, visa, whish, usdCounts, "2026-11-16 12:00:00", true)
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		r := data.ShiftReport
		if r.CountedLBP-r.ExpectedLBP != 0 || r.CountedVisa-r.ExpectedVisa != 0 {
			t.Fatalf("run %d: differences LBP %d Visa %d, want 0", run, r.CountedLBP-r.ExpectedLBP, r.CountedVisa-r.ExpectedVisa)
		}
		if usd >= 0 && r.CountedTotal != r.TotalPayments {
			t.Fatalf("run %d: counted total %d != paid %d (LBP %d, Visa %d)", run, r.CountedTotal, r.TotalPayments, lbp, visa)
		}

		// One cent off on Visa is one cent of difference.
		off, _, err := f.app.buildShiftClosePrintData(f.shiftID, f.personID, countedUSD, lbp, visa-1, whish, usdCounts, "2026-11-16 12:00:00", true)
		if err != nil {
			t.Fatal(err)
		}
		if off.ShiftReport.CountedTotal != r.CountedTotal-1 {
			t.Fatalf("run %d: one cent off Visa moved the total by %d", run, off.ShiftReport.CountedTotal-r.CountedTotal)
		}
		// Short on LBP (more negative) is a shortfall, never an overage.
		short, _, err := f.app.buildShiftClosePrintData(f.shiftID, f.personID, countedUSD, lbp-100000, visa, whish, usdCounts, "2026-11-16 12:00:00", true)
		if err != nil {
			t.Fatal(err)
		}
		if short.ShiftReport.CountedTotal >= r.CountedTotal {
			t.Fatalf("run %d: 100,000 LBP short did not lower the counted total", run)
		}

		if _, err := f.app.CloseShiftByID(f.shiftID, f.personID, lbp, visa, whish, bills); err != nil {
			t.Fatalf("run %d: close: %v", run, err)
		}
	}
	if f.swapRows(t) != 0 {
		t.Fatal("closing swapped a balance")
	}
}
