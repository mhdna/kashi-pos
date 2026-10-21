package main

import (
	"encoding/json"
	"testing"

	db "kashi-pos/db/sqlc"
)

// An expense central paid for is still the month's expense, but the shift
// doesn't take it off the drawer -- until it's marked as the till's again.
func TestExpenseFromCentralLeavesTheShiftAlone(t *testing.T) {
	app, store, cashbox, _ := setUpTestInvoiceApp(t, "")
	shift, err := store.GetOpenShift(app.ctx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := app.CreateExpense("Paint", "", 3000, "USD", cashbox.ID, false); err != nil {
		t.Fatal(err)
	}
	central, err := app.CreateExpense("Rent", "", 50000, "USD", cashbox.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if central["fromCentral"] != true {
		t.Fatalf("fromCentral = %v, want true", central["fromCentral"])
	}

	spend, err := app.shiftSpend(shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	if spend.USD != 3000 || len(spend.Lines) != 1 {
		t.Fatalf("shift spend = %+v, want only the till's 3000", spend)
	}
	summary, err := app.GetSpendSummary(shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rows := summary["monthExpenses"].([]db.SumExpensesByCurrencySinceRow); len(rows) != 1 || rows[0].Total != 53000 {
		t.Fatalf("month expenses = %+v, want both (53000)", rows)
	}

	if _, err := app.setExpenseFromCentral(app.ctx, central["clientRef"].(string), false, true); err != nil {
		t.Fatal(err)
	}
	spend, err = app.shiftSpend(shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	if spend.USD != 53000 {
		t.Fatalf("shift spend after unmarking = %d, want 53000", spend.USD)
	}
}

// Like a category edit: the mark rides along with an expense kashi doesn't
// have yet, and goes up on its own once kashi has it.
func TestExpenseFromCentralFollowsTheExpense(t *testing.T) {
	app, store, cashbox, _ := setUpTestInvoiceApp(t, "http://central.test")

	created, err := app.CreateExpense("Rent", "", 50000, "USD", cashbox.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	ref := created["clientRef"].(string)

	if _, err := app.setExpenseFromCentral(app.ctx, ref, true, true); err != nil {
		t.Fatal(err)
	}
	rows, err := store.ListDueSyncOutboxEntries(app.ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].EntityType != "expense" {
		t.Fatalf("outbox = %+v, want just the expense", rows)
	}
	var pending branchExpensePayload
	if err := json.Unmarshal([]byte(rows[0].PayloadJson), &pending); err != nil {
		t.Fatalf("pending expense payload %s: %v", rows[0].PayloadJson, err)
	}
	if !pending.FromCentral {
		t.Fatal("pending expense isn't marked from central")
	}

	if err := store.MarkSyncOutboxSynced(app.ctx, db.MarkSyncOutboxSyncedParams{SyncedTo: "http://central.test", ID: rows[0].ID}); err != nil {
		t.Fatal(err)
	}
	for _, on := range []bool{false, true} {
		if _, err := app.setExpenseFromCentral(app.ctx, ref, on, true); err != nil {
			t.Fatal(err)
		}
	}
	rows, err = store.ListDueSyncOutboxEntries(app.ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].EntityType != "expense_from_central" {
		t.Fatalf("outbox = %+v, want one expense_from_central row", rows)
	}
	var change branchExpenseFromCentralPayload
	if err := json.Unmarshal([]byte(rows[0].PayloadJson), &change); err != nil {
		t.Fatal(err)
	}
	if change.ClientRef != ref || !change.FromCentral {
		t.Fatalf("change = %+v, want the latest (true) for %s", change, ref)
	}
}

// kashi's change is applied without echoing it back; a ref another register
// holds is fine.
func TestSetExpenseFromCentralCommand(t *testing.T) {
	app, store, cashbox, _ := setUpTestInvoiceApp(t, "http://central.test")
	created, err := app.CreateExpense("Rent", "", 50000, "USD", cashbox.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	ref := created["clientRef"].(string)
	if _, err := app.conn.Exec(`UPDATE sync_outbox SET status = 'synced'`); err != nil {
		t.Fatal(err)
	}

	for _, payload := range []string{
		`{"client_ref":"` + ref + `","from_central":true}`,
		`{"client_ref":"another-register","from_central":true}`,
	} {
		cmd := branchCommand{Type: "set_expense_from_central", Payload: json.RawMessage(payload)}
		if err := app.executeCommand(app.ctx, cmd); err != nil {
			t.Fatalf("set_expense_from_central %s: %v", payload, err)
		}
	}
	got, err := store.GetExpenseByClientRef(app.ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if got.FromCentral != 1 {
		t.Fatal("expense not marked from central")
	}
	rows, err := store.ListDueSyncOutboxEntries(app.ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("a change from kashi queued %+v back to kashi, want nothing", rows)
	}
}
