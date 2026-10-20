package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	db "kashi-pos/db/sqlc"
)

// An expense or loan keyed in by mistake is cancelled, not deleted: the row
// stays, listed as cancelled, and the shift's spend and the month totals
// stop counting it (see the cancelled_at filters in expense.sql/loan.sql).
// An admin cancels one here and the cancel is queued for kashi; kashi's own
// cancel arrives as a cancel_record command and is applied without echoing
// it back.

const (
	recordKindExpense = "expense"
	recordKindLoan    = "loan"
)

// CancelExpense cancels one of this till's expenses and tells kashi. Admin
// only, like CancelInvoice.
func (a *App) CancelExpense(id int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if !a.currentUserIsAdmin() {
		return fmt.Errorf("only an admin can cancel an expense")
	}
	expense, err := a.store.GetExpense(a.ctx, id)
	if err != nil {
		return fmt.Errorf("find expense: %w", err)
	}
	return a.cancelRecord(a.ctx, recordKindExpense, expense.ClientRef, true)
}

// CancelLoan is CancelExpense's counterpart for loans.
func (a *App) CancelLoan(id int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if !a.currentUserIsAdmin() {
		return fmt.Errorf("only an admin can cancel a loan")
	}
	loan, err := a.store.GetLoan(a.ctx, id)
	if err != nil {
		return fmt.Errorf("find loan: %w", err)
	}
	return a.cancelRecord(a.ctx, recordKindLoan, loan.ClientRef, true)
}

// cancelRecord stamps the row and, when pushToKashi is set and this install
// syncs, queues the cancel in the same transaction. Cancelling a record
// that is already cancelled is a no-op; one this till doesn't hold is an
// error.
func (a *App) cancelRecord(ctx context.Context, kind, clientRef string, pushToKashi bool) error {
	err := a.store.ExecTx(ctx, func(q *db.Queries) error {
		var err error
		switch kind {
		case recordKindExpense:
			_, err = q.CancelExpenseByClientRef(ctx, clientRef)
		case recordKindLoan:
			_, err = q.CancelLoanByClientRef(ctx, clientRef)
		default:
			return fmt.Errorf("unknown record kind %q", kind)
		}
		if err == sql.ErrNoRows {
			// Already cancelled, or never here: tell them apart.
			if err := recordExists(ctx, q, kind, clientRef); err != nil {
				return err
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("cancel %s: %w", kind, err)
		}

		if !pushToKashi {
			return nil
		}
		settings, err := q.GetSettings(ctx)
		if err != nil {
			return fmt.Errorf("load settings: %w", err)
		}
		return queueBranchRecordCancelSync(ctx, q, settings.CentralApiUrl, kind, clientRef)
	})
	if err != nil {
		return err
	}
	if pushToKashi {
		a.kickSyncWorker()
	}
	return nil
}

func recordExists(ctx context.Context, q *db.Queries, kind, clientRef string) error {
	var err error
	if kind == recordKindExpense {
		_, err = q.GetExpenseByClientRef(ctx, clientRef)
	} else {
		_, err = q.GetLoanByClientRef(ctx, clientRef)
	}
	if err != nil {
		return fmt.Errorf("%s %q not found on this till: %w", kind, clientRef, err)
	}
	return nil
}

// branchRecordCancelPayload is the exact JSON body POSTed to kashi's
// /branch/expenses/cancel and /branch/loans/cancel (see
// branchCancelRecordRequest in api/branch_cancel.go in the kashi repo).
type branchRecordCancelPayload struct {
	ClientRef string `json:"client_ref"`
}

// recordCancelEntityType and recordCancelOutboxRef name the outbox row: its
// client_ref is unique across the table, so it can't reuse the record's own.
func recordCancelEntityType(kind string) string { return kind + "_cancel" }
func recordCancelOutboxRef(kind, clientRef string) string {
	return "cancel-" + kind + "-" + clientRef
}

// queueBranchRecordCancelSync is queueBranchExpenseSync's counterpart for a
// cancel: opt-in-if-configured, same transaction as the write.
func queueBranchRecordCancelSync(ctx context.Context, q *db.Queries, centralAPIURL, kind, clientRef string) error {
	if centralAPIURL == "" {
		return nil
	}
	body, err := json.Marshal(branchRecordCancelPayload{ClientRef: clientRef})
	if err != nil {
		return fmt.Errorf("marshal sync outbox payload: %w", err)
	}
	_, err = q.CreateSyncOutboxEntry(ctx, db.CreateSyncOutboxEntryParams{
		EntityType:  recordCancelEntityType(kind),
		ClientRef:   recordCancelOutboxRef(kind, clientRef),
		PayloadJson: string(body),
	})
	return err
}

type cancelRecordPayload struct {
	Kind      string `json:"kind"`
	ClientRef string `json:"client_ref"`
}

// executeCancelRecordCommand applies kashi's cancel of an expense or loan.
// kashi queues it for every register of the branch, so a ref this till
// doesn't hold is not an error -- another register has it.
func (a *App) executeCancelRecordCommand(ctx context.Context, payload json.RawMessage) error {
	var p cancelRecordPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("decode cancel_record payload: %w", err)
	}
	p.ClientRef = strings.TrimSpace(p.ClientRef)
	if p.ClientRef == "" || (p.Kind != recordKindExpense && p.Kind != recordKindLoan) {
		return fmt.Errorf("kind (expense or loan) and client_ref are required")
	}

	var err error
	if p.Kind == recordKindExpense {
		_, err = a.store.GetExpenseByClientRef(ctx, p.ClientRef)
	} else {
		_, err = a.store.GetLoanByClientRef(ctx, p.ClientRef)
	}
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find %s: %w", p.Kind, err)
	}

	if err := a.cancelRecord(ctx, p.Kind, p.ClientRef, false); err != nil {
		return err
	}
	a.emitSyncApplied(p.Kind + "s")
	return nil
}
