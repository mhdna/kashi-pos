package main

import (
	"fmt"
	"math"
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
}

type unitPricing struct {
	price    int64
	discount int64
}

func (a *App) salesInventoryID(configuredID int64) (int64, error) {
	if configuredID > 0 {
		if _, err := a.store.GetInventoryByID(a.ctx, configuredID); err != nil {
			return 0, fmt.Errorf("sales inventory: %w", err)
		}
		return configuredID, nil
	}
	inventories, err := a.store.ListInventories(a.ctx)
	if err != nil {
		return 0, fmt.Errorf("list inventories: %w", err)
	}
	if len(inventories) != 1 {
		return 0, fmt.Errorf("choose a sales inventory in settings")
	}
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

func (a *App) validateAndPriceItems(q *db.Queries, inventoryID int64, items []salesInvoiceItem, roundingMode string, customPricingEnabled bool, overrides activeOverrides, perUnitPricingEnabled ...bool) ([]pricedInvoiceProduct, int64, int64, []invoiceItem, error) {
	products := make([]pricedInvoiceProduct, 0, len(items))
	receiptItems := make([]invoiceItem, 0, len(items))
	var subtotal, discountedSubtotal int64
	for _, item := range items {
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
		if product.IsActive == 0 {
			return nil, 0, 0, nil, fmt.Errorf("product %s is inactive", item.Code)
		}
		if item.CustomPriceCents != nil {
			if !customPricingEnabled {
				return nil, 0, 0, nil, fmt.Errorf("custom item pricing is disabled")
			}
			product.Price = *item.CustomPriceCents
		}
		// A manual override (a whole-line custom price, or a per-unit
		// price/discount array) always wins over a price-list/discount-list
		// default - list overrides only fill in where the cashier hasn't
		// already specified something for that field.
		var priceListID int64
		if item.CustomPriceCents == nil && len(item.UnitPricesCents) == 0 {
			if override, ok := overrides.prices[product.Code]; ok {
				product.Price = override.cents
				priceListID = override.listID
			}
		}
		var discountListID int64
		if len(item.UnitDiscounts) == 0 {
			if override, ok := overrides.discounts[product.Code]; ok {
				product.Discount = override.percent
				discountListID = override.listID
			}
		}
		priceQuantities := map[unitPricing]int64{
			{price: product.Price, discount: product.Discount}: int64(item.Qty),
		}
		if len(item.UnitPricesCents) > 0 || len(item.UnitDiscounts) > 0 {
			if len(perUnitPricingEnabled) == 0 || !perUnitPricingEnabled[0] {
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
			if pricing.price == product.Price {
				priced.priceListID = priceListID
			}
			if pricing.discount == product.Discount {
				priced.discountListID = discountListID
			}
			gross := pricing.price * qty
			lineTotal := priced.lineTotal
			if subtotal > math.MaxInt64-gross || discountedSubtotal > math.MaxInt64-lineTotal {
				return nil, 0, 0, nil, fmt.Errorf("invoice total is too large")
			}
			subtotal += gross
			discountedSubtotal += lineTotal
			products = append(products, priced)
			receiptItems = append(receiptItems, invoiceItem{Name: product.Name, Code: product.Code, Barcode: product.Barcode, Category: product.Description, Color: product.Color, Size: product.Size, Price: float64(pricing.price) / 100, Discount: float64(pricing.discount), Qty: float64(qty)})
		}
	}
	return products, subtotal, discountedSubtotal, receiptItems, nil
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
