package main

import (
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

func TestListBranchTargetsWithProgress_OnlyCurrentMonth(t *testing.T) {
	app, store, _, _ := setUpTestInvoiceApp(t, "")

	now := time.Now()
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
	ymd := func(t time.Time) string { return t.Format("2006-01-02") }

	seed := []struct {
		remoteID int64
		from, to time.Time
		want     bool
	}{
		// Last month ends the day before this one starts: lastOfMonth minus a
		// month overflows (Oct 31 -> "Sep 31" = Oct 1) in a 31-day month.
		{1, firstOfMonth.AddDate(0, -1, 0), firstOfMonth.AddDate(0, 0, -1), false}, // last month
		{2, firstOfMonth, lastOfMonth, true},                                       // this month
		{3, firstOfMonth.AddDate(0, 1, 0), lastOfMonth.AddDate(0, 1, 0), false},    // next month
		{4, firstOfMonth.AddDate(0, -1, 0), lastOfMonth.AddDate(0, 1, 0), true},    // quarter spanning now
	}
	for _, s := range seed {
		if err := store.UpsertBranchTarget(app.ctx, db.UpsertBranchTargetParams{
			RemoteID:          s.remoteID,
			DateFrom:          ymd(s.from),
			DateTo:            ymd(s.to),
			TargetAmountCents: 100000 * s.remoteID,
			Color:             "#4C6EF5",
		}); err != nil {
			t.Fatal(err)
		}
	}

	got, err := app.ListBranchTargetsWithProgress()
	if err != nil {
		t.Fatal(err)
	}

	gotIDs := map[int64]bool{}
	for _, row := range got {
		// UpsertBranchTarget has no RETURNING; ids are assigned in insert
		// order, so remote_id N -> local id N here.
		gotIDs[row["id"].(int64)] = true
	}
	for _, s := range seed {
		if gotIDs[s.remoteID] != s.want {
			t.Fatalf("target remote_id %d: shown=%v, want %v", s.remoteID, gotIDs[s.remoteID], s.want)
		}
	}
	if len(got) != 2 {
		t.Fatalf("expected exactly 2 current-month targets, got %d", len(got))
	}
}
