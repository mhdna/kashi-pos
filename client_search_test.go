package main

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"

	_ "modernc.org/sqlite"
)

func TestSearchClientsIsBoundedAndIncludesSelectedClient(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "clients.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}

	store := db.NewStore(database)
	var selectedID int64
	for i := 1; i <= 40; i++ {
		client, createErr := store.CreateClient(ctx, db.CreateClientParams{
			Name: fmt.Sprintf("Client %02d", i), Phone: fmt.Sprintf("555%05d", i),
		})
		if createErr != nil {
			t.Fatal(createErr)
		}
		if i == 40 {
			selectedID = client.ID
		}
	}

	app := &App{ctx: ctx, store: store}
	results, err := app.SearchClients("", selectedID)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) > 25 {
		t.Fatalf("result count = %d, want at most 25", len(results))
	}
	if len(results) == 0 || results[0]["id"] != selectedID {
		t.Fatalf("selected client %d was not included first: %#v", selectedID, results)
	}

	results, err = app.SearchClients("Client 31", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0]["name"] != "Client 31" {
		t.Fatalf("search results = %#v, want Client 31", results)
	}
}
