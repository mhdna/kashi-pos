package main

import (
	"fmt"
	"time"

	db "kashi-pos/db/sqlc"
)

func toLocalTime(s string) string {
	if s == "" {
		return ""
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z07:00",
		time.RFC3339,
	}
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, s, time.Local)
		if err == nil {
			return t.Format("02/01/2006 15:04")
		}
	}
	return s
}

func shiftToMap(s db.Shift) map[string]any {
	return map[string]any{
		"id":        s.ID,
		"isClosed":  s.IsClosed,
		"createdAt": toLocalTime(s.CreatedAt),
		"closedAt":  toLocalTime(s.ClosedAt),
	}
}

func (a *App) OpenShift() (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	if err := a.store.CloseOpenShifts(a.ctx, now); err != nil {
		return nil, fmt.Errorf("close previous shift: %w", err)
	}

	shift, err := a.store.CreateShift(a.ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("create shift: %w", err)
	}

	return shiftToMap(shift), nil
}

func (a *App) GetOpenShift() (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	shift, err := a.store.GetOpenShift(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("no open shift: %w", err)
	}

	return shiftToMap(shift), nil
}

func (a *App) CloseShiftByID(id int64, balances []map[string]any) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}

	return a.store.CloseShift(a.ctx, db.CloseShiftParams{
		ClosedAt: time.Now().Format("2006-01-02 15:04:05"),
		ID:       id,
	})
}
