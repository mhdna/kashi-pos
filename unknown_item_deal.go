package main

import (
	"database/sql"
	"fmt"
)

// An "Unknown item" has no discount list row, so no list puts it on a deal.
// It joins the deal the shop's own products are on all the same (pooled with
// every other unit on that deal, whatever the deal's numbers), and the
// salesperson picks what its units get outside a set -- the same two fallbacks
// Central gives the shop's own rows (000216: the outlet codes' 30%, everything
// else's 20%) -- or takes it out of the deal. One of the three, always; the
// deal_toggle setting doesn't apply, since the line has no list's deal to keep.
// The choices keep their b2g1_ names whatever the deal: they are stored on carts.

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

// isFreeUnitsDeal is a deal that gives units free ("Buy 2 get 1", "Buy 1 get 1"),
// as opposed to "the 3rd 50% off" or a bundle price.
func isFreeUnitsDeal(t dealTerms) bool {
	return t.Kind == dealBuyGet && t.GetQty > 0 && t.Percent >= 100
}

// pickUnknownItemDeal is the deal unknown items join: the free-units deal the
// most products are on in the lists in force, so they pool with the shop's items
// and follow the branch to another list. A deal no list names is left out: the
// till keeps deals after their list has gone. Ties go to the lowest uid, so the
// pick never depends on map order. "" when no product is on such a deal.
func pickUnknownItemDeal(deals map[string]dealTerms, discounts map[string]activeDiscount) string {
	rows := map[string]int{}
	for _, d := range discounts {
		if terms, ok := deals[d.dealUID]; ok && isFreeUnitsDeal(terms) {
			rows[d.dealUID]++
		}
	}
	best := ""
	for uid, count := range rows {
		if best == "" || count > rows[best] || (count == rows[best] && uid < best) {
			best = uid
		}
	}
	return best
}

// unknownDealOffer names the deal for the picker: its short label with no
// fallback ("B1G1"), since the salesperson is picking that, and its name.
func unknownDealOffer(pricing activePricing) (label, name string) {
	terms, ok := pricing.deals[pricing.unknownDealUID]
	if !ok {
		return "", ""
	}
	terms.FallbackPercent = 0
	return terms.label(), terms.Name
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
