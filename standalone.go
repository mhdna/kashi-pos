package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	db "kashi-pos/db/sqlc"
)

// Standalone mode. A till with no central server configured owns its own
// products and clients: it creates, edits and removes them itself. Once a
// central server is configured, kashi is the only authority for both -- the
// till copies kashi's catalog and roster (see sync_pull.go and
// sync_full_check.go) and these editing paths are refused. Every product and
// client carries a uid either way (000095_sync_uid), so a standalone till
// that later connects to kashi adopts kashi's identities without collisions.
//
// Removal follows one rule in both modes: a row anything historical refers
// to (an invoice, a transfer, stock on hand, a coupon, an account entry) is
// deactivated, never deleted; only a row nothing refers to is really
// removed.

var errManagedByKashi = errors.New("managed by kashi while this till is connected to a central server")

// isStandalone reports whether this till runs without a central server.
func (a *App) isStandalone() (bool, error) {
	url, err := a.syncCentralAPIURL()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(url) == "", nil
}

// requireStandalone refuses a product/client edit kashi owns in connected
// mode.
func (a *App) requireStandalone() error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	standalone, err := a.isStandalone()
	if err != nil {
		return err
	}
	if !standalone {
		return errManagedByKashi
	}
	return nil
}

// IsStandaloneMode is the Wails binding the UI uses to show or hide product
// and client management.
func (a *App) IsStandaloneMode() (bool, error) {
	if a.store == nil {
		return false, fmt.Errorf("database not initialized")
	}
	return a.isStandalone()
}

// ProductInput is the editable part of a product in standalone mode.
type ProductInput struct {
	Barcode     string `json:"barcode"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"priceCents"`
	Discount    int64  `json:"discount"`
	Color       string `json:"color"`
	Size        string `json:"size"`
	IsActive    bool   `json:"isActive"`
}

func (in *ProductInput) normalize() error {
	in.Barcode = strings.TrimSpace(in.Barcode)
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.Color = strings.TrimSpace(in.Color)
	in.Size = strings.TrimSpace(in.Size)
	switch {
	case in.Barcode == "":
		return fmt.Errorf("barcode is required")
	case in.Name == "":
		return fmt.Errorf("name is required")
	case in.PriceCents <= 0:
		// priceInvoiceProduct rejects a 0 price, so such a product could
		// never be sold.
		return fmt.Errorf("price must be greater than 0")
	case in.Discount < 0 || in.Discount > 100:
		return fmt.Errorf("discount must be between 0 and 100")
	}
	if in.Code == "" {
		in.Code = in.Barcode
	}
	return nil
}

// managedProduct is the product shape the standalone product screen shows.
func managedProduct(p db.Product) map[string]any {
	return map[string]any{
		"id": p.ID, "uid": p.Uid.String, "barcode": p.Barcode, "code": p.Code,
		"name": p.Name, "description": p.Description,
		"priceCents": p.Price, "discount": p.Discount,
		"color": p.Color, "size": p.Size, "isActive": p.IsActive != 0,
	}
}

// ManagedProductPage is one page of the product management list.
type ManagedProductPage struct {
	Items      []map[string]any `json:"items"`
	NextCursor int64            `json:"nextCursor"`
	HasMore    bool             `json:"hasMore"`
}

