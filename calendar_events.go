package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	db "kashi-pos/db/sqlc"
)

// Calendar events are a read-only mirror of central's user-defined events (the
// "calendar_events" sync entity, api/calendar_event.go in the central repo).
// Central sends the whole set on every pull, so the apply upserts what it is
// given and deletes the rest. The built-in retail calendar (Valentine's,
// Black Friday...) is not synced: the frontend carries its own copy of the
// event lists, and the "calendar_settings" entity says which business type's
// list to show and which events central has switched off.

type syncedCalendarEvent struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	Color     string `json:"color"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

const calendarDateLayout = "2006-01-02"

func applySyncedCalendarEvents(ctx context.Context, q *db.Queries, events []syncedCalendarEvent) error {
	ids := make([]int64, 0, len(events))
	for _, e := range events {
		start, err := time.Parse(calendarDateLayout, e.StartDate)
		if err != nil {
			return fmt.Errorf("event %d: bad start_date %q", e.ID, e.StartDate)
		}
		end, err := time.Parse(calendarDateLayout, e.EndDate)
		if err != nil || end.Before(start) {
			return fmt.Errorf("event %d: bad end_date %q", e.ID, e.EndDate)
		}
		if err := q.UpsertCalendarEvent(ctx, db.UpsertCalendarEventParams{
			ID: e.ID, Name: e.Name, Icon: e.Icon, Color: e.Color,
			StartDate: e.StartDate, EndDate: e.EndDate,
		}); err != nil {
			return err
		}
		ids = append(ids, e.ID)
	}
	if len(ids) == 0 {
		ids = []int64{-1} // an empty IN () is invalid SQL; no real id is negative
	}
	return q.DeleteCalendarEventsNotIn(ctx, ids)
}

// ListCalendarEvents returns the synced events for the Events page.
func (a *App) ListCalendarEvents() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := a.store.ListCalendarEvents(a.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, len(rows))
	for i, r := range rows {
		out[i] = map[string]any{
			"id": r.ID, "name": r.Name, "icon": r.Icon, "color": r.Color,
			"startDate": r.StartDate, "endDate": r.EndDate,
		}
	}
	return out, nil
}

// syncedCalendarSettings is the one item of the "calendar_settings" entity.
type syncedCalendarSettings struct {
	BusinessType string          `json:"business_type"`
	HiddenEvents []string        `json:"hidden_builtin_events"`
	Catalog      json.RawMessage `json:"catalog"`
}

func applySyncedCalendarSettings(ctx context.Context, q *db.Queries, items []syncedCalendarSettings) error {
	if len(items) == 0 {
		return nil
	}
	s := items[0]
	if s.BusinessType == "" {
		s.BusinessType = "clothing"
	}
	if s.HiddenEvents == nil {
		s.HiddenEvents = []string{}
	}
	hidden, err := json.Marshal(s.HiddenEvents)
	if err != nil {
		return err
	}
	// The built-in events as date rules. Kept only when it is valid JSON; an
	// older central sends none, and the page then falls back to its bundled copy.
	catalog := ""
	if len(s.Catalog) > 0 && json.Valid(s.Catalog) {
		catalog = string(s.Catalog)
	}
	return q.SetCalendarSettings(ctx, db.SetCalendarSettingsParams{
		BusinessType: s.BusinessType, HiddenEvents: string(hidden), Catalog: catalog,
	})
}

// GetCalendarSettings tells the Events page which built-in calendar to show.
func (a *App) GetCalendarSettings() (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	row, err := a.store.GetCalendarSettings(a.ctx)
	if err != nil {
		return nil, err
	}
	hidden := []string{}
	_ = json.Unmarshal([]byte(row.HiddenEvents), &hidden) // bad JSON: show everything
	return map[string]any{"businessType": row.BusinessType, "hiddenEvents": hidden, "catalog": row.Catalog}, nil
}
