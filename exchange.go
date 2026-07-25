package main

import (
	"database/sql"
	"fmt"
	"strings"

	db "kashi-pos/db/sqlc"
)

// GetExchangeInvoice loads a sales invoice and only the quantities that remain
// eligible for return.
func (a *App) GetExchangeInvoice(code string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, fmt.Errorf("enter a sales invoice code")
	}

	invoice, err := a.store.GetSalesInvoiceByCode(a.ctx, code)
	if err != nil {
		return nil, fmt.Errorf("sales invoice not found")
	}
	lines, err := a.store.ListInvoiceProducts(a.ctx, invoice.ID)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		returned, err := a.store.GetReturnedQuantityForProduct(a.ctx, db.GetReturnedQuantityForProductParams{
			SalesInvoiceID: invoice.ID,
			ProductID:      line.ProductID,
		})
		if err != nil {
			return nil, err
		}
		available := line.Quantity - databaseInt64(returned)
		if available <= 0 {
			continue
		}
		product, err := a.store.GetProductByID(a.ctx, line.ProductID)
		if err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"code":      product.Code,
			"barcode":   product.Barcode,
			"name":      product.Name,
			"price":     float64(line.UnitPrice) / 100,
			"discount":  line.Discount,
			"available": available,
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("all items on this invoice have already been returned")
	}
	client, err := a.store.GetClientByID(a.ctx, invoice.ClientID)
	if err != nil {
		return nil, fmt.Errorf("invoice customer: %w", err)
	}
	return map[string]any{
		"id":          invoice.ID,
		"invoiceCode": invoice.InvoiceCode,
		"client":      clientResponse(client),
		"items":       items,
	}, nil
}

func databaseInt64(value any) int64 {
	switch n := value.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case []byte:
		var parsed int64
		_, _ = fmt.Sscan(string(n), &parsed)
		return parsed
	case sql.NullInt64:
		return n.Int64
	default:
		return 0
	}
}