// ListManagedProducts lists every product, active or not, for the product
// management screen. Read-only, so it works in both modes.
func (a *App) ListManagedProducts(afterID int64, limit int, search string) (ManagedProductPage, error) {
	if a.store == nil {
		return ManagedProductPage{}, fmt.Errorf("database not initialized")
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := a.store.ListProductsForManagement(a.ctx, db.ListProductsForManagementParams{
		AfterID: afterID, Search: strings.TrimSpace(search), Limit: int64(limit + 1),
	})
	if err != nil {
		return ManagedProductPage{}, err
	}
	page := ManagedProductPage{NextCursor: afterID, HasMore: len(rows) > limit}
	if page.HasMore {
		rows = rows[:limit]
	}
	page.Items = make([]map[string]any, len(rows))
	for i, p := range rows {
		page.Items[i] = managedProduct(p)
	}
	if len(rows) > 0 {
		page.NextCursor = rows[len(rows)-1].ID
	}
	return page, nil
}

// CreateProduct adds a product (standalone mode only).
func (a *App) CreateProduct(in ProductInput) (map[string]any, error) {
	if err := a.requireStandalone(); err != nil {
		return nil, err
	}
	if err := in.normalize(); err != nil {
		return nil, err
	}
	if _, err := a.store.GetProductByBarcode(a.ctx, in.Barcode); err == nil {
		return nil, fmt.Errorf("a product with barcode %s already exists", in.Barcode)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	created, err := a.store.CreateProduct(a.ctx, db.CreateProductParams{
		Barcode: in.Barcode, Code: in.Code, Name: in.Name, Description: in.Description,
		IsActive: boolInt(in.IsActive), Price: in.PriceCents, Discount: in.Discount,
		Color: in.Color, Size: in.Size, CurrencyCode: "USD",
	})
	if err != nil {
		return nil, err
	}
	// Re-read: the uid is minted by an AFTER INSERT trigger, which the
	// INSERT's RETURNING row doesn't include.
	product, err := a.store.GetProductByID(a.ctx, created.ID)
	if err != nil {
		return nil, err
	}
	return managedProduct(product), nil
}

// UpdateProduct edits a product (standalone mode only). Attributes this
// screen doesn't edit (brand, season, currency, ...) are kept as they are.
func (a *App) UpdateProduct(productID int64, in ProductInput) (map[string]any, error) {
	if err := a.requireStandalone(); err != nil {
		return nil, err
	}
	if err := in.normalize(); err != nil {
		return nil, err
	}
	existing, err := a.store.GetProductByID(a.ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("product not found")
	}
	if other, err := a.store.GetProductByBarcode(a.ctx, in.Barcode); err == nil && other.ID != productID {
		return nil, fmt.Errorf("a product with barcode %s already exists", in.Barcode)
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	updated, err := a.store.UpdateProduct(a.ctx, db.UpdateProductParams{
		ID: productID, Barcode: in.Barcode, Code: in.Code, Name: in.Name, Description: in.Description,
		IsActive: boolInt(in.IsActive), Price: in.PriceCents, Discount: in.Discount,
		Color: in.Color, Size: in.Size, CurrencyCode: existing.CurrencyCode,
		Brand: existing.Brand, Family: existing.Family, Kind: existing.Kind,
		Season: existing.Season, ProductYear: existing.ProductYear,
	})
	if err != nil {
		return nil, err
	}
	return managedProduct(updated), nil
}

// DeleteProduct removes a product (standalone mode only) -- or, when any
// history refers to it, deactivates it instead. It returns "deleted" or
// "deactivated".
func (a *App) DeleteProduct(productID int64) (string, error) {
	if err := a.requireStandalone(); err != nil {
		return "", err
	}
	if _, err := a.store.GetProductByID(a.ctx, productID); err != nil {
		return "", fmt.Errorf("product not found")
	}
	outcome := "deleted"
	err := a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		referenced, err := q.ProductIsReferenced(a.ctx, productID)
		if err != nil {
			return err
		}
		if referenced != 0 {
			outcome = "deactivated"
			return q.SetProductActive(a.ctx, db.SetProductActiveParams{IsActive: 0, ID: productID})
		}
		for _, del := range []func(context.Context, int64) error{
			q.DeleteProductOwnedRows, q.DeleteProductInventoryRows, q.DeleteProductQuickAccess,
		} {
			if err := del(a.ctx, productID); err != nil {
				return err
			}
		}
		// Override rows reference the product through a nullable column.
		nullableID := sql.NullInt64{Int64: productID, Valid: true}
		if err := q.DeleteProductOverrides(a.ctx, nullableID); err != nil {
			return err
		}
		if err := q.DeleteProductDiscountOverrides(a.ctx, nullableID); err != nil {
			return err
		}
		return q.DeleteProduct(a.ctx, productID)
	})
	if err != nil {
		return "", err
	}
	return outcome, nil
}

// SetProductActive reactivates or deactivates a product (standalone only).
func (a *App) SetProductActive(productID int64, active bool) error {
	if err := a.requireStandalone(); err != nil {
		return err
	}
	return a.store.SetProductActive(a.ctx, db.SetProductActiveParams{IsActive: boolInt(active), ID: productID})
}

// SetClientActive reactivates or deactivates a client (standalone only).
func (a *App) SetClientActive(clientID int64, active bool) error {
	if err := a.requireStandalone(); err != nil {
		return err
	}
	client, err := a.store.GetClientByID(a.ctx, clientID)
	if err != nil {
		return fmt.Errorf("client not found")
	}
	if isWalkInClient(client) {
		return fmt.Errorf("the walk-in customer cannot be changed")
	}
	return a.store.SetClientActive(a.ctx, db.SetClientActiveParams{IsActive: boolInt(active), ID: clientID})
}
