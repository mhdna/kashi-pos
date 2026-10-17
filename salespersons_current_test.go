package main

import (
	"context"
	"testing"
)

// Connected to kashi, a salesperson only the till has (a misspelt copy of
// one kashi has) isn't offered once kashi has sent its own; it's kept for
// its history.
func TestListSalespersonsCurrentOnceKashiManages(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "http://kashi.test")
	ctx := context.Background()
	local, err := store.CreateSalesperson(ctx, "Ahmad Robaie")
	if err != nil {
		t.Fatal(err)
	}
	current := func() map[string]bool {
		list, err := app.ListSalespersons()
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, s := range list {
			out[s["fullName"].(string)] = s["isCurrent"].(bool)
		}
		return out
	}
	if got := current(); !got["Ahmad Robaie"] {
		t.Fatalf("before kashi sent any staff, the till's own must be offered: %v", got)
	}
	if err := applySyncedSalesperson(ctx, store.Queries, syncedSalesperson{ID: 6, Name: "Ahmad Robai", IsActive: true}); err != nil {
		t.Fatal(err)
	}
	got := current()
	if !got["Ahmad Robai"] || got["Ahmad Robaie"] {
		t.Fatalf("kashi's should be offered, the till's leftover not: %v", got)
	}
	if _, err := store.GetSalespersonByID(ctx, local.ID); err != nil {
		t.Fatalf("the till's own must be kept for its history: %v", err)
	}
}
