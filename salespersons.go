package main

import "fmt"

func (a *App) ListSalespersons() ([]map[string]any, error) {
	if a.queries == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	list, err := a.queries.ListSalespersons(a.ctx)
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
