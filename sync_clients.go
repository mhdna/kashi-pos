package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	db "kashi-pos/db/sqlc"
)

// syncedClient mirrors the JSON shape kashi's GET /branch/sync/changes
// returns for the "clients" entity (see ListClientsUpdatedSince in kashi's
// db/query/client.sql). Matched locally by phone, never by kashi's numeric
// id -- see the package-level note on syncedCurrency in sync_pull.go for
// why matching is never done by id across these two separate id spaces.
type syncedClient struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// applySyncedClient creates or updates a local client to match what kashi
// reports. On a match, kashi's name wins (kashi is authoritative for the
// shared identity once a client is known there -- the same rule every
// other synced entity follows), but total_loyalty_points/
// valid_loyalty_points are left exactly as this branch already has them:
// kashi has no opinion on a branch's own loyalty bookkeeping, only on
// aggregating it at redemption time (see kashi's SumClientLoyaltyPoints).
func applySyncedClient(ctx context.Context, q *db.Queries, c syncedClient) error {
	existing, err := q.GetClientByPhone(ctx, c.Phone)
	if err == sql.ErrNoRows {
		_, err := q.CreateClient(ctx, db.CreateClientParams{
			Name:               c.Name,
			Phone:              c.Phone,
			TotalLoyaltyPoints: 0,
			ValidLoyaltyPoints: 0,
		})
		return err
	}
	if err != nil {
		return err
	}

	_, err = q.UpdateClient(ctx, db.UpdateClientParams{
		ID:                 existing.ID,
		Name:               c.Name,
		Phone:              c.Phone,
		TotalLoyaltyPoints: existing.TotalLoyaltyPoints,
		ValidLoyaltyPoints: existing.ValidLoyaltyPoints,
	})
	return err
}

// branchClientPayload is the exact JSON body PUT to kashi's /branch/clients
// (see branchClientRequest in kashi's api/branch_client.go). BranchClientID
// is this install's own local client id -- opaque to kashi, used only to
// remember which of this branch's clients maps to which kashi client (see
// kashi's client_links table) so a later edit updates the same one instead
// of creating a duplicate link.
type branchClientPayload struct {
	BranchClientID int64  `json:"branch_client_id"`
	Name           string `json:"name"`
	Phone          string `json:"phone"`
}

// queueBranchClientSync inserts a sync_outbox row for a client create/edit,
// using the same *db.Queries (and therefore the same transaction) as the
// write that produced it -- see queueBranchInvoiceSync in sync_outbox.go
// for why that matters. No-op if this install isn't configured to sync.
func queueBranchClientSync(ctx context.Context, q *db.Queries, centralAPIURL string, clientID int64, name, phone string) error {
	if centralAPIURL == "" {
		return nil
	}

	body, err := json.Marshal(branchClientPayload{
		BranchClientID: clientID,
		Name:           name,
		Phone:          phone,
	})
	if err != nil {
		return fmt.Errorf("marshal client sync payload: %w", err)
	}

	// client_ref must be unique per sync_outbox row but has no natural
	// idempotency meaning for a client edit the way it does for an
	// invoice (a client can legitimately be edited many times) -- a
	// fresh id per edit is correct here, not a bug.
	_, err = q.CreateSyncOutboxEntry(ctx, db.CreateSyncOutboxEntryParams{
		EntityType:  "client",
		ClientRef:   fmt.Sprintf("client-%d-%d", clientID, time.Now().UnixNano()),
		PayloadJson: string(body),
	})
	return err
}
