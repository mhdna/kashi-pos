package main

import (
	"fmt"

	db "kashi-pos/db/sqlc"
)

// Read-only, exactly like price_lists.go -- see the note there.

func discountListToMap(dl db.DiscountList) map[string]any {
	return map[string]any{
		"id": dl.ID, "uid": dl.Uid.String, "name": dl.Name,
		"isActive": dl.IsActive != 0, "isDefault": dl.IsDefault != 0,
		"createdAt": dl.CreatedAt,
	}
}

func discountListItemToMap(item db.DiscountListItem) map[string]any {
	return map[string]any{
		"id": item.ID, "discountListId": item.DiscountListID, "productUid": item.ProductUid,
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
