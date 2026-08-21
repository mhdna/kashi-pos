package main

import (
	"database/sql"
	"fmt"
	"strings"

	db "kashi-pos/db/sqlc"
)

const (
	customItemDiscountsFlag = "custom_item_discounts"
	customItemPricesFlag    = "custom_item_prices"
	perUnitItemPricesFlag   = "per_unit_item_prices"
)

type ProductPage struct {
	Items []map[string]any `json:"items"`
	Total int64            `json:"total"`
}

// ProductCursorPage is a count-free, keyset-paginated slice of the catalog.
// It is used by infinite-scroll clients so later batches do not become slower
// as they would with increasingly large SQL OFFSET values.
type ProductCursorPage struct {
	Items      []map[string]any `json:"items"`
	NextCursor int64            `json:"nextCursor"`
	HasMore    bool             `json:"hasMore"`
}

func (a *App) GetProductByBarcode(barcode string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	product, err := a.store.GetProductByBarcode(a.ctx, barcode)
	if err != nil {
		return nil, fmt.Errorf("product not found")
	}

	return map[string]any{
		"id": product.ID, "barcode": product.Barcode, "code": product.Code,
		"name": product.Name, "category": product.Description,
		"price": float64(product.Price) / 100, "discount": product.Discount,
		"color": product.Color, "size": product.Size, "currency": product.CurrencyCode,
		"brand": product.Brand, "family": product.Family, "kind": product.Kind,
		"season": product.Season, "year": product.ProductYear,
	}, nil
}

func (a *App) GetProductByCode(code string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	product, err := a.store.GetProductByCode(a.ctx, code)
	if err != nil {
		return nil, fmt.Errorf("product not found")
	}

	return map[string]any{
		"id": product.ID, "barcode": product.Barcode, "code": product.Code,
		"name": product.Name, "category": product.Description,
		"price": float64(product.Price) / 100, "discount": product.Discount,
		"color": product.Color, "size": product.Size, "currency": product.CurrencyCode,
		"brand": product.Brand, "family": product.Family, "kind": product.Kind,
		"season": product.Season, "year": product.ProductYear,
	}, nil
}

func (a *App) ListProducts() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	products, err := a.store.ListProducts(a.ctx)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, len(products))
	for i, p := range products {
		result[i] = map[string]any{
			"id": p.ID, "barcode": p.Barcode, "code": p.Code,
			"name": p.Name, "category": p.Description,
			"price": float64(p.Price) / 100, "discount": p.Discount,
			"color": p.Color, "size": p.Size, "currency": p.CurrencyCode,
			"brand": p.Brand, "family": p.Family, "kind": p.Kind,
			"season": p.Season, "year": p.ProductYear,
		}
	}
	return result, nil
}

func productToMap(product db.Product) map[string]any {
	return map[string]any{
		"id": product.ID, "barcode": product.Barcode, "code": product.Code,
		"name": product.Name, "category": product.Description,
		"price": float64(product.Price) / 100, "discount": product.Discount,
		"color": product.Color, "size": product.Size, "currency": product.CurrencyCode,
		"brand": product.Brand, "family": product.Family, "kind": product.Kind,
		"season": product.Season, "year": product.ProductYear,
	}
}

func (a *App) ListQuickAccessProducts() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	products, err := a.store.ListQuickAccessProducts(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("list quick-access products: %w", err)
	}
	result := make([]map[string]any, len(products))
	for i, product := range products {
		result[i] = productToMap(product)
	}
	return result, nil
}

func (a *App) AddQuickAccessProduct(productID int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if productID <= 0 {
		return fmt.Errorf("invalid product")
	}
	if _, err := a.store.GetProductByID(a.ctx, productID); err != nil {
		return fmt.Errorf("product not found")
	}
	return a.store.AddQuickAccessProduct(a.ctx, productID)
}

func (a *App) RemoveQuickAccessProduct(productID int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if productID <= 0 {
		return fmt.Errorf("invalid product")
	}
	return a.store.RemoveQuickAccessProduct(a.ctx, productID)
}

