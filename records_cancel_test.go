package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	db "kashi-pos/db/sqlc"
)

func TestCancelRecordCommand_StampsWithoutEchoingToKashi(t *testing.T) {
	f := newRemoteReturnFixture(t)
	ctx := context.Background()
	loan, err := f.store.CreateLoan(ctx, db.CreateLoanParams{
		ClientRef: "loan-1", Description: "float", Category: "Owner",
		Amount: 5000, CurrencyCode: "USD", CashboxAccountID: f.cashbox.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	cmd := branchCommand{Type: "cancel_record", Payload: json.RawMessage(`{"kind":"loan","client_ref":"loan-1"}`)}
	if err := f.app.executeCommand(ctx, cmd); err != nil {
		t.Fatalf("cancel_record: %v", err)
	}
	// A repeat, and a ref another register holds, are both fine.
	if err := f.app.executeCommand(ctx, cmd); err != nil {
		t.Fatalf("repeat cancel_record: %v", err)
	}
	other := branchCommand{Type: "cancel_record", Payload: json.RawMessage(`{"kind":"loan","client_ref":"nope"}`)}
	if err := f.app.executeCommand(ctx, other); err != nil {
		t.Fatalf("cancel_record for an unknown ref: %v", err)
	}

	got, err := f.store.GetLoan(ctx, loan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.CancelledAt.Valid {
		t.Fatal("loan not stamped cancelled")
	}
	var queued int
	if err := f.database.QueryRow(`SELECT COUNT(*) FROM sync_outbox WHERE entity_type = 'loan_cancel'`).Scan(&queued); err != nil {
		t.Fatal(err)
	}
	if queued != 0 {
		t.Fatalf("a cancel from kashi queued %d pushes back to kashi, want 0", queued)
	}
}

func TestCancelRecord_QueuesPushAndLeavesTotals(t *testing.T) {
	f := newRemoteReturnFixture(t)
	ctx := context.Background()
	if _, err := f.store.CreateExpense(ctx, db.CreateExpenseParams{
		ClientRef: "exp-1", Description: "tape", Category: "Supplies",
		Amount: 700, CurrencyCode: "USD", CashboxAccountID: f.cashbox.ID,
	}); err != nil {
		t.Fatal(err)
	}

	if err := f.app.cancelRecord(ctx, recordKindExpense, "exp-1", true); err != nil {
		t.Fatalf("cancelRecord: %v", err)
	}
	if err := f.app.cancelRecord(ctx, recordKindExpense, "exp-1", true); err != nil {
		t.Fatalf("repeat cancelRecord: %v", err)
	}
	if err := f.app.cancelRecord(ctx, recordKindExpense, "missing", true); err == nil {
		t.Fatal("cancelling an expense this till doesn't hold should fail")
	}

	var payload string
	var queued int
	if err := f.database.QueryRow(`SELECT COUNT(*), COALESCE(MAX(payload_json), '') FROM sync_outbox WHERE entity_type = 'expense_cancel'`).Scan(&queued, &payload); err != nil {
		t.Fatal(err)
	}
	if queued != 1 || payload != `{"client_ref":"exp-1"}` {
		t.Fatalf("outbox: %d rows, payload %q; want one cancel for exp-1", queued, payload)
	}

	totals, err := f.store.SumExpensesByCurrencySince(ctx, "2000-01-01 00:00:00")
	if err != nil {
		t.Fatal(err)
	}
	if len(totals) != 0 {
		t.Fatalf("a cancelled expense is still in the month total: %+v", totals)
	}
	if _, err := f.store.GetExpenseByClientRef(ctx, "exp-1"); err == sql.ErrNoRows {
		t.Fatal("cancelled expense must stay in the table")
	}
}
