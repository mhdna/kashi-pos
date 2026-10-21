package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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
//
// fromCentral marks an expense central paid for (see SetExpenseFromCentral):
// it is still recorded against the account it would have come from, for
// its currency, but the shift doesn't take it off the drawer.
func (a *App) CreateExpense(description, category string, amount int64, currencyCode string, cashboxAccountID int64, fromCentral bool) (map[string]any, error) {
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
			FromCentral:      boolToInt64(fromCentral),
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
		accountName := ""
		if account, err := q.GetCashboxAccountByID(a.ctx, cashboxAccountID); err == nil {
			accountName = account.Name
		}
		return queueBranchExpenseSync(a.ctx, q, settings.CentralApiUrl, branchExpensePayload{
			ClientRef:              clientRef,
			Description:            description,
			Category:               category,
			Amount:                 amount,
			CurrencyCode:           currencyCode,
			BranchCashboxAccountID: cashboxAccountID,
			AccountName:            accountName,
			BranchShiftID:          branchShiftID,
			OccurredAt:             time.Now().UTC(),
			FromCentral:            fromCentral,
		})
	})
	if err != nil {
		return nil, err
	}
	if syncConfigured {
		a.kickSyncWorker()
	}

	result := expenseMap(expense)
	// syncConfigured tells the frontend whether it's even worth waiting
	// for an "expense_upload_ready:" event -- an unconfigured install
	// never queues the expense for sync, so that event would never come.
	result["syncConfigured"] = syncConfigured
	return result, nil
}

// CheckExpenseUploadStatus is the poll-based fallback for kashi-pos's QR
// upload dialog, used alongside (not instead of) the "expense_images_uploaded"
// WS push -- see emitExpenseImagesUploaded's doc comment in sync_pull.go for
// why a push alone isn't enough. Frontend calls this every few seconds while
// the dialog is open in its "ready" (QR shown) state.
func (a *App) CheckExpenseUploadStatus(clientRef string) (bool, error) {
	if a.store == nil {
		return false, fmt.Errorf("database not initialized")
	}

	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return false, fmt.Errorf("load settings: %w", err)
	}
	if settings.CentralApiUrl == "" {
		return false, errCentralSyncDisabled
	}

	client := newCentralClient(settings.CentralApiUrl, settings.BranchCode, settings.ApiKey)
	return client.ExpenseUploadStatus(a.ctx, clientRef)
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
		"fromCentral":      expense.FromCentral != 0,
		"occurredAt":       expense.OccurredAt,
		"createdAt":        expense.CreatedAt,
	}
	if expense.CancelledAt.Valid {
		m["cancelledAt"] = expense.CancelledAt.String
	}
	if expense.ShiftID.Valid {
		m["shiftId"] = expense.ShiftID.Int64
	}
	return m
}

// branchExpenseFromCentralPayload is the body POSTed to kashi's
// /branch/expenses/from_central (see branchExpenseFromCentralRequest in
// api/branch_expense_from_central.go in the kashi repo).
type branchExpenseFromCentralPayload struct {
	ClientRef   string `json:"client_ref"`
	FromCentral bool   `json:"from_central"`
}

// SetExpenseFromCentral marks an expense as paid by central (من المركزي), or
// takes the mark off, after it was made. Admin only, like CancelExpense.
// The expense's shift stops (or starts again) taking it off the drawer's
// expected cash; a shift already closed keeps the figures it closed with.
func (a *App) SetExpenseFromCentral(id int64, fromCentral bool) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if !a.currentUserIsAdmin() {
		return nil, fmt.Errorf("only an admin can change where an expense was paid from")
	}
	expense, err := a.store.GetExpense(a.ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find expense: %w", err)
	}
	expense, err = a.setExpenseFromCentral(a.ctx, expense.ClientRef, fromCentral, true)
	if err != nil {
		return nil, err
	}
	return expenseMap(expense), nil
}

// setExpenseFromCentral saves the mark and, when pushToKashi is set, gets it
// to kashi the way updateRecordCategory does a category: rewritten into the
// expense's own pending outbox row if kashi doesn't have the expense yet,
// otherwise an "expense_from_central" row keyed per expense, so a later
// flip replaces an unsent earlier one.
func (a *App) setExpenseFromCentral(ctx context.Context, clientRef string, fromCentral, pushToKashi bool) (db.Expense, error) {
	var expense db.Expense
	var queued bool
	err := a.store.ExecTx(ctx, func(q *db.Queries) error {
		var err error
		expense, err = q.SetExpenseFromCentral(ctx, db.SetExpenseFromCentralParams{
			FromCentral: boolToInt64(fromCentral), ClientRef: clientRef,
		})
		if err != nil {
			return fmt.Errorf("update expense: %w", err)
		}
		if !pushToKashi {
			return nil
		}
		settings, err := q.GetSettings(ctx)
		if err != nil {
			return fmt.Errorf("load settings: %w", err)
		}
		if settings.CentralApiUrl == "" {
			return nil
		}
		rewritten, err := q.SetUnsyncedSyncOutboxJSONField(ctx, db.SetUnsyncedSyncOutboxJSONFieldParams{
			FieldPath: "$.from_central", JsonValue: strconv.FormatBool(fromCentral), ClientRef: clientRef,
		})
		if err != nil {
			return fmt.Errorf("update pending expense: %w", err)
		}
		if rewritten > 0 {
			return nil
		}
		body, err := json.Marshal(branchExpenseFromCentralPayload{ClientRef: clientRef, FromCentral: fromCentral})
		if err != nil {
			return fmt.Errorf("marshal sync outbox payload: %w", err)
		}
		queued = true
		return q.UpsertSyncOutboxEntry(ctx, db.UpsertSyncOutboxEntryParams{
			EntityType:  "expense_from_central",
			ClientRef:   "expense-from-central-" + clientRef,
			PayloadJson: string(body),
		})
	})
	if err != nil {
		return expense, err
	}
	if queued {
		a.kickSyncWorker()
	}
	return expense, nil
}

// executeSetExpenseFromCentralCommand applies kashi's change without
// echoing it back. kashi sends it to every register of the branch, so a ref
// this till doesn't hold is not an error -- another register has it.
func (a *App) executeSetExpenseFromCentralCommand(ctx context.Context, payload json.RawMessage) error {
	var p branchExpenseFromCentralPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("decode set_expense_from_central payload: %w", err)
	}
	p.ClientRef = strings.TrimSpace(p.ClientRef)
	if p.ClientRef == "" {
		return fmt.Errorf("client_ref is required")
	}
	if _, err := a.store.GetExpenseByClientRef(ctx, p.ClientRef); err == sql.ErrNoRows {
		return nil
	} else if err != nil {
		return fmt.Errorf("find expense: %w", err)
	}
	if _, err := a.setExpenseFromCentral(ctx, p.ClientRef, p.FromCentral, false); err != nil {
		return err
	}
	a.emitSyncApplied("expenses")
	return nil
}
