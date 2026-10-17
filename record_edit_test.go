package main

import (
	"encoding/json"
	"testing"

	db "kashi-pos/db/sqlc"
)

// A category edit rides along with an expense kashi doesn't have yet, and
// goes up on its own once kashi has it -- never ahead of the expense.
func TestUpdateExpenseCategoryFollowsTheExpense(t *testing.T) {
	app, store, cashbox, _ := setUpTestInvoiceApp(t, "http://central.test")

	created, err := app.CreateExpense("Internet", "", 5000, "USD", cashbox.ID)
	if err != nil {
		t.Fatal(err)
	}
	id := created["id"].(int64)
	ref := created["clientRef"].(string)

	if _, err := app.UpdateExpenseCategory(id, "Utilities"); err != nil {
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
		t.Fatal(err)
	}
	if pending.Category != "Utilities" {
		t.Fatalf("pending expense category = %q, want Utilities", pending.Category)
	}

	if err := store.MarkSyncOutboxSynced(app.ctx, db.MarkSyncOutboxSyncedParams{SyncedTo: "http://central.test", ID: rows[0].ID}); err != nil {
		t.Fatal(err)
	}
	for _, category := range []string{"Rent", "Bills"} {
		if _, err := app.UpdateExpenseCategory(id, category); err != nil {
			t.Fatal(err)
		}
	}
	rows, err = store.ListDueSyncOutboxEntries(app.ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].EntityType != "expense_category" {
		t.Fatalf("outbox = %+v, want one expense_category row", rows)
	}
	var edit branchRecordCategoryPayload
	if err := json.Unmarshal([]byte(rows[0].PayloadJson), &edit); err != nil {
		t.Fatal(err)
	}
	if edit.ClientRef != ref || edit.Category != "Bills" {
		t.Fatalf("category edit = %+v, want the latest (Bills) for %s", edit, ref)
	}

	got, err := store.GetExpense(app.ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Category != "Bills" {
		t.Fatalf("local category = %q, want Bills", got.Category)
	}
}
