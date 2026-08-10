package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

// GetExchangeInvoice loads a sales invoice and only the quantities that remain
// eligible for return.
func (a *App) GetExchangeInvoice(code string) (map[string]any, error) {
	return a.getExchangeInvoice(code, true)
}

func (a *App) getExchangeInvoice(code string, enforceWindow bool) (map[string]any, error) {
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
	if enforceWindow {
		if err := validateExchangeWindow(invoice.CreatedAt, time.Now()); err != nil {
			return nil, err
		}
	}
	lines, err := a.store.ListInvoiceProducts(a.ctx, invoice.ID)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		returned, err := a.store.GetReturnedQuantityForProductPrice(a.ctx, db.GetReturnedQuantityForProductPriceParams{
			SalesInvoiceID: invoice.ID,
			ProductID:      line.ProductID,
			UnitPrice:      line.UnitPrice,
			Discount:       line.Discount,
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
			"productId": product.ID,
			"code":      product.Code,
			"barcode":   product.Barcode,
			"name":      product.Name,
			"color":     product.Color,
			"size":      product.Size,
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

const exchangeWindow = 3 * 24 * time.Hour

func validateExchangeWindow(createdAt string, now time.Time) error {
	invoiceTime, err := parseInvoiceTime(createdAt)
	if err != nil {
		return fmt.Errorf("invalid sales invoice date")
	}
	if now.UTC().After(invoiceTime.Add(exchangeWindow)) {
		return fmt.Errorf("exchanges are not allowed more than 3 days after the sale")
	}
	return nil
}

func parseInvoiceTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339Nano} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported invoice date %q", value)
}

func databaseInt64(value any) int64 {
	switch n := value.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
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
