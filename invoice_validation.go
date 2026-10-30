package main

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	db "kashi-pos/db/sqlc"
)

type pricedInvoiceProduct struct {
	product        db.Product
	qty            int64
	unitPrice      int64
	discount       int64
	lineTotal      int64
	salespersonID  int64
	note           string
	priceListID    int64
	discountListID int64
	saleOption     string
	// The option as the cart sent it, kept even when it no longer counts,
	// so the quote's line matches the cart's (invoice_quote.go).
	cartSaleOption string
	// The deal the line is on, and what it took off the whole line (cents);
	// lineTotal already has it off. See deals.go.
	dealUID      string
	dealName     string
	dealLabel    string
	dealDiscount int64
	// What the line gave up by having its discount (NoDiscount) or its deal
	// (NoDeal) turned off: the percent, or the deal's short label.
	offDiscount  int64
	offDealLabel string
	// The line's share of the sale's invoice discount (invoice_discounts.go);
	// lineTotal already has it off.
	invoiceDiscount int64
}

type unitPricing struct {
	price    int64
	discount int64
}

// salesInventoryID picks the inventory to stamp on invoices.inventory_id.
//
// Nothing about a sale depends on this beyond the FK: stock is neither
// checked nor decremented at checkout (see the disabled block in
// salesInvoiceTx) and validateAndPriceItems ignores the id entirely. So an
// unset setting, an unknown id, or an empty inventory must never stop a
// cashier from selling - the worst case is an invoice labelled with the
// wrong inventory, which the separate stock-reconciliation workflow can
// correct. Only a database with no inventories at all is unrecoverable
// here, because the FK has nothing to point at.
//
// If stock deduction is ever re-enabled in salesInvoiceTx, revisit this:
// silently guessing an inventory is fine for a label, but not for deciding
// which shelf to take units off.
func (a *App) salesInventoryID(configuredID int64) (int64, error) {
	inventories, err := a.store.ListInventories(a.ctx)
	if err != nil {
		return 0, fmt.Errorf("list inventories: %w", err)
	}
	if len(inventories) == 0 {
		return 0, fmt.Errorf("no inventory exists to record the sale against")
	}
	// Configured and real - the normal path.
	for _, inventory := range inventories {
		if inventory.ID == configuredID {
			return inventory.ID, nil
		}
	}
	if len(inventories) == 1 {
		return inventories[0].ID, nil
	}
	// Unset, or pointing at an inventory that no longer exists. Prefer the
	// one this branch has actually been selling from; a second inventory can
	// appear at any time as a side effect of a stock import (admin_server.go
	// does INSERT INTO inventories ... ON CONFLICT DO NOTHING).
	lastUsed, err := a.store.GetLastInvoiceInventoryID(a.ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("last sales inventory: %w", err)
	}
	if err == nil {
		for _, inventory := range inventories {
			if inventory.ID == lastUsed {
				return inventory.ID, nil
			}
		}
	}
	// No history to learn from. ListInventories is ordered by id, so this is
	// the oldest - the "Default" seeded at first run.
	return inventories[0].ID, nil
}

func (a *App) resolveClientID(id int64) (int64, error) {
	if id > 0 {
		if _, err := a.store.GetClientByID(a.ctx, id); err != nil {
			return 0, fmt.Errorf("client: %w", err)
		}
		return id, nil
	}
	walkIn, err := a.store.GetClientByPhone(a.ctx, "00000000")
	if err != nil {
		return 0, fmt.Errorf("walk-in client is not configured")
	}
	return walkIn.ID, nil
}

