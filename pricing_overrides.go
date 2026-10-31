package main

import (
	"context"
	"database/sql"
	"sync"

	db "kashi-pos/db/sqlc"
)

// Since 000097 a price is not a property of a product row -- it is an entry
// in a price list that kashi dictates and the sync mirrors down. products.price
// and products.discount still exist but are frozen: nothing reads them, and a
// product's sellable price comes from here or from nowhere.
//
// "From nowhere" is a real, intended state. A product in no active list has no
// price, and priceInvoiceProduct rejects a zero price, so the sale fails at the
// till rather than ringing something up for free. That is the whole point of
// the change: central decides what is sellable.

type activePrice struct {
	cents     int64
	listID    int64
	listName  string
	isSpecial bool
}

type activeDiscount struct {
	percent   int64
	listID    int64
	listName  string
	isSpecial bool
	// The deal the row puts the product on (000128); percent is 0 then.
	dealUID string
	// What the product gets outside a set on that deal, in place of the
	// deal's fallback (000133); unset for the deal's own.
	dealFallback sql.NullInt64
}

// activePricing is every price and discount currently in force, keyed by
// product uid. It is loaded once per pricing pass (checkout, a product
// listing/search page, a cart add) rather than once per product, so pricing N
// products costs 2 queries, not 2N.
type activePricing struct {
	prices    map[string]activePrice
	discounts map[string]activeDiscount
	// optionDiscounts[uid][option] is the discount for a line sold as that
	// sale option (000126); see discountForOption.
	optionDiscounts map[string]map[string]activeDiscount
	// saleOptions[uid] is what the till asks when the product is sold, in
	// display order; absent for an ordinary product.
	saleOptions map[string][]string
	// deals[uid] is a deal's terms, for the rows naming it (deals.go).
	deals map[string]dealTerms
	// The "Buy 2 get 1" deal unknown items join (unknown_item_deal.go); ""
	// when there's none.
	unknownDealUID string
}

// priceFor / discountFor report whether central said anything about a product
// at all. The bool is the part that matters: false means unsellable, and a
// caller must never collapse it to a zero price.
func (p activePricing) priceFor(productUID string) (activePrice, bool) {
	v, ok := p.prices[productUID]
	return v, ok
}

func (p activePricing) discountFor(productUID string) (activeDiscount, bool) {
	v, ok := p.discounts[productUID]
	return v, ok
}

// discountForOption is the discount for a line sold as saleOption: the
// option's own row when a list has one, otherwise the product's plain
// discount. An empty saleOption is the plain discount.
func (p activePricing) discountForOption(productUID, saleOption string) (activeDiscount, bool) {
	if saleOption != "" {
		if v, ok := p.optionDiscounts[productUID][saleOption]; ok {
			return v, true
		}
	}
	return p.discountFor(productUID)
}

// pricingCache keeps the last loaded activePricing, so a quote doesn't re-read
// every price and discount (~13k rows, most of a quote's time) on each scan.
// It's keyed on pricing_version (000127), which triggers move on any write
// to what prices a sale, so a reuse is only ever of exactly what the database
// holds now. The token tells databases apart (tests, a branch switch).
// Callers only read the maps, never write them, so sharing them is safe.
var pricingCache struct {
	sync.Mutex
	token   string
	version int64
	pricing *activePricing
}

// resetPricingCache drops the cached pricing; called when the till opens a
// database, since two branches' databases can carry the same version.
func resetPricingCache() {
	pricingCache.Lock()
	pricingCache.pricing = nil
	pricingCache.Unlock()
}

// loadActivePricing is the pricing in force, from the cache when nothing that
// prices a sale changed since it was loaded. q may be a transaction's: the
// version is read through it, so a checkout prices from what its own
// transaction sees.
func loadActivePricing(ctx context.Context, q *db.Queries) (activePricing, error) {
	current, err := q.GetPricingVersion(ctx)
	if err != nil {
		return activePricing{}, err
	}
	pricingCache.Lock()
	if cached := pricingCache.pricing; cached != nil &&
		pricingCache.token == current.Token && pricingCache.version == current.Version {
		pricingCache.Unlock()
		return *cached, nil
	}
	pricingCache.Unlock()

	pricing, err := readActivePricing(ctx, q)
	if err != nil {
		return pricing, err
	}
	pricingCache.Lock()
	pricingCache.token, pricingCache.version, pricingCache.pricing = current.Token, current.Version, &pricing
	pricingCache.Unlock()
	return pricing, nil
}

func readActivePricing(ctx context.Context, q *db.Queries) (activePricing, error) {
	pricing := activePricing{
		prices:          map[string]activePrice{},
		discounts:       map[string]activeDiscount{},
		optionDiscounts: map[string]map[string]activeDiscount{},
		saleOptions:     map[string][]string{},
		deals:           map[string]dealTerms{},
	}

	// Both queries order is_default DESC, so a default-list row is written
	// first and a branch-list row for the same product overwrites it -- the
	// branch's own list wins, and a product only the default mentions keeps
	// the default price. See ListActivePriceListItems.
	priceRows, err := q.ListActivePriceListItems(ctx)
	if err != nil {
		return pricing, err
	}
	for _, row := range priceRows {
		pricing.prices[row.ProductUid] = activePrice{
			cents: row.Price, listID: row.ListID, listName: row.ListName,
			isSpecial: row.IsSpecial != 0,
		}
	}

	discountRows, err := q.ListActiveDiscountListItems(ctx)
	if err != nil {
		return pricing, err
	}
	for _, row := range discountRows {
		pricing.discounts[row.ProductUid] = activeDiscount{
			percent: row.Discount, listID: row.ListID, listName: row.ListName,
			dealUID: row.DealUid, dealFallback: row.DealFallbackPercent,
		}
	}

	// Same order, same rule: a branch list's option row overwrites the
	// default list's for the same product and option.
	optionRows, err := q.ListActiveDiscountListOptionItems(ctx)
	if err != nil {
		return pricing, err
	}
	for _, row := range optionRows {
		byOption := pricing.optionDiscounts[row.ProductUid]
		if byOption == nil {
			byOption = map[string]activeDiscount{}
			pricing.optionDiscounts[row.ProductUid] = byOption
		}
		byOption[row.SaleOption] = activeDiscount{
			percent: row.Discount, listID: row.ListID, listName: row.ListName,
		}
	}

	saleOptionRows, err := q.ListAllProductSaleOptions(ctx)
	if err != nil {
		return pricing, err
	}
	for _, row := range saleOptionRows {
		pricing.saleOptions[row.ProductUid.String] = append(pricing.saleOptions[row.ProductUid.String], row.Name)
	}

	dealRows, err := q.ListDeals(ctx)
	if err != nil {
		return pricing, err
	}
	for _, row := range dealRows {
		pricing.deals[row.Uid] = dealTermsFromRow(row)
	}
	pricing.unknownDealUID = pickUnknownItemDeal(pricing.deals, pricing.discounts)
	return pricing, nil
}
