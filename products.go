package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	db "kashi-pos/db/sqlc"
)

const (
	customItemDiscountsFlag       = "custom_item_discounts"
	customItemPricesFlag          = "custom_item_prices"
	perUnitItemPricesFlag         = "per_unit_item_prices"
	priceChangeManualOverrideFlag = "price_change_manual_override_mode"
	invoiceKeyboardModeFlag       = "invoice_keyboard_mode"
	// Named as a "required" flag defaulting to true when unset, so a
	// database that has never touched this flag keeps requiring a client
	// selection on checkout - matching behavior before this setting existed.
	clientRequiredFlag = "client_required"
	// Named as a "hide" flag (rather than "show") so a database that has
	// never touched this flag - where GetFeatureFlag errors and the read
	// side treats it as false - keeps the currency line visible, matching
	// the calculator's behavior before this setting existed.
	hidePaymentMethodCurrencyFlag = "hide_payment_method_currency"
	autoAppendLbpZerosFlag        = "auto_append_lbp_zeros"
	// Named as a "confirm" flag defaulting to true when unset, so a database
	// that has never touched this flag keeps the checkout calculator's
	// overchange confirmation dialog on - matching its behavior before this
	// setting existed.
	overchangeConfirmationFlag = "overchange_confirmation"
	// Gates the product-search (magnifier) button on the sale screen.
	// Toggleable both from the local Settings page and remotely from kashi
	// (via an update_settings branch command). Defaults to true when unset,
	// so a database that has never touched this flag keeps the search button
	// available - matching behavior before this setting existed.
	productSearchEnabledFlag = "product_search_enabled"
	// When on, the salesperson selection is cleared for every new invoice so
	// the cashier must pick one each time. Defaults to false when unset, so a
	// database that has never touched this flag keeps carrying the previous
	// invoice's salesperson forward - matching behavior before this setting
	// existed.
	salespersonPerInvoiceRequiredFlag = "salesperson_per_invoice_required"
	// Gates the monthly income chart's prev/next month navigation for
	// non-admins. Defaults to false when unset, so a database that has never
	// touched this flag keeps that navigation admin-only - matching behavior
	// before this setting existed.
	nonAdminMonthNavEnabledFlag = "non_admin_month_nav_enabled"
)

// overchangeConfirmationEnabled reports whether the checkout calculator's
// "Confirm change amount" dialog is on, defaulting to true when the flag has
// never been set.
func (a *App) overchangeConfirmationEnabled() bool {
	if v, err := a.store.GetFeatureFlag(a.ctx, overchangeConfirmationFlag); err == nil {
		return v != 0
	}
	return true
}

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
	// Total is the full count matching the current search. It is only
	// computed for the first batch (afterID == 0) so infinite scroll stays
	// count-free; later batches leave it -1 and the client keeps the
	// value it already has.
	Total int64 `json:"total"`
}

func (a *App) GetProductByBarcode(barcode string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	product, err := a.store.GetProductByBarcode(a.ctx, barcode)
	if err != nil {
		return nil, fmt.Errorf("product not found")
	}
	overrides, err := loadActiveOverrides(a.ctx, a.store.Queries)
	if err != nil {
		return nil, err
	}
	attrs, err := loadOneProductAttributeValues(a.ctx, a.store.Queries, product.ID)
	if err != nil {
		return nil, err
	}
	return productMap(product, overrides, attrs), nil
}

func (a *App) GetProductByCode(code string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	product, err := a.store.GetProductByCode(a.ctx, code)
	if err != nil {
		return nil, fmt.Errorf("product not found")
	}
	overrides, err := loadActiveOverrides(a.ctx, a.store.Queries)
	if err != nil {
		return nil, err
	}
	attrs, err := loadOneProductAttributeValues(a.ctx, a.store.Queries, product.ID)
	if err != nil {
		return nil, err
	}
	return productMap(product, overrides, attrs), nil
}

