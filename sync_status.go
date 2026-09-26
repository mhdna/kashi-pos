package main

import "fmt"

// SyncStatusResult is the global sync health the till's nav-rail dot and the
// settings page read. "Green" (fully synced) is: configured, nothing
// pending, nothing failed, and nothing still stamped for a previous server.
type SyncStatusResult struct {
	Configured         bool   `json:"configured"`
	Target             string `json:"target"`
	Pending            int64  `json:"pending"`
	Failed             int64  `json:"failed"`
	ConfirmedOnTarget  int64  `json:"confirmedOnTarget"`
	ConfirmedElsewhere int64  `json:"confirmedElsewhere"`
	LastSyncedAt       string `json:"lastSyncedAt"`
	LastReconcileAt    string `json:"lastReconcileAt"`
	// LastFullCheckAt is when the completeness check last finished (see
	// sync_full_check.go); empty if it never has.
	LastFullCheckAt string `json:"lastFullCheckAt"`
	FullySynced     bool   `json:"fullySynced"`
}

// GetSyncStatus is the Wails binding behind the sync indicator. Cheap
// enough to call on every "sync_state_changed" event.
func (a *App) GetSyncStatus() (SyncStatusResult, error) {
	if a.store == nil {
		return SyncStatusResult{}, fmt.Errorf("database not initialized")
	}
	ctx := a.ctx

	settings, err := a.store.GetSettings(ctx)
	if err != nil {
		return SyncStatusResult{}, err
	}
	target := trimSlash(settings.CentralApiUrl)

	res := SyncStatusResult{
		Configured: target != "" && settings.BranchCode != "" && settings.ApiKey != "",
		Target:     target,
	}

	if res.Pending, err = a.store.CountSyncOutboxByStatus(ctx, "pending"); err != nil {
		return res, err
	}
	if res.Failed, err = a.store.CountSyncOutboxByStatus(ctx, "failed"); err != nil {
		return res, err
	}
	if res.ConfirmedOnTarget, err = a.store.CountSyncOutboxConfirmedOnTarget(ctx, target); err != nil {
		return res, err
	}
	if res.ConfirmedElsewhere, err = a.store.CountSyncOutboxConfirmedElsewhere(ctx, target); err != nil {
		return res, err
	}
	if res.LastSyncedAt, err = a.store.LatestSyncOutboxSyncedAt(ctx); err != nil {
		return res, err
	}
	if st, err := a.store.GetSyncState(ctx); err == nil {
		if st.LastReconcileAt.Valid {
			res.LastReconcileAt = st.LastReconcileAt.String
		}
		if st.LastFullCheckAt.Valid {
			res.LastFullCheckAt = st.LastFullCheckAt.String
		}
	}

	res.FullySynced = res.Configured &&
		res.Pending == 0 && res.Failed == 0 && res.ConfirmedElsewhere == 0

	return res, nil
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
