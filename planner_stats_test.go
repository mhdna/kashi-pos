package main

import (
	"context"
	"fmt"
	"testing"

	db "kashi-pos/db/sqlc"
)

// A till analyzed while clients was nearly empty kept "1 row" stats forever
// (plain PRAGMA optimize at open looked at no table). refreshPlannerStats
// must bring them up to date, so plans stop walking every client.
func TestRefreshPlannerStatsUpdatesStaleCounts(t *testing.T) {
	ctx := context.Background()
	database := openTestDatabase(t)
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	if _, err := store.CreateClient(ctx, db.CreateClientParams{Name: "First", Phone: "+96170000000"}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`ANALYZE`); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 600; i++ {
		if _, err := store.CreateClient(ctx, db.CreateClientParams{Name: fmt.Sprintf("C%d", i), Phone: fmt.Sprintf("+9617%07d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	rowsInStats := func() int {
		var stat string
		if err := database.QueryRow(`SELECT stat FROM sqlite_stat1 WHERE tbl = 'clients' AND idx = 'idx_clients_phone'`).Scan(&stat); err != nil {
			t.Fatal(err)
		}
		var n int
		fmt.Sscanf(stat, "%d", &n)
		return n
	}
	// Seeded data counts too; what matters is the stats lag far behind.
	stale := rowsInStats()
	if stale > 50 {
		t.Fatalf("setup: stats already say %d rows", stale)
	}
	// What the till used to run: changes nothing on a fresh connection.
	if _, err := database.Exec(`PRAGMA optimize`); err != nil {
		t.Fatal(err)
	}
	refreshPlannerStats(ctx, database)
	if got := rowsInStats(); got < 600 {
		t.Fatalf("clients stats after refresh = %d rows, want the real count (600+)", got)
	}
}
