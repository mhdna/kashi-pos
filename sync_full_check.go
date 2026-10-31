package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	db "kashi-pos/db/sqlc"
)

// The completeness check. The regular pull (sync_pull.go) follows kashi's
// updated_at cursor, so it only ever sees rows that changed after the
// cursor -- a row this till never received (a failed apply that a later
// page skipped past, a restored backup, a till that joined late) would stay
// missing forever. This pass compares the till's full set of client and
// product uids against kashi's and fetches whatever is missing or older
// than kashi's copy, so every till converges on the same copy of kashi's
// clients and products. It also adopts kashi's uid for clients this branch
// had linked before uids existed.

const (
	fullCheckInterval      = 6 * time.Hour
	fullCheckRetryInterval = 30 * time.Minute
	syncIDsPageLimit       = 5000
	syncFetchChunk         = 1000
)

// fullCheckEntities are the kashi-owned entities that must exist on every
// till, identical.
var fullCheckEntities = []string{"clients", "products"}

type syncIDEntry struct {
	UID       string    `json:"uid"`
	UpdatedAt time.Time `json:"updated_at"`
}

type syncIDsResponse struct {
	IDs  []syncIDEntry `json:"ids"`
	Next string        `json:"next"`
}

type clientUIDPair struct {
	BranchClientID int64  `json:"branch_client_id"`
	UID            string `json:"uid"`
}

// doJSON sends in (if non-nil) as a JSON body and decodes a 2xx response
// into out. Non-2xx comes back as responseError, like every other call.
func (c *centralClient) doJSON(ctx context.Context, method, path string, in, out any) error {
	if !c.enabled() {
		return errCentralSyncDisabled
	}
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := c.newRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError(method, path, resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s response: %w", path, err)
	}
	return nil
}

// SyncIDs returns one page of kashi's uid/updated_at list for entity.
func (c *centralClient) SyncIDs(ctx context.Context, entity, after string) (syncIDsResponse, error) {
	var out syncIDsResponse
	err := c.doJSON(ctx, http.MethodPost, "/branch/sync/ids",
		map[string]any{"entity": entity, "after": after, "limit": syncIDsPageLimit}, &out)
	return out, err
}

// SyncFetch returns kashi's full catch-up rows for the given uids, in the
// same shape /branch/sync/changes uses for that entity.
func (c *centralClient) SyncFetch(ctx context.Context, entity string, uids []string) (json.RawMessage, error) {
	var out struct {
		Items json.RawMessage `json:"items"`
	}
	err := c.doJSON(ctx, http.MethodPost, "/branch/sync/fetch",
		map[string]any{"entity": entity, "uids": uids}, &out)
	return out.Items, err
}

// ClientUIDs returns this branch's (local client id -> kashi uid) links.
func (c *centralClient) ClientUIDs(ctx context.Context) ([]clientUIDPair, error) {
	var out struct {
		Pairs []clientUIDPair `json:"pairs"`
	}
	err := c.doJSON(ctx, http.MethodGet, "/branch/sync/client_uids", nil, &out)
	return out.Pairs, err
}

// adoptBranchClientUIDs gives every local client this branch had already
// linked to a kashi client (before uids existed) that client's uid. Done
// before a pull, it makes the pull match those clients by uid instead of by
// phone -- which matters exactly when the phones differ (kashi trimmed it,
// replaced an unusable one, or an admin edited it). If the till already
// holds a separate copy under that uid (pulled by phone), the two are the
// same person and are merged, keeping the linked row and its history.
func (a *App) adoptBranchClientUIDs(ctx context.Context, client *centralClient) (int, error) {
	pairs, err := client.ClientUIDs(ctx)
	if err != nil {
		return 0, err
	}
	adopted := 0
	for start := 0; start < len(pairs); start += syncFetchChunk {
		end := min(start+syncFetchChunk, len(pairs))
		chunk := pairs[start:end]
		if err := a.resyncTx(ctx, func(q *db.Queries) error {
			for _, p := range chunk {
				changed, err := adoptClientUID(ctx, q, p.BranchClientID, p.UID)
				if err != nil {
					return fmt.Errorf("client %d: %w", p.BranchClientID, err)
				}
				if changed {
					adopted++
				}
			}
			return nil
		}); err != nil {
			return adopted, err
		}
	}
	return adopted, nil
}

// adoptClientUID stamps uid on local client localID, merging away any other
// local row that already carries it. It reports whether anything changed.
func adoptClientUID(ctx context.Context, q *db.Queries, localID int64, uid string) (bool, error) {
	if uid == "" {
		return false, nil
	}
	local, err := q.GetClientByID(ctx, localID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil // deleted or merged locally since
	}
	if err != nil {
		return false, err
	}
	if local.Uid.String == uid {
		return false, nil
	}
	holder, err := q.GetClientByUID(ctx, sql.NullString{String: uid, Valid: true})
	switch {
	case err == nil:
		if err := mergeLocalClient(ctx, q, local.ID, holder.ID); err != nil {
			return false, err
		}
	case !errors.Is(err, sql.ErrNoRows):
		return false, err
	}
	return true, q.SetClientUID(ctx, db.SetClientUIDParams{Uid: sql.NullString{String: uid, Valid: true}, ID: local.ID})
}

