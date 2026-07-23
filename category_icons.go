package main

import "fmt"

func (a *App) ListCategoryIcons() ([]map[string]any, error) {
	if a.queries == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	list, err := a.queries.ListCategoryIcons(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(list))
	for i, ci := range list {
		result[i] = map[string]any{
			"icon":       ci.Icon,
			"categories": ci.Categories,
		}
	}
	return result, nil
}
