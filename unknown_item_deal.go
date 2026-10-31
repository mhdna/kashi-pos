package main

import (
	"database/sql"
	"fmt"
)

// An "Unknown item" has no discount list row, so no list puts it on a deal.
// It joins the shop's "Buy 2 get 1" pool all the same, pooled with every
// other unit on that deal, and the salesperson picks what its units get
// outside a set -- the same two fallbacks kashi gives the shop's own rows
// (000216: the outlet codes' 30%, everything else's 20%) -- or takes it out
// of the deal. One of the three, always; the deal_toggle setting doesn't
// apply, since the line has no list's deal to keep.

const (
	unknownDeal30   = "b2g1_30"
	unknownDeal20   = "b2g1_20"
	unknownDealNone = "none"

	// What an unknown line gets until the salesperson changes it.
	unknownDealDefault = unknownDeal20
)

// unknownDealFallback is what a choice gives the line's units outside a set;
// ok is false for "none". An empty choice is the default.
func unknownDealFallback(choice string) (sql.NullInt64, bool, error) {
	if choice == "" {
		choice = unknownDealDefault
	}
	switch choice {
	case unknownDeal30:
		return sql.NullInt64{Int64: 30, Valid: true}, true, nil
	case unknownDeal20:
		return sql.NullInt64{Int64: 20, Valid: true}, true, nil
	case unknownDealNone:
		return sql.NullInt64{}, false, nil
	}
	return sql.NullInt64{}, false, fmt.Errorf("unknown item deal %q is not one of b2g1_30, b2g1_20, none", choice)
}

// isShopBuy2Get1 is a deal that gives the third unit of every three free.
func isShopBuy2Get1(t dealTerms) bool {
	return t.Kind == dealBuyGet && t.BuyQty == 2 && t.GetQty == 1 && t.Percent >= 100
}

// pickUnknownItemDeal is the "Buy 2 get 1" deal unknown items join: the one
// the most products are on, so they pool with the shop's items. Ties go to
// the lowest uid, so the pick never depends on map order. "" when the till
// knows no such deal.
func pickUnknownItemDeal(deals map[string]dealTerms, discounts map[string]activeDiscount) string {
	rows := map[string]int{}
	for _, d := range discounts {
		if terms, ok := deals[d.dealUID]; ok && isShopBuy2Get1(terms) {
			rows[d.dealUID]++
		}
	}
	best := ""
	for uid, terms := range deals {
		if !isShopBuy2Get1(terms) {
			continue
		}
		if best == "" || rows[uid] > rows[best] || (rows[uid] == rows[best] && uid < best) {
			best = uid
		}
	}
	return best
}

// hasUnitDiscount says whether the cashier typed the line a discount, on any
// unit.
func hasUnitDiscount(item salesInvoiceItem, discount int64) bool {
	if len(item.UnitDiscounts) == 0 {
		return discount > 0
	}
	for _, d := range item.UnitDiscounts {
		if d > 0 {
			return true
		}
	}
	return false
}
