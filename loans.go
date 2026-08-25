package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	db "kashi-pos/db/sqlc"
)

// CreateLoan records a branch loan and, if this install is configured to
// sync (central_api_url set), queues it for push to kashi in the same
// transaction as the write itself -- see queueBranchLoanSync's doc comment
// in sync_outbox.go for why that matters. Mirrors CreateExpense in
// expenses.go: an open shift is looked up on a best-effort basis, since a
// loan (e.g. an owner injecting cash before the day's first shift opens)
// is still valid with no shift open.
func (a *App) CreateLoan(description, category string, amount int64, currencyCode string, cashboxAccountID int64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var shiftID sql.NullInt64
	if shift, err := a.store.GetOpenShift(a.ctx); err == nil {
		shiftID = sql.NullInt64{Int64: shift.ID, Valid: true}
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("check open shift: %w", err)
	}

	var loan db.Loan
	var syncConfigured bool
	err := a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		settings, err := q.GetSettings(a.ctx)
		if err != nil {
			return fmt.Errorf("load settings: %w", err)
		}
		syncConfigured = settings.CentralApiUrl != ""

		clientRef := uuid.NewString()
		loan, err = q.CreateLoan(a.ctx, db.CreateLoanParams{
			ClientRef:        clientRef,
			Description:      description,
			Category:         category,
			Amount:           amount,
			CurrencyCode:     currencyCode,
			CashboxAccountID: cashboxAccountID,
			ShiftID:          shiftID,
		})
		if err != nil {
			return fmt.Errorf("create loan: %w", err)
		}

		if settings.CentralApiUrl == "" {
			return nil
		}

		var branchShiftID *int64
		if shiftID.Valid {
			branchShiftID = &shiftID.Int64
		}
		return queueBranchLoanSync(a.ctx, q, settings.CentralApiUrl, branchLoanPayload{
			ClientRef:              clientRef,
			Description:            description,
			CategoryName:           category,
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

	return loanMap(loan), nil
}

func (a *App) ListLoans(pageSize, pageID int32) ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	loans, err := a.store.ListLoans(a.ctx, db.ListLoansParams{
		Limit:  int64(pageSize),
		Offset: int64(pageID) * int64(pageSize),
	})
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, len(loans))
	for i, loan := range loans {
		result[i] = loanMap(loan)
	}
	return result, nil
}

func loanMap(loan db.Loan) map[string]any {
	m := map[string]any{
		"id":               loan.ID,
		"clientRef":        loan.ClientRef,
		"description":      loan.Description,
		"category":         loan.Category,
		"amount":           loan.Amount,
		"currencyCode":     loan.CurrencyCode,
		"cashboxAccountId": loan.CashboxAccountID,
		"occurredAt":       loan.OccurredAt,
		"createdAt":        loan.CreatedAt,
	}
	if loan.ShiftID.Valid {
		m["shiftId"] = loan.ShiftID.Int64
	}
	return m
}
