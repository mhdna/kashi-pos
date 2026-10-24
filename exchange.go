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
		settings, err := a.store.GetSettings(a.ctx)
		if err != nil {
			return nil, err
		}
		if err := validateExchangeWindow(invoice.CreatedAt, time.Now(), exchangeWindowDuration(settings.ExchangeWindowHours)); err != nil {
			return nil, err
		}
	}
	lines, err := a.store.ListInvoiceProducts(a.ctx, invoice.ID)
	if err != nil {
		return nil, err
	}

	dealNames := map[string]string{}
	if deals, err := a.store.ListInvoiceDeals(a.ctx, invoice.ID); err == nil {
		for _, d := range deals {
			dealNames[d.DealUid] = d.Name
		}
	}
	items := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		key := keyOfLine(line)
		returned, err := a.store.GetReturnedQuantityForProductPrice(a.ctx, db.GetReturnedQuantityForProductPriceParams{
			SalesInvoiceID:   invoice.ID,
			ProductID:        line.ProductID,
			UnitPrice:        line.UnitPrice,
			Discount:         line.Discount,
			DealUid:          line.DealUid,
			UnitDealDiscount: key.unitDeal,
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
		lineSalespersonName := ""
		if line.SalespersonID > 0 {
			if person, personErr := a.store.GetSalespersonByID(a.ctx, line.SalespersonID); personErr == nil {
				lineSalespersonName = person.FullName
			}
		}
		items = append(items, map[string]any{
			"productId":       product.ID,
			"code":            product.Code,
			"barcode":         product.Barcode,
			"name":            product.Name,
			"color":           product.Color,
			"size":            product.Size,
			"price":           float64(line.UnitPrice) / 100,
			"discount":        line.Discount,
			"available":       available,
			"salespersonId":   line.SalespersonID,
			"salespersonName": lineSalespersonName,
			// Deal units (deal_exchange.go): a free one swaps for anything up
			// to its price; what was paid per unit is what a paid one credits.
			"deal":          dealNames[line.DealUid],
			"dealDiscount":  float64(key.unitDeal) / 100,
			"free":          key.unitDeal > 0 && key.unitDeal == line.UnitPrice,
			"paidUnitPrice": float64(line.LineTotal/max(1, line.Quantity)) / 100,
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

func exchangeWindowDuration(hours int64) time.Duration {
	return time.Duration(hours) * time.Hour
}

// validateExchangeWindow rejects an exchange once window has elapsed since
// createdAt. A zero or negative window (settings.exchange_window_hours <= 0)
// means no time limit is enforced.
func validateExchangeWindow(createdAt string, now time.Time, window time.Duration) error {
	if window <= 0 {
		return nil
	}
	invoiceTime, err := parseInvoiceTime(createdAt)
	if err != nil {
		return fmt.Errorf("invalid sales invoice date")
	}
	if now.UTC().After(invoiceTime.Add(window)) {
		return fmt.Errorf("exchanges are not allowed more than %s after the sale", formatExchangeWindow(window))
	}
	return nil
}

func formatExchangeWindow(window time.Duration) string {
	hours := int64(window.Hours())
	if hours%24 == 0 {
		days := hours / 24
		if days == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", days)
	}
	if hours == 1 {
		return "1 hour"
	}
	return fmt.Sprintf("%d hours", hours)
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
