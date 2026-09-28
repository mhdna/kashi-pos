package main

import (
	"fmt"

	db "kashi-pos/db/sqlc"
)

// Price lists are read-only here. kashi owns them: it decides what exists,
// what a product costs and which list this branch prices from, and the sync
// mirrors that down (applySyncedPriceList). The create/update/activate and
// per-item editing methods this file used to expose are gone, along with the
// activation conflict check that policed two local lists fighting over one
// product -- central is the only writer now, so there is nothing to police.

func priceListToMap(pl db.PriceList) map[string]any {
	return map[string]any{
		"id": pl.ID, "uid": pl.Uid.String, "name": pl.Name,
		"isActive": pl.IsActive != 0, "isDefault": pl.IsDefault != 0,
		"createdAt": pl.CreatedAt,
	}
}

func priceListItemToMap(item db.PriceListItem) map[string]any {
	return map[string]any{
		"id": item.ID, "priceListId": item.PriceListID, "productUid": item.ProductUid,
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
