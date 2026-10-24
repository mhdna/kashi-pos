package main

import (
	"context"
	"database/sql"
	"fmt"

	db "kashi-pos/db/sqlc"
)

// Price and discount lists arrive whole -- a list row with its complete item
// set inline -- rather than as item-level deltas. kashi sends them that way
// (see api/branch_sync_lists.go in the kashi repo) because nothing in this
// sync can express "this one item is gone": there is no row left to carry a
// flag, and the periodic full-mirror check only reconciles clients and
// products. Receiving the whole list lets the apply below clear and refill
// it, so a removal is expressed by the item simply not being in the payload.
//
// Both apply functions must stay idempotent: pullEntityLocked only advances
// the sync cursor after a full successful walk, so a failure mid-page means
// the same payload is applied again on the next tick.

type syncedPriceListItem struct {
	ProductVariantUID string `json:"product_variant_uid"`
	Price             int64  `json:"price"`
}

type syncedPriceList struct {
	UID       string                `json:"uid"`
	Name      string                `json:"name"`
	IsActive  bool                  `json:"is_active"`
	IsDefault bool                  `json:"is_default"`
	IsSpecial bool                  `json:"is_special"`
	Items     []syncedPriceListItem `json:"items"`
}

type syncedDiscountListItem struct {
	ProductVariantUID string `json:"product_variant_uid"`
	Discount          int64  `json:"discount"`
	// The deal the variant is on (deals.go), one of the list's Deals.
	DealUID string `json:"deal_uid,omitempty"`
}

// A deal's terms, sent with every list whose rows name it.
type syncedDeal struct {
	UID         string `json:"uid"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	BuyQty      int64  `json:"buy_qty"`
	GetQty      int64  `json:"get_qty"`
	Percent     int64  `json:"percent"`
	BundlePrice int64  `json:"bundle_price"`
	// What units outside a set get (kashi 000205); 0 for none.
	FallbackPercent int64 `json:"fallback_percent"`
}

// A discount that applies only when the line is sold as the named sale
// option -- see 000126_sale_options.up.sql.
type syncedDiscountListOptionItem struct {
	ProductVariantUID string `json:"product_variant_uid"`
	SaleOption        string `json:"sale_option"`
	Discount          int64  `json:"discount"`
}

type syncedDiscountList struct {
	UID         string                         `json:"uid"`
	Name        string                         `json:"name"`
	IsActive    bool                           `json:"is_active"`
	IsDefault   bool                           `json:"is_default"`
	IsSpecial   bool                           `json:"is_special"`
	Items       []syncedDiscountListItem       `json:"items"`
	OptionItems []syncedDiscountListOptionItem `json:"option_items"`
	Deals       []syncedDeal                   `json:"deals"`
}

// A list kashi reports as inactive is deleted outright rather than kept and
// flagged: an inactive list prices nothing, and leaving thousands of dead
// items behind on a till only slows the pricing queries down. It comes back
// in full if kashi reactivates it.
func applySyncedPriceList(ctx context.Context, q *db.Queries, l syncedPriceList) error {
	if l.UID == "" {
		return fmt.Errorf("price list %q has no uid", l.Name)
	}
	if !l.IsActive {
		return q.DeletePriceListByUID(ctx, sql.NullString{String: l.UID, Valid: true})
	}

	list, err := q.UpsertPriceListByUID(ctx, db.UpsertPriceListByUIDParams{
		Uid:       sql.NullString{String: l.UID, Valid: true},
		Name:      l.Name,
		IsActive:  boolToInt64(l.IsActive),
		IsDefault: boolToInt64(l.IsDefault),
		IsSpecial: boolToInt64(l.IsSpecial),
	})
	if err != nil {
		return err
	}

	if err := q.DeletePriceListItemsForList(ctx, list.ID); err != nil {
		return err
	}
	for _, item := range l.Items {
		// An item for a variant this till has never seen is skipped, not
		// fatal: the product feed and the list feed advance independently, so
		// a brand new product can be priced centrally a moment before its
		// product row arrives here. The next pull carries the list again.
		if item.ProductVariantUID == "" {
			continue
		}
		if err := q.InsertPriceListItem(ctx, db.InsertPriceListItemParams{
			PriceListID: list.ID,
			ProductUid:  item.ProductVariantUID,
			Price:       item.Price,
		}); err != nil {
			return err
		}
	}
	return nil
}

func applySyncedDiscountList(ctx context.Context, q *db.Queries, l syncedDiscountList) error {
	if l.UID == "" {
		return fmt.Errorf("discount list %q has no uid", l.Name)
	}
	if !l.IsActive {
		return q.DeleteDiscountListByUID(ctx, sql.NullString{String: l.UID, Valid: true})
	}

	list, err := q.UpsertDiscountListByUID(ctx, db.UpsertDiscountListByUIDParams{
		Uid:       sql.NullString{String: l.UID, Valid: true},
		Name:      l.Name,
		IsActive:  boolToInt64(l.IsActive),
		IsDefault: boolToInt64(l.IsDefault),
		IsSpecial: boolToInt64(l.IsSpecial),
	})
	if err != nil {
		return err
	}

	// Deals first: they are shared between lists and kept by uid, so a deal
	// a list stops naming is simply left unused, never deleted out from
	// under another list.
	for _, d := range l.Deals {
		if d.UID == "" {
			continue
		}
		if err := q.UpsertDeal(ctx, db.UpsertDealParams{
			Uid: d.UID, Name: d.Name, Kind: d.Kind, BuyQty: d.BuyQty,
			GetQty: d.GetQty, Percent: d.Percent, BundlePrice: d.BundlePrice,
			FallbackPercent: d.FallbackPercent,
		}); err != nil {
			return err
		}
	}

	if err := q.DeleteDiscountListItemsForList(ctx, list.ID); err != nil {
		return err
	}
	for _, item := range l.Items {
		if item.ProductVariantUID == "" {
			continue
		}
		if err := q.InsertDiscountListItem(ctx, db.InsertDiscountListItemParams{
			DiscountListID: list.ID,
			ProductUid:     item.ProductVariantUID,
			Discount:       item.Discount,
			DealUid:        item.DealUID,
		}); err != nil {
			return err
		}
	}

	// Replaced whole like the plain items, so a removed option row goes too.
	if err := q.DeleteDiscountListOptionItemsForList(ctx, list.ID); err != nil {
		return err
	}
	for _, item := range l.OptionItems {
		if item.ProductVariantUID == "" || item.SaleOption == "" {
			continue
		}
		if err := q.InsertDiscountListOptionItem(ctx, db.InsertDiscountListOptionItemParams{
			DiscountListID: list.ID,
			ProductUid:     item.ProductVariantUID,
			SaleOption:     item.SaleOption,
			Discount:       item.Discount,
		}); err != nil {
			return err
		}
	}
	return nil
}
