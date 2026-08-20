package main

import (
	"encoding/json"
	"testing"

	db "kashi-pos/db/sqlc"
)

// TestBalanceAggregationAgreesWithInvoiceTotalsAcrossMixedPayments is the
// explicit regression guard for "balance page total doesn't match invoice
// total": for a shift with several invoices paid through a mix of
// currencies (some requiring LBP cash-rounding, some not), every income
// aggregation endpoint must sum to exactly the same total as the invoices
// themselves - no rounding drift from summing already-canonical amounts.
func TestBalanceAggregationAgreesWithInvoiceTotalsAcrossMixedPayments(t *testing.T) {
	app, usdCash, client, database := setupQuoteTestApp(t, 0.11, "half_up", "LBP")
	defer database.Close()

	lbpCash, err := app.store.CreateCashboxAccount(app.ctx, db.CreateCashboxAccountParams{Name: "LBP Cash", CurrencyCode: "LBP"})
	if err != nil {
		t.Fatal(err)
	}
	visa, err := app.store.CreateCashboxAccount(app.ctx, db.CreateCashboxAccountParams{Name: "Visa", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}

	items := cartJSON(3)
	quote, err := app.QuoteInvoiceTotal(items)
	if err != nil {
		t.Fatal(err)
	}

	// Invoice 1: paid entirely in USD cash.
	payments1, _ := json.Marshal([]checkoutPayment{{AccountID: usdCash.ID, AmountCents: quote.GrandTotalCents}})
	result1, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: items, PaymentsJSON: string(payments1), ClientID: client.ID})
	if err != nil {
		t.Fatal(err)
	}

	// Invoice 2: paid entirely in LBP cash, using the exact backend-computed
	// LBP amount due (the same conversion the calculator uses).
	lbpCurrency, err := app.store.GetCurrencyByCode(app.ctx, "LBP")
	if err != nil {
		t.Fatal(err)
	}
	dueLbpUnits, err := roundCurrencyUnitsFromCents(quote.GrandTotalCents, lbpCurrency.UnitsPerUsdMicros, lbpCurrency.CashRoundingUnit, quote.RoundingMode)
	if err != nil {
		t.Fatal(err)
	}
	lbpAmountCents, err := app.ConvertCashEntryToCents("LBP", float64(dueLbpUnits))
	if err != nil {
		t.Fatal(err)
	}
	payments2, _ := json.Marshal([]checkoutPayment{{AccountID: lbpCash.ID, AmountCents: lbpAmountCents}})
	result2, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: items, PaymentsJSON: string(payments2), ClientID: client.ID})
	if err != nil {
		t.Fatal(err)
	}

	// Invoice 3: split between Visa and LBP cash.
	quote3, err := app.QuoteInvoiceTotal(items)
	if err != nil {
		t.Fatal(err)
	}
	visaShare := quote3.GrandTotalCents / 3
	remainderCents := quote3.GrandTotalCents - visaShare
	remainderLbpUnits, err := roundCurrencyUnitsFromCents(remainderCents, lbpCurrency.UnitsPerUsdMicros, lbpCurrency.CashRoundingUnit, quote3.RoundingMode)
	if err != nil {
		t.Fatal(err)
	}
	remainderLbpCents, err := app.ConvertCashEntryToCents("LBP", float64(remainderLbpUnits))
	if err != nil {
		t.Fatal(err)
	}
	payments3, _ := json.Marshal([]checkoutPayment{
		{AccountID: visa.ID, AmountCents: visaShare},
		{AccountID: lbpCash.ID, AmountCents: remainderLbpCents},
	})
	result3, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: items, PaymentsJSON: string(payments3), ClientID: client.ID})
	if err != nil {
		t.Fatalf("split payment checkout rejected: %v", err)
	}

	invoiceTotalSum := result1.GrandTotal + result2.GrandTotal + result3.GrandTotal

	shift, err := app.store.GetOpenShift(app.ctx)
	if err != nil {
		t.Fatal(err)
	}

	balances, err := app.ListShiftBalances(shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	var balanceSum int64
	for _, row := range balances {
		if row["isOnAccount"].(bool) {
			continue
		}
		balanceSum += databaseInt64(row["balance"])
	}
	if balanceSum != invoiceTotalSum {
		t.Fatalf("ListShiftBalances sum = %d, want %d (sum of invoice grand totals)", balanceSum, invoiceTotalSum)
	}

	currentShiftIncome, err := app.GetCurrentShiftIncomeByAccount()
	if err != nil {
		t.Fatal(err)
	}
	var currentShiftSum int64
	for _, row := range currentShiftIncome {
		currentShiftSum += databaseInt64(row["total"])
	}
	if currentShiftSum != invoiceTotalSum {
		t.Fatalf("GetCurrentShiftIncomeByAccount sum = %d, want %d", currentShiftSum, invoiceTotalSum)
	}

	todayIncome, err := app.GetTodayIncomeByAccount()
	if err != nil {
		t.Fatal(err)
	}
	var todaySum int64
	for _, row := range todayIncome {
		if row["isOnAccount"].(bool) {
			continue
		}
		todaySum += databaseInt64(row["total"])
	}
	if todaySum != invoiceTotalSum {
		t.Fatalf("GetTodayIncomeByAccount sum = %d, want %d", todaySum, invoiceTotalSum)
	}

	shiftTotals, err := app.GetShiftTotals()
	if err != nil {
		t.Fatal(err)
	}
	if len(shiftTotals) != 1 || databaseInt64(shiftTotals[0]["total"]) != invoiceTotalSum {
		t.Fatalf("GetShiftTotals = %#v, want a single shift totaling %d", shiftTotals, invoiceTotalSum)
	}
}
