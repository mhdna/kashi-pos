package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
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
	// Lets the sale screen's scan field fall back to the product code when
	// nothing has that barcode. Defaults to true when unset, so a database
	// that has never touched this flag keeps accepting codes there - matching
	// behavior before this setting existed.
	scanByCodeFlag = "scan_by_code"
	// When on, the product search dialog hides product names and matches only
	// the code or barcode. Defaults to false when unset, so a database that
	// has never touched this flag keeps the name search - matching behavior
	// before this setting existed.
	productSearchCodeOnlyFlag = "product_search_code_only"
	// When on, a line's price and discount can be edited by hand only for the
	// codes in settings.price_edit_codes (on top of the custom price/discount
	// flags). Defaults to false when unset, so every line stays editable as it
	// was before this setting existed.
	priceEditRestrictedFlag = "price_edit_restricted"
)

// SetProductSearchCodeOnly toggles code-only product search. Called both by
// the local Settings page and by an update_settings command from kashi.
func (a *App) SetProductSearchCodeOnly(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := a.setFeatureFlagBool(a.ctx, productSearchCodeOnlyFlag, enabled); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

// SetPriceEditRestricted toggles the per-code limit on manual price and
// discount edits.
func (a *App) SetPriceEditRestricted(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := a.setFeatureFlagBool(a.ctx, priceEditRestrictedFlag, enabled); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

// normalizePriceEditCodes splits a comma/space/newline separated list into
// distinct trimmed codes joined by commas, so the stored form is the same
// however kashi or the Settings page typed it. Codes compare case-insensitively
// on the sale screen, so case is kept as typed.
func normalizePriceEditCodes(raw string) string {
	seen := map[string]bool{}
	var codes []string
	for _, code := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == ' ' || r == '\t'
	}) {
		key := strings.ToLower(code)
		if seen[key] {
			continue
		}
		seen[key] = true
		codes = append(codes, code)
	}
	return strings.Join(codes, ",")
}

// SetPriceEditCodes stores the codes whose price/discount may be edited by
// hand while price_edit_restricted is on.
func (a *App) SetPriceEditCodes(raw string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := a.store.SetPriceEditCodes(a.ctx, normalizePriceEditCodes(raw)); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

func (a *App) priceEditCodes() string {
	if s, err := a.store.GetSettings(a.ctx); err == nil {
		return s.PriceEditCodes
	}
	return ""
}

// overchangeConfirmationEnabled reports whether the checkout calculator's
// "Confirm change amount" dialog is on, defaulting to true when the flag has
// never been set.
func (a *App) overchangeConfirmationEnabled() bool {
	if v, err := a.store.GetFeatureFlag(a.ctx, overchangeConfirmationFlag); err == nil {
		return v != 0
	}
	return true
}

// scanByCodeEnabled reports whether the scan field accepts product codes,
// defaulting to true when the flag has never been set.
func (a *App) scanByCodeEnabled() bool {
	if v, err := a.store.GetFeatureFlag(a.ctx, scanByCodeFlag); err == nil {
		return v != 0
	}
	return true
}

// SetScanByCodeEnabled toggles the scan field's product-code fallback.
// Called both by the local Settings page and by an update_settings command
// from kashi; emits "settings_changed" like the other sale-screen toggles.
func (a *App) SetScanByCodeEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := a.setFeatureFlagBool(a.ctx, scanByCodeFlag, enabled); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
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
	listPricing, err := loadActivePricing(a.ctx, a.store.Queries)
	if err != nil {
		return nil, err
	}
	attrs, err := loadOneProductAttributeValues(a.ctx, a.store.Queries, product.ID)
	if err != nil {
		return nil, err
	}
	return productMap(product, listPricing, attrs), nil
}

// FindProductsForScan resolves what the cashier typed or scanned into the
// sale screen's scan field. A barcode names exactly one product and always
// wins. Failing that, and only while scanByCodeFlag is on, the value is
// tried as a product code -- which can return several active variants
// (colour/size share a code); the screen then asks which one. An empty
// result means nothing matched.
func (a *App) FindProductsForScan(value string) ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return []map[string]any{}, nil
	}

	var products []db.Product
	if p, err := a.store.GetProductByBarcode(a.ctx, value); err == nil {
		products = []db.Product{p}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	} else if a.scanByCodeEnabled() {
		products, err = a.store.ListActiveProductsByCode(a.ctx, value)
		if err != nil {
			return nil, err
		}
	}
	if len(products) == 0 {
		return []map[string]any{}, nil
	}

	listPricing, err := loadActivePricing(a.ctx, a.store.Queries)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(products))
	for _, p := range products {
		attrs, err := loadOneProductAttributeValues(a.ctx, a.store.Queries, p.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, productMap(p, listPricing, attrs))
	}
	return result, nil
}