func (a *App) validateAndPriceItems(q *db.Queries, inventoryID int64, items []salesInvoiceItem, roundingMode string, customPricingEnabled bool, listPricing activePricing, perUnitPricingEnabled ...bool) ([]pricedInvoiceProduct, int64, int64, []invoiceItem, error) {
	products := make([]pricedInvoiceProduct, 0, len(items))
	var subtotal int64
	discountToggleChecked, dealToggleChecked := false, false
	for _, item := range items {
		if item.NoDiscount && !discountToggleChecked {
			if !a.flagEnabled(a.ctx, discountToggleFlag) {
				return nil, 0, 0, nil, fmt.Errorf("turning a discount off is disabled")
			}
			discountToggleChecked = true
		}
		if item.NoDeal && !dealToggleChecked {
			if !a.flagEnabled(a.ctx, dealToggleFlag) {
				return nil, 0, 0, nil, fmt.Errorf("turning a deal off is disabled")
			}
			dealToggleChecked = true
		}
		if item.SalespersonID > 0 {
			if _, err := q.GetSalespersonByID(a.ctx, item.SalespersonID); err != nil {
				return nil, 0, 0, nil, fmt.Errorf("salesperson for item %s: %w", item.Code, err)
			}
		}
		var product db.Product
		var err error
		if strings.TrimSpace(item.Barcode) != "" {
			product, err = q.GetProductByBarcode(a.ctx, strings.TrimSpace(item.Barcode))
		} else {
			product, err = q.GetProductByCode(a.ctx, item.Code)
		}
		if err != nil {
			return nil, 0, 0, nil, fmt.Errorf("product %s: %w", item.Code, err)
		}
		// The built-in "Unknown item" placeholder is never really off: earlier
		// versions' catalog check could switch it off by mistake.
		if product.IsActive == 0 && product.Barcode != defaultUnknownBarcode {
			return nil, 0, 0, nil, fmt.Errorf("product %s is inactive", item.Code)
		}
		// The product row's price/discount columns are frozen (000097): the
		// sellable price comes from the active lists, keyed by the shared uid.
		// They're zeroed first so a stale row value can never be charged -- a
		// product in no list ends up at 0 and is rejected by
		// priceInvoiceProduct below, which is what "kashi never priced this,
		// so it can't be sold" looks like at the till.
		product.Price = 0
		product.Discount = 0
		var priceListID, discountListID int64
		dealUID := ""
		if listed, ok := listPricing.priceFor(product.Uid.String); ok {
			product.Price = listed.cents
			priceListID = listed.listID
		}
		// The option only counts if the product still asks it: one left over
		// in a cart from before kashi took the product's options away is sold
		// at the plain discount, rather than failing the sale.
		saleOption := ""
		if slices.Contains(listPricing.saleOptions[product.Uid.String], item.SaleOption) {
			saleOption = item.SaleOption
		}
		if listed, ok := listPricing.discountForOption(product.Uid.String, saleOption); ok {
			product.Discount = listed.percent
			discountListID = listed.listID
			dealUID = listed.dealUID
		}

		// A manual override still wins over the list price -- the cashier is
		// deliberately overriding a price they can see, not inventing one for
		// a product central never priced. The line stops being attributed to
		// the list once it no longer carries the list's value.
		// The "Unknown item" placeholder has no list price: whatever the
		// cashier typed is its price, whatever the custom-price settings say
		// (kashi's picture session is what vouches for the sale instead).
		isUnknown := product.Barcode == a.unknownBarcode()
		if item.CustomPriceCents != nil {
			if !customPricingEnabled && !isUnknown {
				return nil, 0, 0, nil, fmt.Errorf("custom item pricing is disabled")
			}
			product.Price = *item.CustomPriceCents
			priceListID = 0
		}
		if len(item.UnitPricesCents) > 0 {
			priceListID = 0
		}
		if len(item.UnitDiscounts) > 0 {
			discountListID = 0
		}
		// A line priced by hand stays out of its deal: the cashier set what it
		// costs, and a deal never stacks on another saving. So does the
		// "Unknown item" placeholder, which is in no list.
		if isUnknown || item.CustomPriceCents != nil || len(item.UnitPricesCents) > 0 || len(item.UnitDiscounts) > 0 {
			dealUID = ""
		}
		// A discount or deal turned off at the till: the line goes without
		// it, and the cart is told what it gave up so it can offer it back.
		var offDiscount int64
		offDealLabel := ""
		if item.NoDiscount && !isUnknown && product.Discount > 0 {
			offDiscount = product.Discount
			product.Discount = 0
			discountListID = 0
		}
		if item.NoDeal && !isUnknown && dealUID != "" {
			if terms, ok := listPricing.deals[dealUID]; ok {
				offDealLabel = terms.label()
			}
			dealUID = ""
		}
		priceQuantities := map[unitPricing]int64{
			{price: product.Price, discount: product.Discount}: int64(item.Qty),
		}
		if len(item.UnitPricesCents) > 0 || len(item.UnitDiscounts) > 0 {
			if !isUnknown && (len(perUnitPricingEnabled) == 0 || !perUnitPricingEnabled[0]) {
				return nil, 0, 0, nil, fmt.Errorf("per-unit item pricing is disabled")
			}
			priceQuantities = make(map[unitPricing]int64)
			for index := int64(0); index < int64(item.Qty); index++ {
				price := product.Price
				if len(item.UnitPricesCents) > 0 {
					price = item.UnitPricesCents[index]
				}
				discount := product.Discount
				if len(item.UnitDiscounts) > 0 {
					discount = item.UnitDiscounts[index]
				}
				priceQuantities[unitPricing{price: price, discount: discount}]++
			}
		}
		prices := make([]unitPricing, 0, len(priceQuantities))
		for pricing := range priceQuantities {
			prices = append(prices, pricing)
		}
		sort.Slice(prices, func(i, j int) bool {
			if prices[i].price == prices[j].price {
				return prices[i].discount < prices[j].discount
			}
			return prices[i].price < prices[j].price
		})
		for _, pricing := range prices {
			qty := priceQuantities[pricing]
			pricedProduct := product
			pricedProduct.Price = pricing.price
			pricedProduct.Discount = pricing.discount
			priced, err := priceInvoiceProduct(pricedProduct, qty, roundingMode)
			if err != nil {
				return nil, 0, 0, nil, err
			}
			priced.salespersonID = item.SalespersonID
			priced.note = strings.TrimSpace(item.Note)
			priced.saleOption = saleOption
			priced.cartSaleOption = item.SaleOption
			priced.dealUID = dealUID
			priced.offDiscount = offDiscount
			priced.offDealLabel = offDealLabel
			if pricing.price == product.Price {
				priced.priceListID = priceListID
			}
			if pricing.discount == product.Discount {
				priced.discountListID = discountListID
			}
			gross := pricing.price * qty
			if subtotal > math.MaxInt64-gross {
				return nil, 0, 0, nil, fmt.Errorf("invoice total is too large")
			}
			subtotal += gross
			products = append(products, priced)
		}
	}
	// Deals price lines together, so they come once every line is known; a
	// deal only ever takes off, so the totals can't grow past the checks above.
	products = applyDeals(products, listPricing.deals, roundingMode)
	var discountedSubtotal int64
	receiptItems := make([]invoiceItem, 0, len(products))
	for _, p := range products {
		discountedSubtotal += p.lineTotal
		receiptItems = append(receiptItems, receiptItemFor(p))
	}
	return products, subtotal, discountedSubtotal, receiptItems, nil
}

