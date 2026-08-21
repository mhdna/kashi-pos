package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	db "kashi-pos/db/sqlc"
)

func priceListToMap(pl db.PriceList) map[string]any {
	return map[string]any{
		"id": pl.ID, "name": pl.Name, "isActive": pl.IsActive != 0, "createdAt": pl.CreatedAt,
	}
}

func priceListItemToMap(item db.PriceListItem) map[string]any {
	return map[string]any{
		"id": item.ID, "priceListId": item.PriceListID, "productCode": item.ProductCode,
		"price": float64(item.Price) / 100, "createdAt": item.CreatedAt,
	}
}

func (a *App) ListPriceLists() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	lists, err := a.store.ListPriceLists(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(lists))
	for i, list := range lists {
		result[i] = priceListToMap(list)
	}
	return result, nil
}

func (a *App) CreatePriceList(name string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	list, err := a.store.CreatePriceList(a.ctx, name)
	if err != nil {
		return nil, err
	}
	return priceListToMap(list), nil
}

func (a *App) UpdatePriceList(id int64, name string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	list, err := a.store.UpdatePriceList(a.ctx, db.UpdatePriceListParams{Name: name, ID: id})
	if err != nil {
		return nil, err
	}
	return priceListToMap(list), nil
}

// SetPriceListActive turns a price list on or off. Turning one on checks
// every one of its product codes against every other currently-active price
// list; if any code is already overridden elsewhere, activation is refused
// with an error naming the product and the conflicting list, rather than
// silently letting two lists fight over the same product's price.
func (a *App) SetPriceListActive(id int64, active bool) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if active {
		if err := checkPriceListActivationConflict(a.ctx, a.store.Queries, id); err != nil {
			return nil, err
		}
	}
	isActive := int64(0)
	if active {
		isActive = 1
	}
	list, err := a.store.SetPriceListActive(a.ctx, db.SetPriceListActiveParams{IsActive: isActive, ID: id})
	if err != nil {
		return nil, err
	}
	return priceListToMap(list), nil
}

func (a *App) ListPriceListItems(listID int64) ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	items, err := a.store.ListPriceListItems(a.ctx, listID)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(items))
	for i, item := range items {
		result[i] = priceListItemToMap(item)
	}
	return result, nil
}

// SetPriceListItem adds or updates the override price for a product code
// within a list. If the list is already active, this runs the same conflict
// check as activation (adding a code to a live list is just as capable of
// colliding with another active list as flipping the whole list on is).
func (a *App) SetPriceListItem(listID int64, productCode string, price float64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	productCode = strings.TrimSpace(productCode)
	if productCode == "" {
		return nil, fmt.Errorf("product code is required")
	}
	if price <= 0 {
		return nil, fmt.Errorf("price must be positive")
	}
	list, err := a.store.GetPriceListByID(a.ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("price list: %w", err)
	}
	if list.IsActive != 0 {
		if err := checkPriceItemConflict(a.ctx, a.store.Queries, listID, productCode); err != nil {
			return nil, err
		}
	}
	item, err := a.store.UpsertPriceListItem(a.ctx, db.UpsertPriceListItemParams{
		PriceListID: listID, ProductCode: productCode, Price: int64(price*100 + 0.5),
	})
	if err != nil {
		return nil, err
	}
	return priceListItemToMap(item), nil
}

func (a *App) RemovePriceListItem(itemID int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.DeletePriceListItem(a.ctx, itemID)
}

func checkPriceItemConflict(ctx context.Context, q *db.Queries, listID int64, code string) error {
	conflict, err := q.FindConflictingActivePriceListItem(ctx, db.FindConflictingActivePriceListItemParams{
		ID: listID, ProductCode: code,
	})
	if err == nil {
		return fmt.Errorf("%q is already overridden by the active price list %q", code, conflict.ListName)
	}
	if err == sql.ErrNoRows {
		return nil
	}
	return err
}

func checkPriceListActivationConflict(ctx context.Context, q *db.Queries, listID int64) error {
	items, err := q.ListPriceListItems(ctx, listID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := checkPriceItemConflict(ctx, q, listID, item.ProductCode); err != nil {
			return err
		}
	}
	return nil
}