func (a *App) GetProductByCode(code string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	product, err := a.store.GetProductByCode(a.ctx, code)
	if err != nil {
		return nil, fmt.Errorf("product not found")
	}
	listPricing, err := loadActivePricing(a.ctx, a.store.Queries)
	if err != nil {
		return nil, err
	}
	attrs, err := loadOneProductAttributeValues(a.ctx, a.store.Queries, product.ID)
	if err != nil {
		return nil, err
	}
	return productMap(product, listPricing, attrs), nil
}

func (a *App) ListQuickAccessProducts() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if a.quickAccessManagedInKashi() {
		return a.listCentralQuickAccessProducts()
	}
	products, err := a.store.ListQuickAccessProducts(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("list quick-access products: %w", err)
	}
	listPricing, err := loadActivePricing(a.ctx, a.store.Queries)
	if err != nil {
		return nil, err
	}
	attrs, err := loadProductAttributeValues(a.ctx, a.store.Queries)
	if err != nil {
		return nil, err
	}
	colorRows, err := a.store.ListQuickAccessColors(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("list quick-access colors: %w", err)
	}
	colors := make(map[int64]db.ListQuickAccessColorsRow, len(colorRows))
	for _, row := range colorRows {
		colors[row.ProductID] = row
	}
	result := make([]map[string]any, len(products))
	for i, product := range products {
		result[i] = productMap(product, listPricing, attrs[product.ID])
		result[i]["quickAccessColor"] = colors[product.ID].Color
		result[i]["quickAccessTextColor"] = colors[product.ID].TextColor
	}
	return result, nil
}

// listCentralQuickAccessProducts is ListQuickAccessProducts in 'kashi'
// mode: kashi's branch list, same card shape.
func (a *App) listCentralQuickAccessProducts() ([]map[string]any, error) {
	rows, err := a.store.ListCentralQuickAccessProducts(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("list central quick-access products: %w", err)
	}
	listPricing, err := loadActivePricing(a.ctx, a.store.Queries)
	if err != nil {
		return nil, err
	}
	attrs, err := loadProductAttributeValues(a.ctx, a.store.Queries)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(rows))
	for i, row := range rows {
		result[i] = productMap(row.Product, listPricing, attrs[row.Product.ID])
		result[i]["quickAccessColor"] = row.QaColor
		result[i]["quickAccessTextColor"] = row.QaTextColor
	}
	return result, nil
}

var quickAccessColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// SetQuickAccessColor sets the background and text colours of one
// quick-access card ("" = default background / automatic text colour). Only
// reachable from Settings, so it's admin-only.
func (a *App) SetQuickAccessColor(productID int64, color string, textColor string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if a.quickAccessManagedInKashi() {
		return errQuickAccessManagedInKashi
	}
	if !a.currentUserIsAdmin() {
		return fmt.Errorf("only an admin can change quick-access colors")
	}
	if productID <= 0 {
		return fmt.Errorf("invalid product")
	}
	for _, c := range []string{color, textColor} {
		if c != "" && !quickAccessColorPattern.MatchString(c) {
			return fmt.Errorf("invalid color %q", c)
		}
	}
	return a.store.SetQuickAccessColor(a.ctx, db.SetQuickAccessColorParams{
		Color:     strings.ToLower(color),
		TextColor: strings.ToLower(textColor),
		ProductID: productID,
	})
}

