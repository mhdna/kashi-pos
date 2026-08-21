package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	db "kashi-pos/db/sqlc"
)

func discountListToMap(dl db.DiscountList) map[string]any {
	return map[string]any{
		"id": dl.ID, "name": dl.Name, "isActive": dl.IsActive != 0, "createdAt": dl.CreatedAt,
	}
}

func discountListItemToMap(item db.DiscountListItem) map[string]any {
	return map[string]any{
		"id": item.ID, "discountListId": item.DiscountListID, "productCode": item.ProductCode,
		"discount": item.Discount, "createdAt": item.CreatedAt,
	}
}

func (a *App) ListDiscountLists() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	lists, err := a.store.ListDiscountLists(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(lists))
	for i, list := range lists {
		result[i] = discountListToMap(list)
	}
	return result, nil
}

func (a *App) CreateDiscountList(name string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	list, err := a.store.CreateDiscountList(a.ctx, name)
	if err != nil {
		return nil, err
	}
	return discountListToMap(list), nil
}

func (a *App) UpdateDiscountList(id int64, name string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	list, err := a.store.UpdateDiscountList(a.ctx, db.UpdateDiscountListParams{Name: name, ID: id})
	if err != nil {
		return nil, err
	}
	return discountListToMap(list), nil
}

// SetDiscountListActive mirrors SetPriceListActive's conflict check, scoped
// to discount lists only - a product can carry one active price override
// and one active discount override at the same time; only same-type
// overlaps are refused.
func (a *App) SetDiscountListActive(id int64, active bool) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if active {
		if err := checkDiscountListActivationConflict(a.ctx, a.store.Queries, id); err != nil {
			return nil, err
		}
	}
	isActive := int64(0)
	if active {
		isActive = 1
	}
	list, err := a.store.SetDiscountListActive(a.ctx, db.SetDiscountListActiveParams{IsActive: isActive, ID: id})
	if err != nil {
		return nil, err
	}
	return discountListToMap(list), nil
}

func (a *App) ListDiscountListItems(listID int64) ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	items, err := a.store.ListDiscountListItems(a.ctx, listID)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(items))
	for i, item := range items {
		result[i] = discountListItemToMap(item)
	}
	return result, nil
}

func (a *App) SetDiscountListItem(listID int64, productCode string, discount int64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	productCode = strings.TrimSpace(productCode)
	if productCode == "" {
		return nil, fmt.Errorf("product code is required")
	}
	if discount < 0 || discount > 100 {
		return nil, fmt.Errorf("discount must be between 0 and 100")
	}
	list, err := a.store.GetDiscountListByID(a.ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("discount list: %w", err)
	}
	if list.IsActive != 0 {
		if err := checkDiscountItemConflict(a.ctx, a.store.Queries, listID, productCode); err != nil {
			return nil, err
		}
	}
	item, err := a.store.UpsertDiscountListItem(a.ctx, db.UpsertDiscountListItemParams{
		DiscountListID: listID, ProductCode: productCode, Discount: discount,
	})
	if err != nil {
		return nil, err
	}
	return discountListItemToMap(item), nil
}

func (a *App) RemoveDiscountListItem(itemID int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.DeleteDiscountListItem(a.ctx, itemID)
}

func checkDiscountItemConflict(ctx context.Context, q *db.Queries, listID int64, code string) error {
	conflict, err := q.FindConflictingActiveDiscountListItem(ctx, db.FindConflictingActiveDiscountListItemParams{
		ID: listID, ProductCode: code,
	})
	if err == nil {
		return fmt.Errorf("%q is already overridden by the active discount list %q", code, conflict.ListName)
	}
	if err == sql.ErrNoRows {
		return nil
	}
	return err
}

func checkDiscountListActivationConflict(ctx context.Context, q *db.Queries, listID int64) error {
	items, err := q.ListDiscountListItems(ctx, listID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := checkDiscountItemConflict(ctx, q, listID, item.ProductCode); err != nil {
			return err
		}
	}
	return nil
}
