package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// reconcileBatchSize is how many refs the branch asks kashi about per call.
// Kept at kashi's own per-request cap (see branchReconcileMaxRefs in
// api/branch_catchup.go) so a batch is never rejected for being too large.
const reconcileBatchSize = 500

type reconcileRequest struct {
	Entity string   `json:"entity"`
	Refs   []string `json:"refs"`
}

type reconcileResponse struct {
	Missing []string `json:"missing"`
}

// Reconcile asks kashi which of `refs` (for one entity) it does NOT already
// hold for this branch. The branch queues only those, rather than
// re-pushing everything and relying on (branch_id, client_ref) idempotency
// to swallow the duplicates. Callers batch; this is one HTTP round trip.
func (c *centralClient) Reconcile(ctx context.Context, entity string, refs []string) ([]string, error) {
	if !c.enabled() {
		return nil, errCentralSyncDisabled
	}

	body, err := json.Marshal(reconcileRequest{Entity: entity, Refs: refs})
	if err != nil {
		return nil, err
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/branch/sync/reconcile", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, responseError(http.MethodPost, "/branch/sync/reconcile", resp)
	}

	var out reconcileResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode reconcile response: %w", err)
	}
	return out.Missing, nil
}

// reconcileEntity walks `refs` in batches through Reconcile and returns the
// union of what kashi is missing, preserving input order. An empty result
// means kashi already has everything -- the "the new central server is
// already up to date" case after re-pointing the URL.
func (a *App) reconcileEntity(ctx context.Context, client *centralClient, entity string, refs []string) ([]string, error) {
	missing := make([]string, 0)
	for start := 0; start < len(refs); start += reconcileBatchSize {
		end := start + reconcileBatchSize
		if end > len(refs) {
			end = len(refs)
		}
		batchMissing, err := client.Reconcile(ctx, entity, refs[start:end])
		if err != nil {
			return nil, fmt.Errorf("reconcile %s [%d:%d]: %w", entity, start, end, err)
		}
		missing = append(missing, batchMissing...)
	}
	return missing, nil
}
