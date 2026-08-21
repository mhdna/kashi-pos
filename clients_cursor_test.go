package main

import (
	"database/sql"
	"fmt"
	"testing"
)

func TestListClientsCursorUsesStableBatches(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()
	insertTestClients(t, database, 45)

	first, err := app.ListClientsCursor(0, 20, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 20 || !first.HasMore || first.NextCursor <= 0 {
		t.Fatalf("unexpected first cursor page: %#v", first)
	}
	if first.Items[0]["name"] != "Client 000" || first.Items[19]["name"] != "Client 019" {
		t.Fatalf("unexpected first cursor items: %#v", first.Items)
	}

	second, err := app.ListClientsCursor(first.NextCursor, 20, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 20 || !second.HasMore || second.Items[0]["name"] != "Client 020" {
		t.Fatalf("unexpected second cursor page: %#v", second)
	}

	last, err := app.ListClientsCursor(second.NextCursor, 20, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Items) != 5 || last.HasMore || last.Items[0]["name"] != "Client 040" {
		t.Fatalf("unexpected final cursor page: %#v", last)
	}

	filtered, err := app.ListClientsCursor(0, 20, " 7000011 ")
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Items) != 1 || filtered.HasMore || filtered.Items[0]["name"] != "Client 011" {
		t.Fatalf("unexpected filtered cursor page: %#v", filtered)
	}
}

func insertTestClients(t *testing.T, database *sql.DB, count int) {
	t.Helper()
	tx, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	stmt, err := tx.Prepare(`
		INSERT INTO clients (name, phone, total_loyalty_points, valid_loyalty_points)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	defer stmt.Close()
	for i := 0; i < count; i++ {
		if _, err := stmt.Exec(
			fmt.Sprintf("Client %03d", i),
			fmt.Sprintf("70000%02d", i),
			i*10,
			i,
		); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}
