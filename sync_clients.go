package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

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

	if _, err := q.UpdateClient(ctx, db.UpdateClientParams{
		ID:                 existing.ID,
		Name:               c.Name,
		Phone:              c.Phone,
		TotalLoyaltyPoints: existing.TotalLoyaltyPoints,
		ValidLoyaltyPoints: existing.ValidLoyaltyPoints,
	}); err != nil {
		return err
	}
	// A client reappearing in a cloud pull is, by definition, still known to
	// kashi -- reactivate it if a previous full-mirror sync had deactivated
	// it locally.
	return q.ActivateClientByPhone(ctx, c.Phone)
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

// queueBranchClientSync upserts the single sync_outbox row for one client,
// in the same transaction as the write that produced it (see
// queueBranchInvoiceSync in sync_outbox.go for why that matters). No-op if
// this install isn't configured to sync.
//
// The ref is stable ("client-<id>"): a client that buys and gets edited
// many times keeps re-arming one row rather than piling up an
// unbounded history of throwaway rows -- kashi's putBranchClient upserts
// by (branch_id, branch_client_id) regardless, so the latest payload wins.
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

	return q.UpsertSyncOutboxEntry(ctx, db.UpsertSyncOutboxEntryParams{
		EntityType:  "client",
		ClientRef:   fmt.Sprintf("client-%d", clientID),
		PayloadJson: string(body),
	})
}

// queueBranchClientSyncForPurchase pushes a client up because they've just
// bought something -- the branch's rule is "only sync clients who actually
// transact". No-op for the walk-in pseudo-client, an unset client, or an
// unconfigured install. Called from the checkout paths right before the
// invoice is queued, so the client row lands first and kashi can attribute
// the invoice immediately.
func queueBranchClientSyncForPurchase(ctx context.Context, q *db.Queries, centralAPIURL string, clientID int64) error {
	if centralAPIURL == "" || clientID <= 0 {
		return nil
	}
	client, err := q.GetClientByID(ctx, clientID)
	if err != nil {
		return fmt.Errorf("load client for sync: %w", err)
	}
	if isWalkInClient(client) {
		return nil
	}
	return queueBranchClientSync(ctx, q, centralAPIURL, client.ID, client.Name, client.Phone)
}
