package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	db "kashi-pos/db/sqlc"
)

// CreateExpense records a branch expense and, if this install is configured
// to sync (central_api_url set), queues it for push to kashi in the same
// transaction as the write itself — see queueBranchInvoiceSync's doc comment
// in sync_outbox.go for why that matters (a crash between the two can never
// happen). An open shift is looked up on a best-effort basis: unlike
// invoices, an expense (e.g. paying a utility bill before the day's first
// shift opens) is still valid with no shift open, so GetOpenShift returning
// sql.ErrNoRows is not an error here.
func (a *App) CreateExpense(description, category string, amount int64, currencyCode string, cashboxAccountID int64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var shiftID sql.NullInt64
	if shift, err := a.store.GetOpenShift(a.ctx); err == nil {
		shiftID = sql.NullInt64{Int64: shift.ID, Valid: true}
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("check open shift: %w", err)
	}

	var expense db.Expense
	var syncConfigured bool
	err := a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		settings, err := q.GetSettings(a.ctx)
		if err != nil {
			return fmt.Errorf("load settings: %w", err)
		}
		syncConfigured = settings.CentralApiUrl != ""

		clientRef := uuid.NewString()
		expense, err = q.CreateExpense(a.ctx, db.CreateExpenseParams{
			ClientRef:        clientRef,
			Description:      description,
			Category:         category,
			Amount:           amount,
			CurrencyCode:     currencyCode,
			CashboxAccountID: cashboxAccountID,
			ShiftID:          shiftID,
		})
		if err != nil {
			return fmt.Errorf("create expense: %w", err)
		}

		if settings.CentralApiUrl == "" {
			return nil
		}

		var branchShiftID *int64
		if shiftID.Valid {
			branchShiftID = &shiftID.Int64
		}
		return queueBranchExpenseSync(a.ctx, q, settings.CentralApiUrl, branchExpensePayload{
			ClientRef:              clientRef,
			Description:            description,
			Category:               category,
			Amount:                 amount,
			CurrencyCode:           currencyCode,
			BranchCashboxAccountID: cashboxAccountID,
			BranchShiftID:          branchShiftID,
			OccurredAt:             time.Now().UTC(),
		})
	})
	if err != nil {
		return nil, err
	}
	if syncConfigured {
		a.kickSyncWorker()
	}

	return expenseMap(expense), nil
}

func (a *App) ListExpenses(pageSize, pageID int32) ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	expenses, err := a.store.ListExpenses(a.ctx, db.ListExpensesParams{
		Limit:  int64(pageSize),
		Offset: int64(pageID) * int64(pageSize),
	})
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, len(expenses))
	for i, expense := range expenses {
		result[i] = expenseMap(expense)
	}
	return result, nil
}

func expenseMap(expense db.Expense) map[string]any {
	m := map[string]any{
		"id":               expense.ID,
		"clientRef":        expense.ClientRef,
		"description":      expense.Description,
		"category":         expense.Category,
		"amount":           expense.Amount,
		"currencyCode":     expense.CurrencyCode,
		"cashboxAccountId": expense.CashboxAccountID,
		"occurredAt":       expense.OccurredAt,
		"createdAt":        expense.CreatedAt,
	}
	if expense.ShiftID.Valid {
		m["shiftId"] = expense.ShiftID.Int64
	}
	return m
}
