package main

import "fmt"

// ListExpenseCategories / ListLoanCategories back the category dropdowns on
// the Expenses and Loans pages. The rows are synced down from kashi (see
// applySyncedExpenseCategory in sync_pull.go); the picker still submits the
// category *name*, which kashi's /branch endpoints resolve on their side.
func (a *App) ListExpenseCategories() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	list, err := a.store.ListActiveExpenseCategories(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(list))
	for i, c := range list {
		result[i] = map[string]any{
			"id":       c.ID,
			"name":     c.Name,
			"isActive": c.IsActive == 1,
			"icon":     c.Icon,
			"color":    c.Color,
		}
	}
	return result, nil
}

func (a *App) ListLoanCategories() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	list, err := a.store.ListActiveLoanCategories(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(list))
	for i, c := range list {
		result[i] = map[string]any{
			"id":       c.ID,
			"name":     c.Name,
			"isActive": c.IsActive == 1,
			"icon":     c.Icon,
			"color":    c.Color,
		}
	}
	return result, nil
}
