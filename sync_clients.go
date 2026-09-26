package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	db "kashi-pos/db/sqlc"
)

// syncedClient mirrors the JSON shape kashi's GET /branch/sync/changes
// (and /branch/sync/fetch) returns for the "clients" entity. UID is the
// shared identity (kashi's 000093_sync_uid) and is what a local row is
// matched on; phone is only the fallback for a row that hasn't adopted
// kashi's uid yet, or for an older kashi that doesn't send one. IsActive is
// nil from an older kashi (treated as active).
type syncedClient struct {
	UID      string `json:"uid"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	IsActive *bool  `json:"is_active"`
}

// applySyncedClient makes the local copy of one kashi client match kashi:
// same uid, name, phone and active flag. Loyalty points are left exactly as
// this branch has them -- kashi has no opinion on a branch's own loyalty
// bookkeeping, only on aggregating it at redemption time.
//
// Matching: by uid, else by phone (then kashi's uid is stamped on the row),
// else a new local client is created under kashi's uid. If the phone kashi
// reports is held locally by a different row, that row is the same person
// (phone is kashi's global client key, and updates arrive in updated_at
// order, so a genuine phone swap between two clients is already applied by
// now) -- it is merged into the matched row rather than duplicated.
func applySyncedClient(ctx context.Context, q *db.Queries, c syncedClient) error {
	active := int64(1)
	if c.IsActive != nil && !*c.IsActive {
		active = 0
	}

	var target db.Client
	found := false
	if c.UID != "" {
		byUID, err := q.GetClientByUID(ctx, sql.NullString{String: c.UID, Valid: true})
		switch {
		case err == nil:
			target, found = byUID, true
		case !errors.Is(err, sql.ErrNoRows):
			return err
		}
	}
	if !found {
		byPhone, err := q.GetClientByPhone(ctx, c.Phone)
		switch {
		case err == nil:
			target, found = byPhone, true
			if c.UID != "" {
				if err := q.SetClientUID(ctx, db.SetClientUIDParams{Uid: sql.NullString{String: c.UID, Valid: true}, ID: byPhone.ID}); err != nil {
					return err
				}
			}
		case errors.Is(err, sql.ErrNoRows):
			created, err := q.CreateClient(ctx, db.CreateClientParams{Name: c.Name, Phone: c.Phone})
			if err != nil {
				return err
			}
			if c.UID != "" {
				if err := q.SetClientUID(ctx, db.SetClientUIDParams{Uid: sql.NullString{String: c.UID, Valid: true}, ID: created.ID}); err != nil {
					return err
				}
			}
			return q.SetClientActive(ctx, db.SetClientActiveParams{IsActive: active, ID: created.ID})
		default:
			return err
		}
	}

	if target.Phone != c.Phone {
		other, err := q.GetClientByPhone(ctx, c.Phone)
		switch {
		case err == nil && other.ID != target.ID:
			if err := mergeLocalClient(ctx, q, target.ID, other.ID); err != nil {
				return fmt.Errorf("merge local client %d into %d: %w", other.ID, target.ID, err)
			}
		case err != nil && !errors.Is(err, sql.ErrNoRows):
			return err
		}
	}

	// Re-read: a merge above may have moved loyalty points onto the row.
	current, err := q.GetClientByID(ctx, target.ID)
	if err != nil {
		return err
	}
	if _, err := q.UpdateClient(ctx, db.UpdateClientParams{
		ID:                 current.ID,
		Name:               c.Name,
		Phone:              c.Phone,
		TotalLoyaltyPoints: current.TotalLoyaltyPoints,
		ValidLoyaltyPoints: current.ValidLoyaltyPoints,
	}); err != nil {
		return err
	}
	return q.SetClientActive(ctx, db.SetClientActiveParams{IsActive: active, ID: current.ID})
}

// mergeLocalClient folds the duplicate local client dropID into keepID:
// every invoice, coupon and account entry is re-pointed, loyalty points are
// added together, the duplicate's unsent outbox row is dropped (keepID is
// the one kashi knows), and only then -- with nothing referencing it -- is
// the duplicate deleted. The walk-in client is never merged.
func mergeLocalClient(ctx context.Context, q *db.Queries, keepID, dropID int64) error {
	keep, err := q.GetClientByID(ctx, keepID)
	if err != nil {
		return err
	}
	drop, err := q.GetClientByID(ctx, dropID)
	if err != nil {
		return err
	}
	if isWalkInClient(keep) || isWalkInClient(drop) {
		return fmt.Errorf("refusing to merge the walk-in client")
	}
	ids := db.RepointInvoicesClientParams{KeepID: keepID, DropID: dropID}
	if err := q.RepointInvoicesClient(ctx, ids); err != nil {
		return err
	}
	if err := q.RepointCouponsClient(ctx, db.RepointCouponsClientParams(ids)); err != nil {
		return err
	}
	if err := q.RepointClientAccountEntriesClient(ctx, db.RepointClientAccountEntriesClientParams(ids)); err != nil {
		return err
	}
	if err := q.DeleteUnsyncedSyncOutboxEntry(ctx, fmt.Sprintf("client-%d", dropID)); err != nil {
		return err
	}
	// Free the duplicate's phone before the keeper may take it over.
	if err := q.DeleteClient(ctx, dropID); err != nil {
		return err
	}
	_, err = q.UpdateClient(ctx, db.UpdateClientParams{
		ID:                 keep.ID,
		Name:               keep.Name,
		Phone:              keep.Phone,
		TotalLoyaltyPoints: keep.TotalLoyaltyPoints + drop.TotalLoyaltyPoints,
		ValidLoyaltyPoints: keep.ValidLoyaltyPoints + drop.ValidLoyaltyPoints,
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
	// UID is the client's shared identity. kashi keeps it for a new client,
	// or answers with the canonical uid when the phone already belongs to
	// an existing client (see adoptCanonicalClientUID).
	UID string `json:"uid,omitempty"`
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
func queueBranchClientSync(ctx context.Context, q *db.Queries, centralAPIURL string, client db.Client) error {
	if centralAPIURL == "" {
		return nil
	}
	clientID := client.ID

	body, err := json.Marshal(branchClientPayload{
		BranchClientID: client.ID,
		Name:           client.Name,
		Phone:          client.Phone,
		UID:            client.Uid.String,
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
	return queueBranchClientSync(ctx, q, centralAPIURL, client)
}

// adoptCanonicalClientUID applies kashi's answer to a client push: the uid
// kashi now knows this local client by. Usually that's the uid the till
// sent. When kashi instead matched the phone to an existing client, the
// local row takes that client's uid -- or, if the till already holds a copy
// of that client (pulled earlier), the pushed row is a duplicate of it and
// is merged into it.
func adoptCanonicalClientUID(ctx context.Context, q *db.Queries, localID int64, canonical string) error {
	local, err := q.GetClientByID(ctx, localID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // merged away since it was queued
	}
	if err != nil {
		return err
	}
	if canonical == "" || local.Uid.String == canonical {
		return nil
	}
	holder, err := q.GetClientByUID(ctx, sql.NullString{String: canonical, Valid: true})
	switch {
	case err == nil:
		return mergeLocalClient(ctx, q, holder.ID, local.ID)
	case errors.Is(err, sql.ErrNoRows):
		return q.SetClientUID(ctx, db.SetClientUIDParams{Uid: sql.NullString{String: canonical, Valid: true}, ID: local.ID})
	default:
		return err
	}
}
