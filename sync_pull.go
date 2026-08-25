package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	db "kashi-pos/db/sqlc"
)

// syncedCurrency/syncedCashboxAccount mirror the JSON shapes kashi's
// GET /branch/sync/changes returns for these entities (see
// api/branch_catchup.go in the kashi repo). Matched by their natural text
// key (code / name), never by numeric id — kashi's row ids and this
// install's local autoincrement ids are different number spaces, and
// overwriting by id risks silently repointing an existing local row at
// the wrong central entity.
type syncedCurrency struct {
	Code              string `json:"code"`
	Name              string `json:"name"`
	Symbol            string `json:"symbol"`
	IsDefault         bool   `json:"is_default"`
	IsActive          bool   `json:"is_active"`
	UnitsPerUsdMicros int64  `json:"units_per_usd_micros"`
	CashRoundingUnit  int64  `json:"cash_rounding_unit"`
}

type syncedCashboxAccount struct {
	Name         string `json:"name"`
	CurrencyCode string `json:"currency_code"`
	SortOrder    int64  `json:"sort_order"`
	Color        string `json:"color"`
}

// syncedBranchTarget mirrors branch_targets rows kashi's admin sets for
// this branch (see db/sqlc/models.go's BranchTarget in the kashi repo).
// Matched by remote id (kashi's own id) rather than a natural key --
// unlike currencies/cashbox accounts, targets have no other stable key,
// and there's no local editing to preserve since this branch never writes
// to them itself.
type syncedBranchTarget struct {
	ID           int64     `json:"id"`
	DateFrom     time.Time `json:"date_from"`
	DateTo       time.Time `json:"date_to"`
	TargetAmount int64     `json:"target_amount"`
	Color        string    `json:"color"`
}

