package main

import (
	"database/sql"
	"testing"
)

// Pulling the same incoming transfer twice must insert it once (with a
// non-NULL synthetic client_ref) and then update it in place by central_id.
func TestApplySyncedIncomingTransfer_UpsertsByCentralID(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "")

	transfer := syncedIncomingTransfer{
		ID:             7,
		FromBranchName: "Main",
		Note:           "first",
		Status:         "pending",
		Items:          []syncedIncomingTransferItem{{Barcode: "123", Quantity: 2}},
	}
	if err := applySyncedIncomingTransfer(app.ctx, store.Queries, transfer); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	transfer.Note = "second"
	transfer.Status = "approved"
	if err := applySyncedIncomingTransfer(app.ctx, store.Queries, transfer); err != nil {
		t.Fatalf("second apply: %v", err)
	}

	got, err := store.GetTransferRequestByCentralID(app.ctx, sql.NullInt64{Int64: 7, Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.ClientRef != "incoming-7" || got.Note != "second" || got.Status != "approved" || got.Direction != "incoming" {
		t.Fatalf("unexpected row: %+v", got)
	}
	incoming, err := store.ListIncomingTransferRequests(app.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(incoming) != 1 {
		t.Fatalf("got %d incoming transfers, want 1", len(incoming))
	}
}