func (a *App) ListQuickAccessProducts() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	products, err := a.store.ListQuickAccessProducts(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("list quick-access products: %w", err)
	}
	overrides, err := loadActiveOverrides(a.ctx, a.store.Queries)
	if err != nil {
		return nil, err
	}
	attrs, err := loadProductAttributeValues(a.ctx, a.store.Queries)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(products))
	for i, product := range products {
		result[i] = productMap(product, overrides, attrs[product.ID])
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

func (a *App) ReorderQuickAccessProducts(productIDs []int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	for i, id := range productIDs {
		if err := a.store.SetQuickAccessPosition(a.ctx, db.SetQuickAccessPositionParams{
			Position:  int64(i),
			ProductID: id,
		}); err != nil {
			return fmt.Errorf("reorder quick-access products: %w", err)
		}
	}
	return nil
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
		SearchPattern: "%" + search + "%",
		Limit:         int64(itemsPerPage),
		Offset:        int64(page-1) * int64(itemsPerPage),
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
			total, err = a.store.CountProducts(a.ctx, params.SearchPattern)
		}
	}
	if err != nil {
		log.Printf("ListProductsPage: search products (search=%q): %v", search, err)
		return ProductPage{}, fmt.Errorf("search products: %w", err)
	}
	overrides, err := loadActiveOverrides(a.ctx, a.store.Queries)
	if err != nil {
		log.Printf("ListProductsPage: load active overrides: %v", err)
		return ProductPage{}, fmt.Errorf("load price/discount overrides: %w", err)
	}
	attrs, err := loadProductAttributeValues(a.ctx, a.store.Queries)
	if err != nil {
		log.Printf("ListProductsPage: load product attributes: %v", err)
		return ProductPage{}, fmt.Errorf("load product attributes: %w", err)
	}

	result := make([]map[string]any, len(products))
	for i, p := range products {
		result[i] = productMap(p, overrides, attrs[p.ID])
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
			AfterID:       afterID,
			SearchPattern: "%" + search + "%",
			Limit:         queryLimit,
		})
	}
	if err != nil {
		log.Printf("ListProductsCursor: search products (search=%q): %v", search, err)
		return ProductCursorPage{}, fmt.Errorf("search products: %w", err)
	}
	overrides, err := loadActiveOverrides(a.ctx, a.store.Queries)
	if err != nil {
		log.Printf("ListProductsCursor: load active overrides: %v", err)
		return ProductCursorPage{}, fmt.Errorf("load price/discount overrides: %w", err)
	}
	attrs, err := loadProductAttributeValues(a.ctx, a.store.Queries)
	if err != nil {
		log.Printf("ListProductsCursor: load product attributes: %v", err)
		return ProductCursorPage{}, fmt.Errorf("load product attributes: %w", err)
	}

	hasMore := len(products) > limit
	if hasMore {
		products = products[:limit]
	}
	result := make([]map[string]any, len(products))
	for i, product := range products {
		result[i] = productMap(product, overrides, attrs[product.ID])
	}
	nextCursor := afterID
	if len(products) > 0 {
		nextCursor = products[len(products)-1].ID
	}

	total := int64(-1)
	if afterID == 0 {
		if search == "" {
			total, err = a.store.CountAllProducts(a.ctx)
		} else {
			total, err = a.store.CountProducts(a.ctx, "%"+search+"%")
		}
		if err != nil {
			log.Printf("ListProductsCursor: count products (search=%q): %v", search, err)
			return ProductCursorPage{}, fmt.Errorf("count products: %w", err)
		}
	}
	return ProductCursorPage{Items: result, NextCursor: nextCursor, HasMore: hasMore, Total: total}, nil
}

// loadProductAttributeValues loads every product's attribute values in one
// query (same batching pattern as loadActiveOverrides below: one query per
// request regardless of how many products are in the result set), keyed
// by product id then attribute name — e.g. attrs[42]["brand"] == "Nike".
//
// This is the source of truth for brand/category/etc. going forward, not
// products.brand/family/kind/season/product_year — those columns still
// exist and are still written by admin_server.go and tools/kashi-admin
// (which are being retired, not converted), but the main app no longer
// reads them.
func loadProductAttributeValues(ctx context.Context, q *db.Queries) (map[int64]map[string]string, error) {
	rows, err := q.ListAllProductAttributeValues(ctx)
	if err != nil {
		return nil, err
	}
	attrs := make(map[int64]map[string]string)
	for _, row := range rows {
		if attrs[row.ProductID] == nil {
			attrs[row.ProductID] = make(map[string]string)
		}
		attrs[row.ProductID][row.AttributeName] = row.Value
	}
	return attrs, nil
}

func loadOneProductAttributeValues(ctx context.Context, q *db.Queries, productID int64) (map[string]string, error) {
	rows, err := q.ListProductAttributeValues(ctx, productID)
	if err != nil {
		return nil, err
	}
	attrs := make(map[string]string, len(rows))
	for _, row := range rows {
		attrs[row.AttributeName] = row.Value
	}
	return attrs, nil
}

// productMap converts a product row to the shape the frontend consumes,
// overlaying an active price-list/discount-list override (if any) onto the
// price/discount so every product-lookup path (search, quick-access, cart
// add, checkout) agrees on what a product currently sells for. attrs is
// this one product's entry from loadProductAttributeValues (nil is fine —
// a product with no attribute rows yet just reports empty strings).
func productMap(p db.Product, overrides activeOverrides, attrs map[string]string) map[string]any {
	price := p.Price
	discount := p.Discount
	priceOverridden, discountOverridden := false, false
	priceListName, discountListName := "", ""
	if override, ok := overrides.prices[p.Code]; ok {
		price = override.cents
		priceOverridden = true
		priceListName = override.listName
	}
	if override, ok := overrides.discounts[p.Code]; ok {
		discount = override.percent
		discountOverridden = true
		discountListName = override.listName
	}
	return map[string]any{
		"id": p.ID, "barcode": p.Barcode, "code": p.Code,
		"name": p.Name, "category": p.Description,
		"price": float64(price) / 100, "discount": discount,
		"color": p.Color, "size": p.Size, "currency": p.CurrencyCode,
		// "family" kept as the frontend-facing key for backward
		// compatibility, sourced from the 'sub-category' attribute — see
		// the migration comment in 000073_create_product_attributes.up.sql
		// for why that's the mapping.
		"brand": attrs["brand"], "family": attrs["sub-category"], "kind": attrs["kind"],
		"season": attrs["season"], "year": attrs["year"],
		"priceListOverridden": priceOverridden, "priceListName": priceListName,
		"discountListOverridden": discountOverridden, "discountListName": discountListName,
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
