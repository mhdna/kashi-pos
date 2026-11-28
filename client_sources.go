package main

import (
	"context"
	"fmt"
	"strings"

	db "kashi-pos/db/sqlc"
)

// Client sources answer "where did the customer hear about us?", asked once
// when a cashier adds a new customer. Central owns the list (entity
// "client_sources", whole set every pull); the till's migration 000151 seeds
// central's starting options under the same uids. The answer is stored on the
// client (clients.source_uid) and sent with it (branchClientPayload), and
// central keeps the first answer a customer gets.

// syncedClientSource mirrors central's client_sources row.
type syncedClientSource struct {
	UID       string `json:"uid"`
	Name      string `json:"name"`
	SortOrder int64  `json:"sort_order"`
	IsActive  bool   `json:"is_active"`
}

// ClientSource is one option the new-customer form offers.
type ClientSource struct {
	UID  string `json:"uid"`
	Name string `json:"name"`
}

// applySyncedClientSources makes the local list exactly central's: every
// option upserted, and any option central no longer has removed. A client's
// saved answer is kept either way (it is only a uid).
func applySyncedClientSources(ctx context.Context, q *db.Queries, items []syncedClientSource) error {
	keep := make(map[string]bool, len(items))
	for _, it := range items {
		uid := strings.TrimSpace(it.UID)
		if uid == "" || strings.TrimSpace(it.Name) == "" {
			continue
		}
		keep[uid] = true
		active := int64(0)
		if it.IsActive {
			active = 1
		}
		if err := q.UpsertClientSource(ctx, db.UpsertClientSourceParams{
			Uid: uid, Name: strings.TrimSpace(it.Name), SortOrder: it.SortOrder, IsActive: active,
		}); err != nil {
			return fmt.Errorf("client source %s: %w", uid, err)
		}
	}
	local, err := q.ListClientSources(ctx)
	if err != nil {
		return err
	}
	for _, row := range local {
		if !keep[row.Uid] {
			if err := q.DeleteClientSource(ctx, row.Uid); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetClientSources lists the options the new-customer form offers, in
// central's order. Empty when central has hidden them all: the form then
// doesn't ask.
func (a *App) GetClientSources() ([]ClientSource, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := a.store.ListActiveClientSources(a.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ClientSource, 0, len(rows))
	for _, r := range rows {
		out = append(out, ClientSource{UID: r.Uid, Name: r.Name})
	}
	return out, nil
}

// knownClientSource returns uid when it names an option this till has, else
// "": an option removed while the form was open is dropped, never refused.
func knownClientSource(ctx context.Context, q *db.Queries, uid string) (string, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return "", nil
	}
	n, err := q.ClientSourceExists(ctx, uid)
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", nil
	}
	return uid, nil
}
