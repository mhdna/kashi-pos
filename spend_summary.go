package main

import (
	"database/sql"
	"fmt"
	"time"
)

// GetSpendSummary is what the shift screens show about money going out:
// this month's expense and loan totals (one per currency; no longer shown,
// the Expenses and Loans pages are manager only) and, for shiftID, the expenses and loans entered during
// that shift (for the income card and the close-shift summary). Amounts are
// in each row's own currency, x100, as entered. shiftID 0 skips the lists.
func (a *App) GetSpendSummary(shiftID int64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).Format("2006-01-02 15:04:05")

	monthExpenses, err := a.store.SumExpensesByCurrencySince(a.ctx, monthStart)
	if err != nil {
		return nil, err
	}
	monthLoans, err := a.store.SumLoansByCurrencySince(a.ctx, monthStart)
	if err != nil {
		return nil, err
	}

	shiftExpenses := []map[string]any{}
	shiftLoans := []map[string]any{}
	if shiftID > 0 {
		expenses, err := a.store.ListExpensesForShift(a.ctx, sql.NullInt64{Int64: shiftID, Valid: true})
		if err != nil {
			return nil, err
		}
		for _, e := range expenses {
			shiftExpenses = append(shiftExpenses, expenseMap(e))
		}
		loans, err := a.store.ListLoansForShift(a.ctx, sql.NullInt64{Int64: shiftID, Valid: true})
		if err != nil {
			return nil, err
		}
		for _, l := range loans {
			shiftLoans = append(shiftLoans, loanMap(l))
		}
	}

	return map[string]any{
		"monthExpenses": monthExpenses,
		"monthLoans":    monthLoans,
		"shiftExpenses": shiftExpenses,
		"shiftLoans":    shiftLoans,
	}, nil
}
