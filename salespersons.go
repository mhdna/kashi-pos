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
			"id":       s.ID,
			"fullName": s.FullName,
		}
	}
	return result, nil
}

func (a *App) CreateSalesperson(fullName string) (map[string]any, error) {
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
	return a.store.DeleteSalesperson(a.ctx, id)
}