// receiptItemFor is a priced line as the receipt prints it. A deal line
// carries its saving as an amount (a bundle's share is rarely a whole
// percent) and says so when a unit is free: "Shirt (FREE)".
func receiptItemFor(p pricedInvoiceProduct) invoiceItem {
	item := invoiceItem{
		Name: nameWithSaleOption(p.product.Name, p.saleOption), Code: p.product.Code, Barcode: p.product.Barcode,
		Category: p.product.Description, Color: p.product.Color, Size: p.product.Size,
		Price: float64(p.unitPrice) / 100, Discount: float64(p.discount), Qty: float64(p.qty),
	}
	if p.dealDiscount > 0 {
		item.Deal = p.dealName
		item.Free = p.isFree()
		item.DealDiscount = float64(p.dealDiscount) / 100
		item.Discount = float64(p.dealDiscount) * 100 / float64(p.unitPrice*p.qty)
		item.Name = nameWithDeal(item.Name, item.Free)
	}
	return item
}

// nameWithDeal marks a free deal unit on the receipt and the till's lists.
// A unit that only got a saving shows it in the discount column, and the
// deal's long name would only wrap the line.
func nameWithDeal(name string, free bool) string {
	if free {
		return name + " (FREE)"
	}
	return name
}

func priceInvoiceProduct(product db.Product, qty int64, roundingMode string) (pricedInvoiceProduct, error) {
	if qty <= 0 || product.Price <= 0 || product.Discount < 0 || product.Discount > 100 {
		return pricedInvoiceProduct{}, fmt.Errorf("invalid pricing for product %s", product.Code)
	}
	if qty > math.MaxInt64/product.Price {
		return pricedInvoiceProduct{}, fmt.Errorf("line total is too large for product %s", product.Code)
	}
	gross := product.Price * qty
	if product.Discount > 0 && gross > math.MaxInt64/product.Discount {
		return pricedInvoiceProduct{}, fmt.Errorf("line discount is too large for product %s", product.Code)
	}
	lineDiscount := roundRatio(gross*product.Discount, 100, roundingMode)
	return pricedInvoiceProduct{
		product:   product,
		qty:       qty,
		unitPrice: product.Price,
		discount:  product.Discount,
		lineTotal: gross - lineDiscount,
	}, nil
}

// nameWithSaleOption is the line's name on the receipt: "Lady Jacket
// (Summer)", so the receipt says which stock was sold.
func nameWithSaleOption(name, saleOption string) string {
	if saleOption == "" {
		return name
	}
	return name + " (" + saleOption + ")"
}