// FullCheckEntityReport is one entity's outcome of a completeness check.
type FullCheckEntityReport struct {
	Entity      string `json:"entity"`
	Central     int    `json:"central"`
	Fetched     int    `json:"fetched"`
	Deactivated int    `json:"deactivated"`
}

// FullCheckReport is what a completeness check did, kept in sync_state for
// the settings page.
type FullCheckReport struct {
	AdoptedClientUIDs int                     `json:"adoptedClientUids"`
	Entities          []FullCheckEntityReport `json:"entities"`
	StartedAt         time.Time               `json:"startedAt"`
	FinishedAt        time.Time               `json:"finishedAt"`
}

// maybeRunFullCheck runs the completeness check from the pull loop when it
// has never run or last ran more than fullCheckInterval ago. It skips
// (rather than waits) if another pull holds the lock; the next tick retries.
func (a *App) maybeRunFullCheck(ctx context.Context, client *centralClient) {
	st, err := a.store.GetSyncState(ctx)
	if err != nil {
		return
	}
	if st.LastFullCheckAt.Valid {
		if last, err := time.Parse(time.RFC3339, st.LastFullCheckAt.String); err == nil && time.Since(last) < fullCheckInterval {
			return
		}
	}
	if !a.syncPullMu.TryLock() {
		return
	}
	defer a.syncPullMu.Unlock()
	if !a.fullCheckAttemptAt.IsZero() && time.Since(a.fullCheckAttemptAt) < fullCheckRetryInterval {
		return
	}
	a.fullCheckAttemptAt = time.Now()
	if _, err := a.runFullCheckLocked(ctx, client); err != nil {
		log.Printf("sync full check: %v", err)
	}
}

// runFullCheckLocked is the completeness check itself; the caller holds
// syncPullMu. The first run on a till also adopts kashi's uids for the
// clients this branch had linked (see adoptBranchClientUIDs).
func (a *App) runFullCheckLocked(ctx context.Context, client *centralClient) (FullCheckReport, error) {
	report := FullCheckReport{StartedAt: time.Now().UTC()}

	st, err := a.store.GetSyncState(ctx)
	if err != nil {
		return report, err
	}
	if !st.IdentityAdoptedAt.Valid {
		n, err := a.adoptBranchClientUIDs(ctx, client)
		if err != nil {
			return report, fmt.Errorf("adopt client uids: %w", err)
		}
		report.AdoptedClientUIDs = n
		now := sql.NullString{String: time.Now().UTC().Format(time.RFC3339), Valid: true}
		if err := retryBusy(func() error { return a.store.SetIdentityAdoptedAt(ctx, now) }); err != nil {
			return report, err
		}
	}

	settings, err := a.store.GetSettings(ctx)
	if err != nil {
		return report, err
	}
	// Local products kashi doesn't have are deactivated only once the
	// operator has confirmed full-mirror mode (the Sync All dialog); until
	// then they're left alone.
	deactivateOrphans := settings.FullMirrorConfirmedAt.Valid

	for _, entity := range fullCheckEntities {
		rep, err := a.fullCheckEntity(ctx, client, entity, deactivateOrphans)
		if err != nil {
			return report, fmt.Errorf("%s: %w", entity, err)
		}
		report.Entities = append(report.Entities, rep)
		if rep.Fetched > 0 || rep.Deactivated > 0 {
			a.emitSyncApplied(entity)
		}
	}

	report.FinishedAt = time.Now().UTC()
	reportJSON, _ := json.Marshal(report)
	if err := retryBusy(func() error {
		return a.store.SetLastFullCheck(ctx, db.SetLastFullCheckParams{
			LastFullCheckAt:   sql.NullString{String: report.FinishedAt.Format(time.RFC3339), Valid: true},
			LastFullCheckJson: string(reportJSON),
		})
	}); err != nil {
		return report, err
	}
	a.emitSyncStateChanged()
	return report, nil
}

// localVersion is what the till holds for one uid.
type localVersion struct {
	id              int64
	remoteUpdatedAt time.Time // zero = never applied from kashi
	active          bool
}

func (a *App) localVersions(ctx context.Context, entity string) (map[string]localVersion, error) {
	out := map[string]localVersion{}
	parse := func(s sql.NullString) time.Time {
		if !s.Valid {
			return time.Time{}
		}
		t, _ := time.Parse(time.RFC3339Nano, s.String)
		return t
	}
	switch entity {
	case "clients":
		rows, err := a.store.ListClientSyncVersions(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out[r.Uid.String] = localVersion{id: r.ID, remoteUpdatedAt: parse(r.RemoteUpdatedAt), active: true}
		}
	case "products":
		rows, err := a.store.ListProductSyncVersions(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out[r.Uid.String] = localVersion{id: r.ID, remoteUpdatedAt: parse(r.RemoteUpdatedAt), active: r.IsActive != 0}
		}
	}
	return out, nil
}

