package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	db "kashi-pos/db/sqlc"
)

// Quick Access can come from this till or from kashi (kashi's
// api/branch_quick_access.go). settings.quick_access_mode is this register's
// choice:
//
//	till   the till's own list (quick_access_items), edited here -- default
//	kashi  its branch's list from kashi (central_quick_access_items), shown
//	       read-only and kept current by the "quick_access" sync entity
//
// Either side can flip it, and the later flip wins: each side keeps the time
// its current value was chosen, kashi only accepts a flip newer than its own
// (SetBranchRegisterQuickAccessMode there), and a pull only adopts kashi's
// value when it's newer than ours (applySyncedQuickAccess). A flip made
// here travels through the outbox, so one made offline still arrives.
//
// None of this runs against a kashi that doesn't support it: the pull, the
// import and the switch all wait for "quick_access" in kashi's hello.

const (
	quickAccessModeTill  = "till"
	quickAccessModeKashi = "kashi"
	// quickAccessModeClientRef is the outbox row a flip rides in. One fixed
	// ref, so a second flip before the first uploads replaces it: only the
	// latest choice matters.
	quickAccessModeClientRef = "quick-access-mode"
)

var errQuickAccessManagedInKashi = fmt.Errorf("Quick Access on this register is managed in kashi")

type syncedQuickAccess struct {
	Mode          string    `json:"mode"`
	ModeChangedAt time.Time `json:"mode_changed_at"`
	Items         []struct {
		ProductUID string `json:"product_uid"`
		Position   int64  `json:"position"`
		Color      string `json:"color"`
		TextColor  string `json:"text_color"`
	} `json:"items"`
}

// applySyncedQuickAccess stores kashi's branch list (always, so it's ready
// the moment the mode becomes 'kashi') and adopts kashi's mode if kashi's
// choice is newer than ours -- a flip made here that hasn't uploaded yet is
// newer, so it survives.
func applySyncedQuickAccess(ctx context.Context, q *db.Queries, s syncedQuickAccess) error {
	settings, err := q.GetSettings(ctx)
	if err != nil {
		return err
	}
	if (s.Mode == quickAccessModeTill || s.Mode == quickAccessModeKashi) && s.ModeChangedAt.After(parseQuickAccessTime(settings.QuickAccessModeChangedAt)) {
		if err := q.SetQuickAccessMode(ctx, db.SetQuickAccessModeParams{
			QuickAccessMode:          s.Mode,
			QuickAccessModeChangedAt: s.ModeChangedAt.UTC().Format(time.RFC3339Nano),
		}); err != nil {
			return err
		}
	}
	if err := q.DeleteCentralQuickAccessItems(ctx); err != nil {
		return err
	}
	for _, item := range s.Items {
		if err := q.InsertCentralQuickAccessItem(ctx, db.InsertCentralQuickAccessItemParams{
			ProductUid: item.ProductUID, Position: item.Position, Color: item.Color, TextColor: item.TextColor,
		}); err != nil {
			return err
		}
	}
	return nil
}

func parseQuickAccessTime(v string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return time.Time{}
	}
	return t
}

// QuickAccessSource is what the Settings page and the Quick Access panel
// need to know about where the cards come from.
type QuickAccessSource struct {
	Mode string `json:"mode"`
	// KashiAvailable: connected to a kashi that manages Quick Access, so
	// switching to 'kashi' would actually show a list. The switch is still
	// allowed without it (the flip waits in the outbox), but the UI warns.
	KashiAvailable bool `json:"kashiAvailable"`
}

func (a *App) GetQuickAccessSource() (QuickAccessSource, error) {
	if a.store == nil {
		return QuickAccessSource{}, fmt.Errorf("database not initialized")
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return QuickAccessSource{}, err
	}
	return QuickAccessSource{Mode: settings.QuickAccessMode, KashiAvailable: a.kashiQuickAccess.Load()}, nil
}

// SetQuickAccessMode flips this register between its own list and kashi's.
// Admin-only, like editing the cards' colours.
func (a *App) SetQuickAccessMode(mode string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if !a.currentUserIsAdmin() {
		return fmt.Errorf("only an admin can change where Quick Access comes from")
	}
	if mode != quickAccessModeTill && mode != quickAccessModeKashi {
		return fmt.Errorf("invalid Quick Access mode %q", mode)
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return err
	}
	if settings.QuickAccessMode == mode {
		return nil
	}

	changedAt := time.Now().UTC()
	payload, err := json.Marshal(map[string]any{"mode": mode, "changed_at": changedAt})
	if err != nil {
		return err
	}
	if err := a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		if err := q.SetQuickAccessMode(a.ctx, db.SetQuickAccessModeParams{
			QuickAccessMode: mode, QuickAccessModeChangedAt: changedAt.Format(time.RFC3339Nano),
		}); err != nil {
			return err
		}
		if settings.CentralApiUrl == "" {
			return nil
		}
		return q.UpsertSyncOutboxEntry(a.ctx, db.UpsertSyncOutboxEntryParams{
			EntityType: "quick_access_mode", ClientRef: quickAccessModeClientRef, PayloadJson: string(payload),
		})
	}); err != nil {
		return err
	}
	a.kickSyncWorker()
	a.emitSyncApplied("quick_access")
	return nil
}

// afterQuickAccessModePushed re-pulls Quick Access once kashi has our flip:
// if kashi kept a newer flip instead, nothing changed there and nothing is
// pushed, so this pull is how the till learns it lost.
func (a *App) afterQuickAccessModePushed(client *centralClient) {
	go func() {
		if err := a.pullEntityOnce(a.ctx, client, "quick_access"); err != nil {
			log.Printf("quick access: pull after mode push: %v", err)
		}
	}()
}

// maybeReportQuickAccess offers this till's own list to kashi, once, so the
// branch's central list starts out as the cards the shop already uses. kashi
// only takes the first non-empty list per branch; an empty list here isn't
// sent, so a register with real cards can still be the one that seeds it.
func (a *App) maybeReportQuickAccess(ctx context.Context, client *centralClient) {
	if !a.kashiQuickAccess.Load() {
		return
	}
	settings, err := a.store.GetSettings(ctx)
	if err != nil || settings.QuickAccessReported != 0 {
		return
	}
	rows, err := a.store.ListQuickAccessForReport(ctx)
	if err != nil || len(rows) == 0 {
		return
	}
	type reportItem struct {
		ProductUID string `json:"product_uid"`
		Color      string `json:"color"`
		TextColor  string `json:"text_color"`
	}
	items := make([]reportItem, len(rows))
	for i, r := range rows {
		items[i] = reportItem{ProductUID: r.Uid.String, Color: r.Color, TextColor: r.TextColor}
	}
	var resp struct {
		Imported bool `json:"imported"`
	}
	if err := client.doJSON(ctx, http.MethodPost, "/branch/quick_access/import", map[string]any{"items": items}, &resp); err != nil {
		log.Printf("quick access: report list: %v", err)
		return
	}
	if err := a.store.SetQuickAccessReported(ctx); err != nil {
		log.Printf("quick access: mark reported: %v", err)
	}
}

// quickAccessManagedInKashi guards the till-side editing bindings: in
// 'kashi' mode the cards are kashi's, so add/remove/reorder/colour here
// would only change the hidden till list.
func (a *App) quickAccessManagedInKashi() bool {
	settings, err := a.store.GetSettings(a.ctx)
	return err == nil && settings.QuickAccessMode == quickAccessModeKashi
}