// syncedProductVariant mirrors ListProductVariantsForSync's flattened row
// shape (kashi has separate products/product_variants tables; kashi-pos
// has one flat product row, so kashi flattens on its side rather than
// kashi-pos needing to reconstruct a product from parts). ColorName/
// SizeName are plain text, matched by name — see the package-level note
// on syncedCurrency for why matching is never done by numeric id.
type syncedProductVariant struct {
	Barcode     string         `json:"barcode"`
	Price       sql.NullInt64  `json:"price"`
	IsActive    bool           `json:"is_active"`
	Code        string         `json:"code"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	ColorName   sql.NullString `json:"color_name"`
	SizeName    sql.NullString `json:"size_name"`
	Brand       sql.NullString `json:"brand"`
	SubCategory sql.NullString `json:"sub_category"`
	Kind        sql.NullString `json:"kind"`
	Season      sql.NullString `json:"season"`
	Year        sql.NullString `json:"year"`
}

// applySyncedCurrency creates or updates a local currency to match what
// kashi reports. The local default currency (the fixed USC accounting
// anchor) is intentionally left alone even if kashi reports changes for
// it — kashi-pos's own UpdateCurrency query already refuses to touch a
// default currency's rate for the same reason, and this respects that
// same invariant rather than working around it.
func applySyncedCurrency(ctx context.Context, q *db.Queries, c syncedCurrency) error {
	existing, err := q.GetCurrencyByCode(ctx, c.Code)
	if err == sql.ErrNoRows {
		_, err := q.CreateCurrency(ctx, db.CreateCurrencyParams{
			Code:              c.Code,
			Name:              c.Name,
			Symbol:            c.Symbol,
			UnitsPerUsdMicros: c.UnitsPerUsdMicros,
			CashRoundingUnit:  c.CashRoundingUnit,
		})
		return err
	}
	if err != nil {
		return err
	}
	if existing.IsDefault {
		return nil
	}

	if _, err := q.UpdateCurrency(ctx, db.UpdateCurrencyParams{
		Code:              c.Code,
		Name:              c.Name,
		Symbol:            c.Symbol,
		UnitsPerUsdMicros: c.UnitsPerUsdMicros,
		CashRoundingUnit:  c.CashRoundingUnit,
	}); err != nil {
		return err
	}
	if existing.IsActive != c.IsActive {
		if _, err := q.SetCurrencyActive(ctx, db.SetCurrencyActiveParams{
			IsActive: c.IsActive,
			Code:     c.Code,
		}); err != nil {
			return err
		}
	}
	return nil
}

func applySyncedCashboxAccount(ctx context.Context, q *db.Queries, a syncedCashboxAccount) error {
	existing, err := q.GetCashboxAccountByName(ctx, a.Name)
	accountID := int64(0)
	sortOrder := int64(-1) // force the sort_order set below on a fresh create
	if err == sql.ErrNoRows {
		// CreateCashboxAccount only accepts name/currency_code (it
		// auto-assigns sort_order and defaults color to '') — color and the
		// pushed sort_order have to be applied as a follow-up update.
		created, err := q.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{
			Name:         a.Name,
			CurrencyCode: a.CurrencyCode,
		})
		if err != nil {
			return err
		}
		accountID = created.ID
	} else if err != nil {
		return err
	} else {
		accountID = existing.ID
		sortOrder = existing.SortOrder
	}

	if _, err := q.UpdateCashboxAccount(ctx, db.UpdateCashboxAccountParams{
		ID:           accountID,
		Name:         a.Name,
		CurrencyCode: a.CurrencyCode,
		Color:        a.Color,
	}); err != nil {
		return err
	}
	if sortOrder != a.SortOrder {
		if err := q.SetCashboxAccountSortOrder(ctx, db.SetCashboxAccountSortOrderParams{
			SortOrder: a.SortOrder,
			ID:        accountID,
		}); err != nil {
			return err
		}
	}
	return nil
}

// applySyncedBranchTarget is a plain upsert keyed by remote_id -- targets
// are read-only display data here, so there's no local state to reconcile
// beyond "match what kashi last reported".
func applySyncedBranchTarget(ctx context.Context, q *db.Queries, t syncedBranchTarget) error {
	return q.UpsertBranchTarget(ctx, db.UpsertBranchTargetParams{
		RemoteID:          t.ID,
		DateFrom:          t.DateFrom.Format("2006-01-02"),
		DateTo:            t.DateTo.Format("2006-01-02"),
		TargetAmountCents: t.TargetAmount,
		Color:             t.Color,
	})
}

// syncedSalesperson mirrors kashi's branch-scoped salespersons roster (see
// db/sqlc/models.go's Salesperson in the kashi repo). Matched by remote id
// like syncedBranchTarget -- names can be edited centrally, so there's no
// stable natural key to match on instead.
type syncedSalesperson struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

// applySyncedSalesperson upserts by remote_id and deliberately never
// touches attendance_user_id -- that's the local Akuvox device pairing for
// this salesperson at this branch, which kashi has no opinion on (see the
// comment on UpdateSyncedSalesperson in db/query/salesperson.sql).
func applySyncedSalesperson(ctx context.Context, q *db.Queries, s syncedSalesperson) error {
	isActive := int64(0)
	if s.IsActive {
		isActive = 1
	}
	remoteID := sql.NullInt64{Int64: s.ID, Valid: true}

	if _, err := q.GetSalespersonByRemoteID(ctx, remoteID); err == sql.ErrNoRows {
		_, err := q.CreateSyncedSalesperson(ctx, db.CreateSyncedSalespersonParams{
			FullName: s.Name,
			RemoteID: remoteID,
			IsActive: isActive,
		})
		return err
	} else if err != nil {
		return err
	}

	_, err := q.UpdateSyncedSalesperson(ctx, db.UpdateSyncedSalespersonParams{
		FullName: s.Name,
		IsActive: isActive,
		RemoteID: remoteID,
	})
	return err
}

// applySyncedProductVariant creates or updates a local product to match
// what kashi reports for one variant. Fields kashi doesn't model at all
// today (discount, currency_code, brand/family/kind/season/product_year)
// are left at whatever this install already has — an update never
// resets them, since kashi has no opinion on them yet.
func applySyncedProductVariant(ctx context.Context, q *db.Queries, v syncedProductVariant) error {
	isActive := int64(0)
	if v.IsActive {
		isActive = 1
	}
	color := ""
	if v.ColorName.Valid {
		color = v.ColorName.String
	}
	size := ""
	if v.SizeName.Valid {
		size = v.SizeName.String
	}
	price := int64(0)
	if v.Price.Valid {
		price = v.Price.Int64
	}

	var productID int64
	existing, err := q.GetProductByBarcode(ctx, v.Barcode)
	if err == sql.ErrNoRows {
		created, err := q.CreateProduct(ctx, db.CreateProductParams{
			Barcode:      v.Barcode,
			Code:         v.Code,
			Name:         v.Name,
			Description:  v.Description,
			IsActive:     isActive,
			Price:        price,
			Color:        color,
			Size:         size,
			CurrencyCode: "USD",
		})
		if err != nil {
			return err
		}
		productID = created.ID
	} else if err != nil {
		return err
	} else {
		if _, err := q.UpdateProduct(ctx, db.UpdateProductParams{
			ID:           existing.ID,
			Barcode:      v.Barcode,
			Code:         v.Code,
			Name:         v.Name,
			Description:  v.Description,
			IsActive:     isActive,
			Price:        price,
			Color:        color,
			Size:         size,
			Discount:     existing.Discount,
			CurrencyCode: existing.CurrencyCode,
			Brand:        existing.Brand,
			Family:       existing.Family,
			Kind:         existing.Kind,
			Season:       existing.Season,
			ProductYear:  existing.ProductYear,
		}); err != nil {
			return err
		}
		productID = existing.ID
	}

	return applySyncedProductAttributes(ctx, q, productID, v)
}

// applySyncedProductAttributes writes brand/sub-category/kind/season/year
// onto the shared attribute system (see 000073_create_product_attributes.up.sql),
// not the legacy products columns — those are frozen/vestigial from the
// main app's perspective now (see productMap in products.go). Attribute
// ids (2=brand, 1=sub-category, 3=kind, 7=season, 6=year) match kashi's
// fixed seed exactly, so this doesn't need a name lookup.
func applySyncedProductAttributes(ctx context.Context, q *db.Queries, productID int64, v syncedProductVariant) error {
	type assignment struct {
		attributeID int64
		value       sql.NullString
	}
	for _, a := range []assignment{
		{2, v.Brand},
		{1, v.SubCategory},
		{3, v.Kind},
		{7, v.Season},
		{6, v.Year},
	} {
		if !a.value.Valid || a.value.String == "" {
			continue
		}
		attrValue, err := q.UpsertAttributeValue(ctx, db.UpsertAttributeValueParams{
			AttributeID: a.attributeID,
			Value:       a.value.String,
		})
		if err != nil {
			return fmt.Errorf("upsert attribute value (attribute_id=%d): %w", a.attributeID, err)
		}
		if err := q.SetProductAttribute(ctx, db.SetProductAttributeParams{
			AttributeID:      a.attributeID,
			AttributeValueID: attrValue.ID,
			ProductID:        productID,
		}); err != nil {
			return fmt.Errorf("set product attribute (attribute_id=%d): %w", a.attributeID, err)
		}
	}
	return nil
}

// pullEntities lists what gets pulled from kashi on each tick. Adding a
// new pulled entity is: extend this list, add its synced* struct and
// apply* function above.
var pullEntities = []string{"currencies", "cashbox_accounts", "products", "clients", "branch_targets", "salespersons"}

// runSyncPullOnce catches this branch up on every entity in pullEntities,
// one at a time. Each entity's cursor advances independently — a decode
// or apply failure on one entity doesn't block the others, and next
// tick's since is still whatever cursor was last successfully saved (never
// advanced past changes that failed to apply).
func (a *App) runSyncPullOnce(ctx context.Context, client *centralClient) {
	for _, entity := range pullEntities {
		if err := a.pullEntityOnce(ctx, client, entity); err != nil {
			log.Printf("sync pull %s: %v", entity, err)
		}
	}
}

func (a *App) pullEntityOnce(ctx context.Context, client *centralClient, entity string) error {
	cursor, err := a.store.GetSyncCursor(ctx, entity)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("load cursor: %w", err)
	}

	changes, err := client.FetchChanges(ctx, entity, cursor)
	if err != nil {
		return fmt.Errorf("fetch changes: %w", err)
	}

	if err := a.store.ExecTx(ctx, func(q *db.Queries) error {
		switch entity {
		case "currencies":
			var items []syncedCurrency
			if err := json.Unmarshal(changes.Items, &items); err != nil {
				return fmt.Errorf("decode items: %w", err)
			}
			for _, item := range items {
				if err := applySyncedCurrency(ctx, q, item); err != nil {
					return fmt.Errorf("apply currency %s: %w", item.Code, err)
				}
			}
		case "cashbox_accounts":
			var items []syncedCashboxAccount
			if err := json.Unmarshal(changes.Items, &items); err != nil {
				return fmt.Errorf("decode items: %w", err)
			}
			for _, item := range items {
				if err := applySyncedCashboxAccount(ctx, q, item); err != nil {
					return fmt.Errorf("apply cashbox account %s: %w", item.Name, err)
				}
			}
		case "products":
			var items []syncedProductVariant
			if err := json.Unmarshal(changes.Items, &items); err != nil {
				return fmt.Errorf("decode items: %w", err)
			}
			for _, item := range items {
				if err := applySyncedProductVariant(ctx, q, item); err != nil {
					return fmt.Errorf("apply product variant %s: %w", item.Barcode, err)
				}
			}
		case "clients":
			var items []syncedClient
			if err := json.Unmarshal(changes.Items, &items); err != nil {
				return fmt.Errorf("decode items: %w", err)
			}
			for _, item := range items {
				if err := applySyncedClient(ctx, q, item); err != nil {
					return fmt.Errorf("apply client %s: %w", item.Phone, err)
				}
			}
		case "branch_targets":
			var items []syncedBranchTarget
			if err := json.Unmarshal(changes.Items, &items); err != nil {
				return fmt.Errorf("decode items: %w", err)
			}
			for _, item := range items {
				if err := applySyncedBranchTarget(ctx, q, item); err != nil {
					return fmt.Errorf("apply branch target %d: %w", item.ID, err)
				}
			}
		case "salespersons":
			var items []syncedSalesperson
			if err := json.Unmarshal(changes.Items, &items); err != nil {
				return fmt.Errorf("decode items: %w", err)
			}
			for _, item := range items {
				if err := applySyncedSalesperson(ctx, q, item); err != nil {
					return fmt.Errorf("apply salesperson %d: %w", item.ID, err)
				}
			}
		}
		return q.SetSyncCursor(ctx, db.SetSyncCursorParams{
			Entity: entity,
			Cursor: changes.ServerTime.Format(time.RFC3339),
		})
	}); err != nil {
		return err
	}

	return nil
}