// fullCheckEntity brings one entity to parity with kashi: fetch every uid
// the till lacks or holds an older version of, then (products only, and
// only once full-mirror mode is confirmed) deactivate active local products
// whose uid kashi doesn't have. Clients kashi doesn't have are left alone --
// they're this till's own new clients, on their way up through the outbox.
func (a *App) fullCheckEntity(ctx context.Context, client *centralClient, entity string, deactivateOrphans bool) (FullCheckEntityReport, error) {
	rep := FullCheckEntityReport{Entity: entity}

	remote := map[string]time.Time{}
	for after := ""; ; {
		page, err := client.SyncIDs(ctx, entity, after)
		if err != nil {
			return rep, err
		}
		for _, id := range page.IDs {
			remote[id.UID] = id.UpdatedAt
		}
		if page.Next == "" {
			break
		}
		after = page.Next
	}
	rep.Central = len(remote)

	local, err := a.localVersions(ctx, entity)
	if err != nil {
		return rep, err
	}
	var need []string
	for uid, remoteAt := range remote {
		lv, ok := local[uid]
		// Stamps are stored at full precision, so any difference (not just
		// "older") means the till's copy isn't kashi's current one.
		if !ok || !lv.remoteUpdatedAt.Equal(remoteAt) {
			need = append(need, uid)
		}
	}

	for start := 0; start < len(need); start += syncFetchChunk {
		end := min(start+syncFetchChunk, len(need))
		items, err := client.SyncFetch(ctx, entity, need[start:end])
		if err != nil {
			return rep, err
		}
		if err := a.resyncTx(ctx, func(q *db.Queries) error {
			return applySyncedEntityBatch(ctx, q, entity, items)
		}); err != nil {
			return rep, err
		}
		var decoded []json.RawMessage
		if json.Unmarshal(items, &decoded) == nil {
			rep.Fetched += len(decoded)
		}
	}

	// An empty list from kashi is far likelier to be a misconfigured or
	// freshly-reset server than a real catalog of zero products; never
	// deactivate a till's whole catalog on that.
	if entity != "products" || !deactivateOrphans || len(remote) == 0 {
		return rep, nil
	}
	// Re-read: the fetch above may have stamped kashi's uid onto rows that
	// were only matched by barcode.
	local, err = a.localVersions(ctx, entity)
	if err != nil {
		return rep, err
	}
	// The built-in "Unknown item" placeholder only ever lives on the till, so
	// kashi never has its uid; it isn't an orphan.
	var placeholderID int64
	if p, err := a.store.GetProductByBarcode(ctx, defaultUnknownBarcode); err == nil {
		placeholderID = p.ID
	}
	var orphans []int64
	for uid, lv := range local {
		if _, ok := remote[uid]; !ok && lv.active && lv.id != placeholderID {
			orphans = append(orphans, lv.id)
		}
	}
	for start := 0; start < len(orphans); start += syncFetchChunk {
		end := min(start+syncFetchChunk, len(orphans))
		ids := orphans[start:end]
		if err := retryBusy(func() error { return a.store.DeactivateProductsByIDs(ctx, ids) }); err != nil {
			return rep, err
		}
	}
	rep.Deactivated = len(orphans)
	return rep, nil
}

// seedCursorFromLocalCatalog spares a till that already holds kashi's
// clients/products (a copied database, a register added to a branch that
// has them) the full re-download an empty cursor would otherwise cost. It
// runs the completeness check for the entity -- which fetches only the uids
// the till lacks or holds a different version of -- and stores the cursor,
// returning it so the caller's regular pull only has to cover what changed
// since. "" means nothing was seeded and the caller pulls from the
// beginning as before.
//
// The cursor is kashi's clock read *before* the uid listing, so a row that
// changes while the check runs is newer than the cursor and lands in the
// next pull. Nothing local is deactivated here: orphan handling stays with
// the regular full check.
func (a *App) seedCursorFromLocalCatalog(ctx context.Context, client *centralClient, entity string) (string, error) {
	if entity != "clients" && entity != "products" {
		return "", nil
	}
	local, err := a.localVersions(ctx, entity)
	if err != nil {
		return "", err
	}
	stamped := false
	for _, lv := range local {
		if !lv.remoteUpdatedAt.IsZero() {
			stamped = true
			break
		}
	}
	if !stamped {
		return "", nil
	}

	// A since in the future matches no row, so this is a cheap way to read
	// kashi's clock (server_time) without a dedicated endpoint.
	probe, err := client.FetchChanges(ctx, fetchChangesParams{
		Entity: entity,
		Since:  time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
		Limit:  1,
	})
	if err != nil {
		return "", err
	}
	rep, err := a.fullCheckEntity(ctx, client, entity, false)
	if err != nil {
		return "", err
	}
	cursor := probe.ServerTime.Format(time.RFC3339)
	if err := retryBusy(func() error {
		return a.store.SetSyncCursor(ctx, db.SetSyncCursorParams{Entity: entity, Cursor: cursor})
	}); err != nil {
		return "", err
	}
	if rep.Fetched > 0 {
		a.emitSyncApplied(entity)
	}
	return cursor, nil
}