func (a *App) ListProductsPage(page int, itemsPerPage int, search string) (ProductPage, error) {
	if a.store == nil {
		return ProductPage{}, fmt.Errorf("database not initialized")
	}
	if page < 1 {
		page = 1
	}
	if itemsPerPage < 1 {
		itemsPerPage = 5
	}
	if itemsPerPage > 100 {
		itemsPerPage = 100
	}

	search = strings.TrimSpace(search)
	params := db.ListProductsPageParams{
		Search: sql.NullString{String: search, Valid: true},
		Limit:  int64(itemsPerPage),
		Offset: int64(page-1) * int64(itemsPerPage),
	}

	var (
		products []db.Product
		total    int64
		err      error
	)
	if search == "" {
		products, err = a.store.ListProductsPageUnfiltered(a.ctx, db.ListProductsPageUnfilteredParams{
			Limit: params.Limit, Offset: params.Offset,
		})
		if err == nil {
			total, err = a.store.CountAllProducts(a.ctx)
		}
	} else {
		products, err = a.store.ListProductsPage(a.ctx, params)
		if err == nil {
			total, err = a.store.CountProducts(a.ctx, params.Search)
		}
	}
	if err != nil {
		return ProductPage{}, err
	}

	result := make([]map[string]any, len(products))
	for i, p := range products {
		result[i] = productMap(p)
	}
	return ProductPage{Items: result, Total: total}, nil
}

func (a *App) ListProductsCursor(afterID int64, limit int, search string) (ProductCursorPage, error) {
	if a.store == nil {
		return ProductCursorPage{}, fmt.Errorf("database not initialized")
	}
	if afterID < 0 {
		afterID = 0
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Fetch one extra row to determine whether another batch exists. This
	// avoids a full COUNT(*) query on every infinite-scroll request.
	queryLimit := int64(limit + 1)
	search = strings.TrimSpace(search)
	var (
		products []db.Product
		err      error
	)
	if search == "" {
		products, err = a.store.ListProductsAfterUnfiltered(a.ctx, db.ListProductsAfterUnfilteredParams{
			AfterID: afterID,
			Limit:   queryLimit,
		})
	} else {
		products, err = a.store.ListProductsAfter(a.ctx, db.ListProductsAfterParams{
			AfterID: afterID,
			Search:  sql.NullString{String: search, Valid: true},
			Limit:   queryLimit,
		})
	}
	if err != nil {
		return ProductCursorPage{}, err
	}

	hasMore := len(products) > limit
	if hasMore {
		products = products[:limit]
	}
	result := make([]map[string]any, len(products))
	for i, product := range products {
		result[i] = productMap(product)
	}
	nextCursor := afterID
	if len(products) > 0 {
		nextCursor = products[len(products)-1].ID
	}
	return ProductCursorPage{Items: result, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func productMap(p db.Product) map[string]any {
	return map[string]any{
		"id": p.ID, "barcode": p.Barcode, "code": p.Code,
		"name": p.Name, "category": p.Description,
		"price": float64(p.Price) / 100, "discount": p.Discount,
		"color": p.Color, "size": p.Size, "currency": p.CurrencyCode,
		"brand": p.Brand, "family": p.Family, "kind": p.Kind,
		"season": p.Season, "year": p.ProductYear,
	}
}

func (a *App) UpdateProductDiscount(code string, discount int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	enabled, err := a.store.GetFeatureFlag(a.ctx, customItemDiscountsFlag)
	if err != nil || enabled == 0 {
		return fmt.Errorf("custom item discounts are disabled")
	}
	if discount < 0 || discount > 100 {
		return fmt.Errorf("discount must be between 0 and 100")
	}
	product, err := a.store.GetProductByCode(a.ctx, strings.TrimSpace(code))
	if err != nil {
		return fmt.Errorf("product not found")
	}
	_, err = a.store.UpdateProduct(a.ctx, db.UpdateProductParams{
		Barcode: product.Barcode, Code: product.Code, Name: product.Name,
		Description: product.Description, IsActive: product.IsActive,
		Price: product.Price, Discount: discount, Color: product.Color, Size: product.Size,
		CurrencyCode: product.CurrencyCode, Brand: product.Brand, Family: product.Family,
		Kind: product.Kind, Season: product.Season, ProductYear: product.ProductYear, ID: product.ID,
	})
	return err
}
