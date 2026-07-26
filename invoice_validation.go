package main

import (
	"fmt"
	"math"
	"strings"

	db "kashi-pos/db/sqlc"
)

type pricedInvoiceProduct struct {
	product   db.Product
	qty       int64
	unitPrice int64
	discount  int64
	lineTotal int64
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
	clients, err := a.store.ListClients(a.ctx)
	if err != nil {
		return 0, fmt.Errorf("list clients: %w", err)
	}
	for _, client := range clients {
		if client.Phone == "00000000" {
			return client.ID, nil
		}
	}
	return 0, fmt.Errorf("walk-in client is not configured")
}

func (a *App) validateAndPriceItems(q *db.Queries, inventoryID int64, items []salesInvoiceItem, roundingMode string) ([]pricedInvoiceProduct, int64, int64, []invoiceItem, error) {
	products := make([]pricedInvoiceProduct, 0, len(items))
	receiptItems := make([]invoiceItem, 0, len(items))
	var subtotal, discountedSubtotal int64
	for _, item := range items {
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
		qty := int64(item.Qty)
		priced, err := priceInvoiceProduct(product, qty, roundingMode)
		if err != nil {
			return nil, 0, 0, nil, err
		}
		gross := product.Price * qty
		lineTotal := priced.lineTotal
		if subtotal > math.MaxInt64-gross || discountedSubtotal > math.MaxInt64-lineTotal {
			return nil, 0, 0, nil, fmt.Errorf("invoice total is too large")
		}
		subtotal += gross
		discountedSubtotal += lineTotal
		products = append(products, priced)
		receiptItems = append(receiptItems, invoiceItem{Name: product.Name, Code: product.Code, Category: product.Description, Price: float64(product.Price) / 100, Discount: float64(product.Discount), Qty: float64(qty)})
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
