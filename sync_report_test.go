package main

import (
	"errors"
	"testing"
)

func TestSyncReportEntities(t *testing.T) {
	var r syncReporter
	boom := errors.New("FOREIGN KEY constraint failed")

	got := r.syncReportEntities([]syncPullResult{
		{entity: "currencies"},
		{entity: "products", applied: 12},
		{entity: "discount_lists", err: boom},
	})
	if len(got) != 2 || got[0].Entity != "products" || got[0].RowsApplied != 12 || got[1].Error != boom.Error() {
		t.Fatalf("first pass = %+v", got)
	}

	// The same error again is old news, and nothing else happened.
	if got := r.syncReportEntities([]syncPullResult{{entity: "discount_lists", err: boom}}); len(got) != 0 {
		t.Fatalf("repeat = %+v", got)
	}

	// Fixed: kashi hears it has cleared, even with nothing applied.
	got = r.syncReportEntities([]syncPullResult{{entity: "discount_lists"}})
	if len(got) != 1 || got[0].Error != "" {
		t.Fatalf("cleared = %+v", got)
	}
	if got := r.syncReportEntities([]syncPullResult{{entity: "discount_lists"}}); len(got) != 0 {
		t.Fatalf("after clearing = %+v", got)
	}
}
