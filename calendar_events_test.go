package main

import (
	"context"
	"testing"
)

// kashi sends its whole set: what it still lists is upserted, what it
// dropped is deleted, and an empty set clears the table.
func TestApplySyncedCalendarEventsMirrorsTheWholeSet(t *testing.T) {
	ctx := context.Background()
	q := setUpTestClientDB(t).Queries

	first := []syncedCalendarEvent{
		{ID: 1, Name: "Opening", Icon: "mdi-party-popper", Color: "pink", StartDate: "2026-11-10", EndDate: "2026-11-12"},
		{ID: 2, Name: "Stocktake", StartDate: "2026-11-20", EndDate: "2026-11-20"},
	}
	if err := applySyncedCalendarEvents(ctx, q, first); err != nil {
		t.Fatal(err)
	}
	rows, _ := q.ListCalendarEvents(ctx)
	if len(rows) != 2 {
		t.Fatalf("want 2 events, got %d", len(rows))
	}

	// 1 renamed, 2 deleted centrally.
	if err := applySyncedCalendarEvents(ctx, q, []syncedCalendarEvent{
		{ID: 1, Name: "Grand opening", Icon: "mdi-party-popper", Color: "pink", StartDate: "2026-11-10", EndDate: "2026-11-12"},
	}); err != nil {
		t.Fatal(err)
	}
	rows, _ = q.ListCalendarEvents(ctx)
	if len(rows) != 1 || rows[0].Name != "Grand opening" {
		t.Fatalf("got %+v", rows)
	}

	if err := applySyncedCalendarEvents(ctx, q, nil); err != nil {
		t.Fatal(err)
	}
	rows, _ = q.ListCalendarEvents(ctx)
	if len(rows) != 0 {
		t.Fatalf("empty set should clear, got %d", len(rows))
	}
}

func TestApplySyncedCalendarEventsRejectsBadDates(t *testing.T) {
	q := setUpTestClientDB(t).Queries
	err := applySyncedCalendarEvents(context.Background(), q, []syncedCalendarEvent{
		{ID: 1, Name: "x", StartDate: "2026-11-12", EndDate: "2026-11-10"},
	})
	if err == nil {
		t.Fatal("end before start should be rejected")
	}
}
