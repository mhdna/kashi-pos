package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	db "kashi-pos/db/sqlc"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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
	IsActive     bool      `json:"is_active"`
}

// syncedProductVariant mirrors ListProductVariantsForSync's flattened row
// shape (kashi has separate products/product_variants tables; kashi-pos
// has one flat product row, so kashi flattens on its side rather than
// kashi-pos needing to reconstruct a product from parts). ColorName/
// SizeName are plain text, matched by name — see the package-level note
// on syncedCurrency for why matching is never done by numeric id.
type syncedProductVariant struct {
	// UID is the variant's shared identity; empty from an older kashi.
	UID         string         `json:"uid"`
	UpdatedAt   string         `json:"updated_at"`
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

// syncedExpenseCategory / syncedLoanCategory mirror the raw row kashi
// returns for scope='branch' expense_categories / loan_categories (see
// api/branch_catchup.go). Matched by name -- see the package-level note on
// syncedCurrency for why matching is never done by numeric id. icon/color
// are carried straight through so a category renders identically here and
// in kashi.
type syncedExpenseCategory struct {
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
	Icon     string `json:"icon"`
	Color    string `json:"color"`
}

type syncedLoanCategory struct {
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
	Icon     string `json:"icon"`
	Color    string `json:"color"`
}

// applySyncedCurrency creates or updates a local currency to match what
// kashi reports. The local default currency (the fixed USC accounting
// anchor) is intentionally left alone even if kashi reports changes for
// it — kashi-pos's own UpdateCurrency query already refuses to touch a
// default currency's rate for the same reason, and this respects that
// same invariant rather than working around it.
func applySyncedCurrency(ctx context.Context, q *db.Queries, c syncedCurrency) error {
	// The hand-entry path validates these (currencies.go), and the table's
	// own CHECK constraints require both to be > 0 - so a bad pulled value
	// would either abort the whole pull or, on a rounding currency, make
	// every invoice total fail to price. Reject the row, not the sync.
	if c.UnitsPerUsdMicros <= 0 || c.CashRoundingUnit <= 0 {
		return fmt.Errorf("currency %s: invalid rate %d or rounding unit %d", c.Code, c.UnitsPerUsdMicros, c.CashRoundingUnit)
	}
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

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// applySyncedExpenseCategory / applySyncedLoanCategory upsert a local
// category row by name, mirroring applySyncedCashboxAccount's
// get-by-natural-key then create-or-update shape.
func applySyncedExpenseCategory(ctx context.Context, q *db.Queries, c syncedExpenseCategory) error {
	existing, err := q.GetExpenseCategoryByName(ctx, c.Name)
	if err == sql.ErrNoRows {
		_, err := q.CreateExpenseCategory(ctx, db.CreateExpenseCategoryParams{
			Name:     c.Name,
			IsActive: boolToInt64(c.IsActive),
			Icon:     c.Icon,
			Color:    c.Color,
		})
		return err
	}
	if err != nil {
		return err
	}
	_, err = q.UpdateExpenseCategory(ctx, db.UpdateExpenseCategoryParams{
		ID:       existing.ID,
		Name:     c.Name,
		IsActive: boolToInt64(c.IsActive),
		Icon:     c.Icon,
		Color:    c.Color,
	})
	return err
}

func applySyncedLoanCategory(ctx context.Context, q *db.Queries, c syncedLoanCategory) error {
	existing, err := q.GetLoanCategoryByName(ctx, c.Name)
	if err == sql.ErrNoRows {
		_, err := q.CreateLoanCategory(ctx, db.CreateLoanCategoryParams{
			Name:     c.Name,
			IsActive: boolToInt64(c.IsActive),
			Icon:     c.Icon,
			Color:    c.Color,
		})
		return err
	}
	if err != nil {
		return err
	}
	_, err = q.UpdateLoanCategory(ctx, db.UpdateLoanCategoryParams{
		ID:       existing.ID,
		Name:     c.Name,
		IsActive: boolToInt64(c.IsActive),
		Icon:     c.Icon,
		Color:    c.Color,
	})
	return err
}

// applySyncedBranchTarget is a plain upsert keyed by remote_id -- targets
// are read-only display data here, so there's no local state to reconcile
// beyond "match what kashi last reported".
func applySyncedBranchTarget(ctx context.Context, q *db.Queries, t syncedBranchTarget) error {
	if !t.IsActive {
		return q.DeleteBranchTargetByRemoteID(ctx, t.ID)
	}
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

// syncedBranchUser mirrors kashi's branch_users roster row (see
// api/branch_user.go in the kashi repo). The PIN is never in this feed --
// password_hash is set only by a set_branch_user_pin command or a local
// admin reset, and applySyncedBranchUser never touches it.
type syncedBranchUser struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	AkuvoxUserID string `json:"akuvox_user_id"`
	IsActive     bool   `json:"is_active"`
}

// applySyncedBranchUser upserts a POS user to match kashi's roster. Matched
// first by remote_id; a username-only match (a user the till created
// locally before its first roster pull -- see the set_branch_user_pin
// command) is adopted rather than duplicated.
func applySyncedBranchUser(ctx context.Context, q *db.Queries, u syncedBranchUser) error {
	roleID, err := roleIDForName(ctx, q, u.Role)
	if err != nil {
		return err
	}
	active := int64(0)
	if u.IsActive {
		active = 1
	}
	remoteID := sql.NullInt64{Int64: u.ID, Valid: true}

	if _, err := q.GetUserByRemoteID(ctx, remoteID); err == nil {
		_, err := q.UpdateSyncedUser(ctx, db.UpdateSyncedUserParams{
			Username:         u.Username,
			RoleID:           roleID,
			Active:           active,
			AttendanceUserID: u.AkuvoxUserID,
			RemoteID:         remoteID,
		})
		return err
	} else if err != sql.ErrNoRows {
		return err
	}

	if _, err := q.GetUserByUsername(ctx, u.Username); err == nil {
		_, err := q.AdoptUserAsSynced(ctx, db.AdoptUserAsSyncedParams{
			RoleID:           roleID,
			Active:           active,
			AttendanceUserID: u.AkuvoxUserID,
			RemoteID:         remoteID,
			Username:         u.Username,
		})
		return err
	} else if err != sql.ErrNoRows {
		return err
	}

	// A synced user with no PIN yet can't log in until a set_branch_user_pin
	// command lands -- password_hash is a placeholder until then.
	_, err = q.CreateSyncedUser(ctx, db.CreateSyncedUserParams{
		Username:         u.Username,
		PasswordHash:     "",
		RoleID:           roleID,
		Active:           active,
		AttendanceUserID: u.AkuvoxUserID,
		RemoteID:         remoteID,
	})
	return err
}

// roleIDForName resolves a kashi role name ("admin"/"cashier") to this
// install's local roles.id, falling back to "cashier" for anything
// unexpected so a synced user is never left role-less.
func roleIDForName(ctx context.Context, q *db.Queries, name string) (int64, error) {
	if r, err := q.GetRoleByName(ctx, name); err == nil {
		return r.ID, nil
	} else if err != sql.ErrNoRows {
		return 0, err
	}
	r, err := q.GetRoleByName(ctx, "cashier")
	if err != nil {
		return 0, fmt.Errorf("no local role %q and no cashier fallback: %w", name, err)
	}
	return r.ID, nil
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
	existing, err := findSyncedProduct(ctx, q, v)
	// A product priced at 0 is unsellable - priceInvoiceProduct rejects it
	// and every quote containing it fails, which surfaces at the till as a
	// total that will not price. An absent price in the pull payload means
	// "central didn't say", not "free", so keep what we already have.
	if err == nil && !v.Price.Valid {
		price = existing.Price
	}
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
		if v.UID != "" {
			if err := q.SetProductUID(ctx, db.SetProductUIDParams{Uid: sql.NullString{String: v.UID, Valid: true}, ID: productID}); err != nil {
				return err
			}
		}
	} else if err != nil {
		return err
	} else {
		if existing.Barcode != v.Barcode {
			if err := freeProductBarcode(ctx, q, v.Barcode, existing.ID); err != nil {
				return err
			}
		}
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

	if err := applySyncedProductAttributes(ctx, q, productID, v); err != nil {
		return err
	}
	return stampRemoteVersion(ctx, v.UpdatedAt, func(ver string) error {
		return q.SetProductRemoteUpdatedAt(ctx, db.SetProductRemoteUpdatedAtParams{RemoteUpdatedAt: sql.NullString{String: ver, Valid: true}, ID: productID})
	})
}

// findSyncedProduct finds the local copy of a kashi variant: by uid, else by
// barcode -- in which case the row adopts kashi's uid on the spot, so every
// later update matches by uid (and a barcode change in kashi updates the
// same row instead of creating a new one). sql.ErrNoRows means "no local
// copy yet".
func findSyncedProduct(ctx context.Context, q *db.Queries, v syncedProductVariant) (db.Product, error) {
	if v.UID != "" {
		p, err := q.GetProductByUID(ctx, sql.NullString{String: v.UID, Valid: true})
		if err == nil || !errors.Is(err, sql.ErrNoRows) {
			return p, err
		}
	}
	p, err := q.GetProductByBarcode(ctx, v.Barcode)
	if err != nil {
		return p, err
	}
	if v.UID != "" {
		if err := q.SetProductUID(ctx, db.SetProductUIDParams{Uid: sql.NullString{String: v.UID, Valid: true}, ID: p.ID}); err != nil {
			return p, err
		}
	}
	return p, nil
}

// freeProductBarcode makes barcode available to product keepID. Barcodes are
// globally unique in kashi and pulls apply in updated_at order, so a
// different local row still holding it is a stale local-only product: it is
// deactivated and its barcode suffixed, keeping its invoice history intact.
func freeProductBarcode(ctx context.Context, q *db.Queries, barcode string, keepID int64) error {
	other, err := q.GetProductByBarcode(ctx, barcode)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if other.ID == keepID {
		return nil
	}
	return q.SetProductBarcodeInactive(ctx, db.SetProductBarcodeInactiveParams{
		Barcode: fmt.Sprintf("%s#stale-%d", barcode, other.ID),
		ID:      other.ID,
	})
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
var pullEntities = []string{"currencies", "cashbox_accounts", "products", "price_lists", "discount_lists", "clients", "branch_targets", "salespersons", "branch_users", "expense_categories", "loan_categories", "coupon_types", "coupons", "branches", "transfer_requests"}

// runSyncPullOnce catches this branch up on every entity in pullEntities,
// one at a time. Each entity's cursor advances independently — a decode
// or apply failure on one entity doesn't block the others, and next
// tick's since is still whatever cursor was last successfully saved (never
// advanced past changes that failed to apply).
func (a *App) runSyncPullOnce(ctx context.Context, client *centralClient) {
	// A full pass is the same work whether the tick or the WS catch-up runs
	// it, so if another pull already holds the lock there's nothing to wait
	// for -- skip and let the holder finish. The 15s tick is the backstop if
	// a WS-triggered pull was the one skipped.
	if !a.syncPullMu.TryLock() {
		return
	}
	defer a.syncPullMu.Unlock()
	a.runSyncPullLocked(ctx, client)
}

// runSyncPullLocked is the pull pass itself, with the caller already holding
// a.syncPullMu -- so ResyncAll can hold that lock across its whole run
// (blocking the tick's and the WS client's pulls) and still drive one pull
// itself.
func (a *App) runSyncPullLocked(ctx context.Context, client *centralClient) {
	for _, entity := range pullEntities {
		if err := a.pullEntityLocked(ctx, client, entity); err != nil {
			log.Printf("sync pull %s: %v", entity, err)
		}
	}
}

// pullEntityOnce is the entry point for a single WS delta ("go re-fetch this
// entity"). Unlike the full pass it blocks for the lock rather than skipping:
// a delta names a specific change that must land, and once it does hold the
// lock FetchChanges is cheap if an overlapping full pull already applied it.
func (a *App) pullEntityOnce(ctx context.Context, client *centralClient, entity string) error {
	a.syncPullMu.Lock()
	defer a.syncPullMu.Unlock()
	return a.pullEntityLocked(ctx, client, entity)
}

// syncPullPageLimit is the page size the branch asks kashi for on the two
// unbounded catch-up entities (products, clients). kashi caps it at 1000;
// the small entities ignore it and answer in one shot.
const syncPullPageLimit = 500

func (a *App) pullEntityLocked(ctx context.Context, client *centralClient, entity string) error {
	since, err := a.store.GetSyncCursor(ctx, entity)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("load cursor: %w", err)
	}

	// Walk pages until kashi stops handing back a next_cursor. Each page is
	// applied in its own transaction so a first-ever pull of a big catalog
	// isn't one multi-minute write lock; the entity's stored cursor is only
	// advanced (to server_time) inside the final page's transaction, so a
	// mid-walk failure just means the next tick re-pulls from the last
	// confirmed point -- every apply is an idempotent upsert, so replaying
	// an already-applied page is harmless.
	pageCursor := ""
	totalApplied := 0
	for {
		changes, err := client.FetchChanges(ctx, fetchChangesParams{
			Entity: entity,
			Since:  since,
			Cursor: pageCursor,
			Limit:  syncPullPageLimit,
		})
		if err != nil {
			return fmt.Errorf("fetch changes: %w", err)
		}

		var rawItems []json.RawMessage
		if err := json.Unmarshal(changes.Items, &rawItems); err != nil {
			return fmt.Errorf("count items: %w", err)
		}
		totalApplied += len(rawItems)
		isLastPage := changes.NextCursor == ""

		if err := a.store.ExecTx(ctx, func(q *db.Queries) error {
			if err := applySyncedEntityBatch(ctx, q, entity, changes.Items); err != nil {
				return err
			}
			if isLastPage {
				return q.SetSyncCursor(ctx, db.SetSyncCursorParams{
					Entity: entity,
					Cursor: changes.ServerTime.Format(time.RFC3339),
				})
			}
			return nil
		}); err != nil {
			return err
		}

		if isLastPage {
			break
		}
		pageCursor = changes.NextCursor
	}

	// Tell the frontend to re-fetch, mirroring the WS push's "go re-fetch
	// this" contract (see wsMessageEntity in sync_ws.go). Skipped when the
	// whole walk applied nothing, so it doesn't fire every 15s for nothing.
	if totalApplied > 0 {
		a.emitSyncApplied(entity)
	}

	return nil
}

// applySyncedEntityBatch decodes one page of a catch-up response for
// `entity` and upserts every row. Split out of pullEntityLocked so the
// page-walk loop stays readable; the switch is the same one that lived
// inline before pagination.
func applySyncedEntityBatch(ctx context.Context, q *db.Queries, entity string, raw json.RawMessage) error {
	switch entity {
	case "currencies":
		var items []syncedCurrency
		if err := json.Unmarshal(raw, &items); err != nil {
			return fmt.Errorf("decode items: %w", err)
		}
		for _, item := range items {
			if err := applySyncedCurrency(ctx, q, item); err != nil {
				return fmt.Errorf("apply currency %s: %w", item.Code, err)
			}
		}
		// kashi sends its whole set for this entity, so anything absent is
		// something kashi does not have -- deactivate it rather than leaving
		// a currency on the till that central knows nothing about.
		if err := reconcileCurrencies(ctx, q, items); err != nil {
			return fmt.Errorf("reconcile currencies: %w", err)
		}
	case "cashbox_accounts":
		var items []syncedCashboxAccount
		if err := json.Unmarshal(raw, &items); err != nil {
			return fmt.Errorf("decode items: %w", err)
		}
		for _, item := range items {
			if err := applySyncedCashboxAccount(ctx, q, item); err != nil {
				return fmt.Errorf("apply cashbox account %s: %w", item.Name, err)
			}
		}
		if err := reconcileCashboxAccounts(ctx, q, items); err != nil {
			return fmt.Errorf("reconcile payment methods: %w", err)
		}
	case "products":
		var items []syncedProductVariant
		if err := json.Unmarshal(raw, &items); err != nil {
			return fmt.Errorf("decode items: %w", err)
		}
		for _, item := range items {
			if err := applySyncedProductVariant(ctx, q, item); err != nil {
				return fmt.Errorf("apply product variant %s: %w", item.Barcode, err)
			}
		}
	case "price_lists":
		var items []syncedPriceList
		if err := json.Unmarshal(raw, &items); err != nil {
			return fmt.Errorf("decode items: %w", err)
		}
		for _, item := range items {
			if err := applySyncedPriceList(ctx, q, item); err != nil {
				return fmt.Errorf("apply price list %s: %w", item.Name, err)
			}
		}
	case "discount_lists":
		var items []syncedDiscountList
		if err := json.Unmarshal(raw, &items); err != nil {
			return fmt.Errorf("decode items: %w", err)
		}
		for _, item := range items {
			if err := applySyncedDiscountList(ctx, q, item); err != nil {
				return fmt.Errorf("apply discount list %s: %w", item.Name, err)
			}
		}
	case "clients":
		var items []syncedClient
		if err := json.Unmarshal(raw, &items); err != nil {
			return fmt.Errorf("decode items: %w", err)
		}
		for _, item := range items {
			if err := applySyncedClient(ctx, q, item); err != nil {
				return fmt.Errorf("apply client %s: %w", item.Phone, err)
			}
		}
	case "branch_targets":
		var items []syncedBranchTarget
		if err := json.Unmarshal(raw, &items); err != nil {
			return fmt.Errorf("decode items: %w", err)
		}
		for _, item := range items {
			if err := applySyncedBranchTarget(ctx, q, item); err != nil {
				return fmt.Errorf("apply branch target %d: %w", item.ID, err)
			}
		}
	case "salespersons":
		var items []syncedSalesperson
		if err := json.Unmarshal(raw, &items); err != nil {
			return fmt.Errorf("decode items: %w", err)
		}
		for _, item := range items {
			if err := applySyncedSalesperson(ctx, q, item); err != nil {
				return fmt.Errorf("apply salesperson %d: %w", item.ID, err)
			}
		}
	case "branch_users":
		var items []syncedBranchUser
		if err := json.Unmarshal(raw, &items); err != nil {
			return fmt.Errorf("decode items: %w", err)
		}
		for _, item := range items {
			if err := applySyncedBranchUser(ctx, q, item); err != nil {
				return fmt.Errorf("apply branch user %d: %w", item.ID, err)
			}
		}
	case "expense_categories":
		var items []syncedExpenseCategory
		if err := json.Unmarshal(raw, &items); err != nil {
			return fmt.Errorf("decode items: %w", err)
		}
		for _, item := range items {
			if err := applySyncedExpenseCategory(ctx, q, item); err != nil {
				return fmt.Errorf("apply expense category %s: %w", item.Name, err)
			}
		}
	case "loan_categories":
		var items []syncedLoanCategory
		if err := json.Unmarshal(raw, &items); err != nil {
			return fmt.Errorf("decode items: %w", err)
		}
		for _, item := range items {
			if err := applySyncedLoanCategory(ctx, q, item); err != nil {
				return fmt.Errorf("apply loan category %s: %w", item.Name, err)
			}
		}
	case "coupon_types":
		var items []syncedCouponType
		if err := json.Unmarshal(raw, &items); err != nil {
			return fmt.Errorf("decode items: %w", err)
		}
		for _, item := range items {
			if err := applySyncedCouponType(ctx, q, item); err != nil {
				return fmt.Errorf("apply coupon type %s: %w", item.Name, err)
			}
		}
	case "coupons":
		var items []syncedCoupon
		if err := json.Unmarshal(raw, &items); err != nil {
			return fmt.Errorf("decode items: %w", err)
		}
		for _, item := range items {
			if err := applySyncedCoupon(ctx, q, item); err != nil {
				return fmt.Errorf("apply coupon %s: %w", item.Code, err)
			}
		}
	case "branches":
		var items []syncedBranch
		if err := json.Unmarshal(raw, &items); err != nil {
			return fmt.Errorf("decode items: %w", err)
		}
		for _, item := range items {
			if err := applySyncedBranch(ctx, q, item); err != nil {
				return fmt.Errorf("apply branch %s: %w", item.Code, err)
			}
		}
	case "transfer_requests":
		var items []syncedTransferRequest
		if err := json.Unmarshal(raw, &items); err != nil {
			return fmt.Errorf("decode items: %w", err)
		}
		for _, item := range items {
			if err := applySyncedTransferRequest(ctx, q, item); err != nil {
				return fmt.Errorf("apply transfer request %d: %w", item.ID, err)
			}
		}
	}
	return nil
}

// emitSyncApplied is a Wails-lifecycle-safe wrapper around
// runtime.EventsEmit: calling it before a.ctx is the real context handed
// to App.startup (e.g. in tests, which construct an App directly with a
// plain context.Background()) crashes the process outright rather than
// erroring, so this only emits once a.ready has closed -- a nil (never
// closed) ready channel, as in tests, makes the select fall through to
// default immediately, every time.
func (a *App) emitSyncApplied(entity string) {
	select {
	case <-a.ready:
		runtime.EventsEmit(a.ctx, "sync_applied:"+entity)
	default:
	}
}

// emitSettingsChanged tells the frontend a feature flag / settings value
// changed out from under it -- e.g. the product-search button was toggled
// remotely from kashi, or on another page of this app. Same ready-channel
// guard as emitSyncApplied, for the same reason.
func (a *App) emitSettingsChanged() {
	select {
	case <-a.ready:
		runtime.EventsEmit(a.ctx, "settings_changed")
	default:
	}
}

// emitExpenseUploadReady tells the frontend the one-time receipt-upload QR
// link for a just-synced expense is ready, carrying the full URL as event
// data. Same ready-channel guard as emitSyncApplied, for the same reason.
func (a *App) emitExpenseUploadReady(clientRef, uploadURL string) {
	select {
	case <-a.ready:
		runtime.EventsEmit(a.ctx, "expense_upload_ready:"+clientRef, uploadURL)
	default:
	}
}

// emitTransferUploadReady is emitExpenseUploadReady's counterpart for a
// transfer request's one-time photo-upload QR link.
func (a *App) emitTransferUploadReady(clientRef, uploadURL string) {
	select {
	case <-a.ready:
		runtime.EventsEmit(a.ctx, "transfer_upload_ready:"+clientRef, uploadURL)
	default:
	}
}

// emitExpenseImagesUploaded tells the frontend a phone finished uploading
// receipt photos for a given expense (pushed from kashi over the branch WS
// -- see sync_ws.go), so the open QR dialog can flip to its success state.
func (a *App) emitExpenseImagesUploaded(clientRef string) {
	select {
	case <-a.ready:
		runtime.EventsEmit(a.ctx, "expense_images_uploaded:"+clientRef)
	default:
	}
}

// emitSyncStateChanged tells the frontend the push side of sync advanced --
// a row was accepted by kashi, moved to failed, or re-armed by a resync --
// so any open sync-status indicator (the nav-rail dot, the per-invoice
// dots, the settings page) can refetch. Payload-free, same pull-not-push
// discipline as the branch WS: the frontend reacts by calling GetSyncStatus
// / reloading the invoice page, never by trusting event data. Same
// ready-channel guard as emitSyncApplied.
func (a *App) emitSyncStateChanged() {
	select {
	case <-a.ready:
		runtime.EventsEmit(a.ctx, "sync_state_changed")
	default:
	}
}