func (a *App) AddQuickAccessProduct(productID int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if a.quickAccessManagedInKashi() {
		return errQuickAccessManagedInKashi
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
	if a.quickAccessManagedInKashi() {
		return errQuickAccessManagedInKashi
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
	if a.quickAccessManagedInKashi() {
		return errQuickAccessManagedInKashi
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
	listPricing, err := loadActivePricing(a.ctx, a.store.Queries)
	if err != nil {
		log.Printf("ListProductsPage: load active pricing: %v", err)
		return ProductPage{}, fmt.Errorf("load price/discount lists: %w", err)
	}
	attrs, err := loadProductAttributeValues(a.ctx, a.store.Queries)
	if err != nil {
		log.Printf("ListProductsPage: load product attributes: %v", err)
		return ProductPage{}, fmt.Errorf("load product attributes: %w", err)
	}

	result := make([]map[string]any, len(products))
	for i, p := range products {
		result[i] = productMap(p, listPricing, attrs[p.ID])
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
	codeOnly := a.flagEnabled(a.ctx, productSearchCodeOnlyFlag)
	if search == "" {
		products, err = a.store.ListProductsAfterUnfiltered(a.ctx, db.ListProductsAfterUnfilteredParams{
			AfterID: afterID,
			Limit:   queryLimit,
		})
	} else if codeOnly {
		products, err = a.store.ListProductsAfterByCode(a.ctx, db.ListProductsAfterByCodeParams{
			AfterID:       afterID,
			SearchPattern: "%" + search + "%",
			Limit:         queryLimit,
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
	listPricing, err := loadActivePricing(a.ctx, a.store.Queries)
	if err != nil {
		log.Printf("ListProductsCursor: load active pricing: %v", err)
		return ProductCursorPage{}, fmt.Errorf("load price/discount lists: %w", err)
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
		result[i] = productMap(product, listPricing, attrs[product.ID])
	}
	nextCursor := afterID
	if len(products) > 0 {
		nextCursor = products[len(products)-1].ID
	}

	total := int64(-1)
	if afterID == 0 {
		if search == "" {
			total, err = a.store.CountAllProducts(a.ctx)
		} else if codeOnly {
			total, err = a.store.CountProductsByCode(a.ctx, "%"+search+"%")
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
// pricing it from the active lists so every product-lookup path (search,
// quick-access, cart add, checkout) agrees on what a product currently sells
// for. attrs is this one product's entry from loadProductAttributeValues (nil
// is fine — a product with no attribute rows yet just reports empty strings).
//
// A product in no active list reports price 0 and priced:false. The frontend
// uses that to mark it unsellable up front, rather than letting a cashier
// scan it and only discover at checkout that the quote won't total.
func productMap(p db.Product, listPricing activePricing, attrs map[string]string) map[string]any {
	var price, discount int64
	priced := false
	priceListName, discountListName := "", ""
	priceSpecial, discountSpecial := false, false
	if listed, ok := listPricing.priceFor(p.Uid.String); ok {
		price = listed.cents
		priced = true
		priceListName = listed.listName
		priceSpecial = listed.isSpecial
	}
	if listed, ok := listPricing.discountFor(p.Uid.String); ok {
		discount = listed.percent
		discountListName = listed.listName
		discountSpecial = listed.isSpecial
	}
	return map[string]any{
		"id": p.ID, "barcode": p.Barcode, "code": p.Code,
		"name": p.Name, "category": p.Description,
		"price": float64(price) / 100, "discount": discount,
		"priced": priced,
		"color":  p.Color, "size": p.Size, "currency": p.CurrencyCode,
		// "family" kept as the frontend-facing key for backward
		// compatibility, sourced from the 'sub-category' attribute — see
		// the migration comment in 000073_create_product_attributes.up.sql
		// for why that's the mapping.
		"brand": attrs["brand"], "family": attrs["sub-category"], "kind": attrs["kind"],
		"season": attrs["season"], "year": attrs["year"],
		"priceListOverridden": priceSpecial, "priceListName": priceListName,
		"discountListOverridden": discountSpecial, "discountListName": discountListName,
		"saleOptions": saleOptionsMap(p, listPricing),
	}
}

// saleOptionsMap lists what the till must ask when this product is sold
// (Summer/Winter...), each with the discount the line gets as that option,
// so the cashier sees the difference on the choice itself. Empty for an
// ordinary product, which is added to the cart without asking.
func saleOptionsMap(p db.Product, listPricing activePricing) []map[string]any {
	names := listPricing.saleOptions[p.Uid.String]
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		var discount int64
		listName := ""
		if listed, ok := listPricing.discountForOption(p.Uid.String, name); ok {
			discount = listed.percent
			listName = listed.listName
		}
		out = append(out, map[string]any{"name": name, "discount": discount, "discountListName": listName})
	}
	return out
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

// RefreshCartPricing returns the current list price and discount of each
// barcode in the cart, so lines already on the sale screen can follow a price
// or discount list that kashi has just changed. Barcodes that no longer
// resolve are left out; the screen keeps those lines as they are.
func (a *App) RefreshCartPricing(barcodesJSON string) ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var barcodes []string
	if err := json.Unmarshal([]byte(barcodesJSON), &barcodes); err != nil {
		return nil, fmt.Errorf("invalid barcodes")
	}
	listPricing, err := loadActivePricing(a.ctx, a.store.Queries)
	if err != nil {
		return nil, fmt.Errorf("load price/discount lists: %w", err)
	}
	out := make([]map[string]any, 0, len(barcodes))
	for _, barcode := range barcodes {
		product, err := a.store.GetProductByBarcode(a.ctx, strings.TrimSpace(barcode))
		if err != nil || product.IsActive == 0 {
			continue
		}
		out = append(out, productMap(product, listPricing, nil))
	}
	return out, nil
}

// productSearchFields are the columns of the product search dialog a branch
// may hide.
var productSearchFields = map[string]bool{"code": true, "name": true, "brand": true, "color": true, "size": true, "price": true}

// normalizeProductSearchHiddenFields keeps the known field names from a
// comma/space separated list, once each, in the order they were given.
func normalizeProductSearchHiddenFields(raw string) string {
	seen := map[string]bool{}
	var fields []string
	for _, field := range strings.FieldsFunc(strings.ToLower(raw), func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
	}) {
		if productSearchFields[field] && !seen[field] {
			seen[field] = true
			fields = append(fields, field)
		}
	}
	return strings.Join(fields, ",")
}

// SetProductSearchHiddenFields chooses which columns the product search dialog
// leaves out. Called by an update_settings command from kashi.
func (a *App) SetProductSearchHiddenFields(raw string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := a.store.SetProductSearchHiddenFields(a.ctx, normalizeProductSearchHiddenFields(raw)); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

func (a *App) productSearchHiddenFields() string {
	if s, err := a.store.GetSettings(a.ctx); err == nil {
		return s.ProductSearchHiddenFields
	}
	return ""
}
