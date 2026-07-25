package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

type exchangeLine struct {
	Code string `json:"code"`
	Qty  int64  `json:"qty"`
}

// GetExchangeInvoice loads only sales invoices that can be used as the source
// of an exchange, together with the remaining quantity of every line.
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
	products, err := a.store.ListInvoiceProducts(a.ctx, invoice.ID)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(products))
	for _, line := range products {
		returned, err := a.store.GetReturnedQuantityForProduct(a.ctx, db.GetReturnedQuantityForProductParams{
			SalesInvoiceID: invoice.ID,
			ProductID:      line.ProductID,
		})
		if err != nil {
			return nil, err
		}
		available := line.Quantity - asInt64(returned)
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
	return map[string]any{
		"id":          invoice.ID,
		"invoiceCode": invoice.InvoiceCode,
		"items":       items,
	}, nil
}

// CheckoutExchangeInvoice records an exchange as a return invoice. The
// returned line is negative and the replacement line is positive; therefore
// the invoice total and payment are only the difference owed by the customer.
func (a *App) CheckoutExchangeInvoice(itemsJSON, paymentsJSON string, originalInvoiceID, clientID int64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var lines []exchangeLine
	if err := json.Unmarshal([]byte(itemsJSON), &lines); err != nil || len(lines) != 2 {
		return nil, fmt.Errorf("an exchange must contain one returned and one replacement item")
	}
	var returned, replacement exchangeLine
	for _, line := range lines {
		if line.Code == "" || (line.Qty != -1 && line.Qty != 1) {
			return nil, fmt.Errorf("invalid exchange item")
		}
		if line.Qty < 0 {
			returned = line
		} else {
			replacement = line
		}
	}
	if returned.Code == "" || replacement.Code == "" || returned.Code == replacement.Code {
		return nil, fmt.Errorf("choose a different replacement product")
	}

	var payments []checkoutPayment
	var err error
	if strings.TrimSpace(paymentsJSON) != "" && paymentsJSON != "[]" {
		payments, err = parsePayments(paymentsJSON)
		if err != nil {
			return nil, err
		}
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}
	if settings.TaxRate < 0 || settings.TaxRate > 1 {
		return nil, fmt.Errorf("invalid tax rate")
	}
	if clientID <= 0 {
		original, err := a.store.GetInvoiceByID(a.ctx, originalInvoiceID)
		if err != nil {
			return nil, fmt.Errorf("source invoice: %w", err)
		}
		clientID = original.ClientID
	}
	shift, err := a.store.GetOpenShift(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("no open shift: %w", err)
	}
	inventoryID, err := a.salesInventoryID(settings.SalesInventoryID)
	if err != nil {
		return nil, err
	}
	roundingMode, err := normalizeRoundingMode(settings.RoundingMode)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	err = a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		// The sales-invoice join prevents exchanges from being made against
		// another exchange or an unrelated invoice.
		source, err := q.GetSalesInvoiceByID(a.ctx, originalInvoiceID)
		if err != nil {
			return fmt.Errorf("source invoice: %w", err)
		}
		returnedProduct, err := q.GetProductByCode(a.ctx, returned.Code)
		if err != nil {
			return fmt.Errorf("returned product: %w", err)
		}
		replacementProduct, err := q.GetProductByCode(a.ctx, replacement.Code)
		if err != nil || replacementProduct.IsActive == 0 {
			return fmt.Errorf("replacement product not found or inactive")
		}
		originalLine, err := q.GetInvoiceProduct(a.ctx, db.GetInvoiceProductParams{InvoiceID: source.ID, ProductID: returnedProduct.ID})
		if err != nil {
			return fmt.Errorf("returned product was not sold on invoice")
		}
		returnedQty, err := q.GetReturnedQuantityForProduct(a.ctx, db.GetReturnedQuantityForProductParams{SalesInvoiceID: source.ID, ProductID: returnedProduct.ID})
		if err != nil || asInt64(returnedQty)+1 > originalLine.Quantity {
			return fmt.Errorf("item has already been returned")
		}
		returnedCents := originalLine.UnitPrice - roundRatio(originalLine.UnitPrice*originalLine.Discount, 100, roundingMode)
		replacementCents := replacementProduct.Price - roundRatio(replacementProduct.Price*replacementProduct.Discount, 100, roundingMode)
		if replacementCents < returnedCents {
			return fmt.Errorf("replacement must be the same price or more expensive")
		}
		netDifference := replacementCents - returnedCents
		taxCents, err := roundFloatToCents(float64(netDifference)/100*settings.TaxRate, roundingMode)
		if err != nil {
			return err
		}
		total := netDifference + taxCents
		if err := validatePayments(a.ctx, q, payments, total); err != nil {
			return err
		}
		var cashbox db.CashboxAccount
		if len(payments) > 0 {
			cashbox, err = q.GetCashboxAccountByID(a.ctx, payments[0].AccountID)
			if err != nil {
				return fmt.Errorf("payment account: %w", err)
			}
		} else {
			accounts, listErr := q.ListCashboxAccounts(a.ctx)
			if listErr != nil || len(accounts) == 0 {
				return fmt.Errorf("no payment account configured")
			}
			cashbox = accounts[0]
		}
		index, err := q.IncrementInvoicesIndex(a.ctx, db.IncrementInvoicesIndexParams{Year: int64(time.Now().Year()), CashboxID: cashbox.ID, Type: "return"})
		if err != nil {
			return err
		}
		invoiceCode := fmt.Sprintf("EX-%s-%d-%05d", cashbox.Name, time.Now().Year(), index)
		invoice, err := q.CreateInvoice(a.ctx, db.CreateInvoiceParams{CashboxID: cashbox.ID, ShiftID: shift.ID, InvoiceCode: invoiceCode, InvoiceIndex: index, Year: int64(time.Now().Year()), ClientID: clientID, InventoryID: inventoryID, Discount: 0, Subtotal: total, DiscountedTotal: total, GrandTotal: total})
		if err != nil {
			return err
		}
		if err := q.CreateReturnInvoice(a.ctx, db.CreateReturnInvoiceParams{InvoiceID: invoice.ID, SalesInvoiceID: source.ID}); err != nil {
			return err
		}
		if err := q.CreateInvoiceProduct(a.ctx, db.CreateInvoiceProductParams{InvoiceID: invoice.ID, ProductID: returnedProduct.ID, UnitPrice: originalLine.UnitPrice, LineTotal: -returnedCents, Discount: originalLine.Discount, Quantity: -1}); err != nil {
			return err
		}
		if err := q.CreateInvoiceProduct(a.ctx, db.CreateInvoiceProductParams{InvoiceID: invoice.ID, ProductID: replacementProduct.ID, UnitPrice: replacementProduct.Price, LineTotal: replacementCents, Discount: replacementProduct.Discount, Quantity: 1}); err != nil {
			return err
		}
		if err := q.AddInventoryProductQuantity(a.ctx, db.AddInventoryProductQuantityParams{InventoryID: inventoryID, ProductID: returnedProduct.ID, Quantity: 1}); err != nil {
			return err
		}
		updated, err := q.DecreaseInventoryProductQuantity(a.ctx, db.DecreaseInventoryProductQuantityParams{Quantity: 1, InventoryID: inventoryID, ProductID: replacementProduct.ID, Quantity_2: 1})
		if err != nil {
			return err
		}
		if count, err := updated.RowsAffected(); err != nil || count != 1 {
			return fmt.Errorf("insufficient stock for replacement product")
		}
		for _, payment := range payments {
			if err := q.CreateInvoicePaymentEntry(a.ctx, db.CreateInvoicePaymentEntryParams{InvoiceID: invoice.ID, CashboxID: payment.AccountID, Amount: payment.AmountCents, EntryType: payment.EntryType}); err != nil {
				return err
			}
			if payment.AmountCents > 0 {
				if err := q.CreateInvoicePayment(a.ctx, db.CreateInvoicePaymentParams{InvoiceID: invoice.ID, CashboxID: payment.AccountID, Amount: payment.AmountCents}); err != nil {
					return err
				}
				if _, err := q.AddCashboxAccountBalance(a.ctx, db.AddCashboxAccountBalanceParams{AccountID: payment.AccountID, ShiftID: shift.ID, Balance: payment.AmountCents}); err != nil {
					return err
				}
			}
		}
		result = map[string]any{"invoiceCode": invoiceCode, "difference": float64(total) / 100}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func asInt64(value any) int64 {
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
