package main

import (
	"fmt"
	"strings"

	db "kashi-pos/db/sqlc"
)

func (a *App) ListSalespersons() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	list, err := a.store.ListSalespersons(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(list))
	for i, s := range list {
		result[i] = map[string]any{
			"id":               s.ID,
			"fullName":         s.FullName,
			"attendanceUserId": s.AttendanceUserID,
			"isActive":         s.IsActive != 0,
			"isSynced":         s.RemoteID.Valid,
			"color":            s.Color,
			// Off for staff who only punch attendance: the selling screens
			// leave them out, the attendance page doesn't.
			"isSalesperson": s.IsSalesperson != 0,
		}
	}
	return result, nil
}

func (a *App) UpdateSalespersonAttendanceID(id int64, attendanceUserID string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.UpdateSalespersonAttendanceID(a.ctx, db.UpdateSalespersonAttendanceIDParams{
		AttendanceUserID: strings.TrimSpace(attendanceUserID),
		ID:               id,
	})
}

// Creating, renaming and deleting salespersons is local-only in standalone
// mode. Once the till is connected, kashi owns the roster (see
// applySyncedSalesperson); a till-only salesperson left over from before can
// still be deleted, but not renamed.
func (a *App) CreateSalesperson(fullName string) (map[string]any, error) {
	if err := a.requireStandalone(); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(fullName)
	if name == "" {
		return nil, fmt.Errorf("salesperson name is required")
	}
	s, err := a.store.CreateSalesperson(a.ctx, name)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": s.ID, "fullName": s.FullName}, nil
}

func (a *App) UpdateSalesperson(id int64, fullName string) (map[string]any, error) {
	if err := a.requireStandalone(); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(fullName)
	if name == "" {
		return nil, fmt.Errorf("salesperson name is required")
	}
	s, err := a.store.UpdateSalesperson(a.ctx, db.UpdateSalespersonParams{ID: id, FullName: name})
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": s.ID, "fullName": s.FullName}, nil
}

func (a *App) DeleteSalesperson(id int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	s, err := a.store.GetSalespersonByID(a.ctx, id)
	if err != nil {
		return err
	}
	if s.RemoteID.Valid {
		return errManagedByKashi
	}
	return a.store.DeleteSalesperson(a.ctx, id)
}
