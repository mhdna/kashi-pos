package main

import (
	"fmt"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

// CheckoutExchangeInvoice records one returned line and one or more
// replacement lines. Only the positive difference is paid by the customer.
func (a *App) CheckoutExchangeInvoice(itemsJSON, paymentsJSON string, originalInvoiceID, _ int64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	request, err := parseExchangeRequest(itemsJSON)
	if err != nil {
		return nil, err
	}
	payments, err := optionalPayments(paymentsJSON)
	if err != nil {
		return nil, err
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}
	if settings.TaxRate < 0 || settings.TaxRate > 1 {
		return nil, fmt.Errorf("invalid tax rate")
	}
	roundingMode, err := normalizeRoundingMode(settings.RoundingMode)
	if err != nil {
		return nil, err
	}
	shift, err := a.store.GetOpenShift(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("no open shift: %w", err)
	}
	inventoryID, err := a.salesInventoryID(settings.SalesInventoryID)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	err = a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		source, err := q.GetSalesInvoiceByID(a.ctx, originalInvoiceID)
		if err != nil {
			return fmt.Errorf("source invoice: %w", err)
		}
		priced, err := a.validateAndPriceExchange(q, source.ID, request, roundingMode)
		if err != nil {
			return err
		}
		netDifference := priced.replacementTotal - priced.returnedTotal
		taxCents, err := roundFloatToCents(float64(netDifference)/100*settings.TaxRate, roundingMode)
		if err != nil {
			return err
		}
		total := netDifference + taxCents
		if err := validatePayments(a.ctx, q, payments, total); err != nil {
			return err
		}
		cashbox, err := exchangeCashbox(a, q, payments)
		if err != nil {
			return err
		}
		invoice, err := createExchangeInvoice(
			a, q, cashbox, shift, source, inventoryID, total, settings.BranchCode,
		)
		if err != nil {
			return err
		}
		if err := recordExchangeProducts(a, q, invoice.ID, inventoryID, priced); err != nil {
			return err
		}
		if err := recordExchangePayments(a, q, invoice.ID, shift.ID, payments); err != nil {
			return err
		}
		result = map[string]any{
			"invoiceCode": invoice.InvoiceCode,
			"difference":  float64(total) / 100,
		}
		return nil
	})
	return result, err
}

func optionalPayments(raw string) ([]checkoutPayment, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "[]" {
		return nil, nil
	}
	return parsePayments(raw)
}

func exchangeCashbox(a *App, q *db.Queries, payments []checkoutPayment) (db.CashboxAccount, error) {
	if len(payments) > 0 {
		account, err := q.GetCashboxAccountByID(a.ctx, payments[0].AccountID)
		if err != nil {
			return db.CashboxAccount{}, fmt.Errorf("payment account: %w", err)
		}
		return account, nil
	}
	accounts, err := q.ListCashboxAccounts(a.ctx)
	if err != nil || len(accounts) == 0 {
		return db.CashboxAccount{}, fmt.Errorf("no payment account configured")
	}
	return accounts[0], nil
}

func createExchangeInvoice(a *App, q *db.Queries, cashbox db.CashboxAccount, shift db.Shift, source db.Invoice, inventoryID, total int64, branchCode string) (db.Invoice, error) {
	year := int64(time.Now().Year())
	invoiceCode, index, err := a.allocateInvoiceCode(
		q, invoiceTypeExchange, branchCode, year,
	)
	if err != nil {
		return db.Invoice{}, err
	}
	invoice, err := q.CreateInvoice(a.ctx, db.CreateInvoiceParams{
		CashboxID: cashbox.ID, ShiftID: shift.ID,
		InvoiceCode:  invoiceCode,
		InvoiceIndex: index, Year: year, ClientID: source.ClientID,
		InventoryID: inventoryID, Discount: 0,
		Subtotal: total, DiscountedTotal: total, GrandTotal: total,
	})
	if err != nil {
		return db.Invoice{}, err
	}
	if err := q.CreateReturnInvoice(a.ctx, db.CreateReturnInvoiceParams{
		InvoiceID: invoice.ID, SalesInvoiceID: source.ID,
	}); err != nil {
		return db.Invoice{}, err
	}
	return invoice, nil
}

func recordExchangeProducts(a *App, q *db.Queries, invoiceID, inventoryID int64, priced pricedExchange) error {
	if err := q.CreateInvoiceProduct(a.ctx, db.CreateInvoiceProductParams{
		InvoiceID: invoiceID, ProductID: priced.returnedProduct.ID,
		UnitPrice: priced.returnedLine.UnitPrice, LineTotal: -priced.returnedTotal,
		Discount: priced.returnedLine.Discount, Quantity: -1,
	}); err != nil {
		return err
	}
	if err := q.AddInventoryProductQuantity(a.ctx, db.AddInventoryProductQuantityParams{
		InventoryID: inventoryID, ProductID: priced.returnedProduct.ID, Quantity: 1,
	}); err != nil {
		return err
	}
	for _, replacement := range priced.replacements {
		if err := q.CreateInvoiceProduct(a.ctx, db.CreateInvoiceProductParams{
			InvoiceID: invoiceID, ProductID: replacement.product.ID,
			UnitPrice: replacement.unitPrice, LineTotal: replacement.lineTotal,
			Discount: replacement.discount, Quantity: replacement.qty,
		}); err != nil {
			return err
		}
		updated, err := q.DecreaseInventoryProductQuantity(a.ctx, db.DecreaseInventoryProductQuantityParams{
			Quantity: replacement.qty, InventoryID: inventoryID,
			ProductID: replacement.product.ID, Quantity_2: replacement.qty,
		})
		if err != nil {
			return err
		}
		if count, err := updated.RowsAffected(); err != nil || count != 1 {
			return fmt.Errorf("insufficient stock for replacement product %s", replacement.product.Code)
		}
	}
	return nil
}

func recordExchangePayments(a *App, q *db.Queries, invoiceID, shiftID int64, payments []checkoutPayment) error {
	netByAccount := make(map[int64]int64)
	for _, payment := range payments {
		if err := q.CreateInvoicePaymentEntry(a.ctx, db.CreateInvoicePaymentEntryParams{
			InvoiceID: invoiceID, CashboxID: payment.AccountID,
			Amount: payment.AmountCents, EntryType: payment.EntryType,
		}); err != nil {
			return err
		}
		netByAccount[payment.AccountID] += payment.AmountCents
	}
	for accountID, amount := range netByAccount {
		if amount <= 0 {
			return fmt.Errorf("payment account %d has no positive net payment", accountID)
		}
		if err := q.CreateInvoicePayment(a.ctx, db.CreateInvoicePaymentParams{
			InvoiceID: invoiceID, CashboxID: accountID, Amount: amount,
		}); err != nil {
			return err
		}
		if _, err := q.AddCashboxAccountBalance(a.ctx, db.AddCashboxAccountBalanceParams{
			AccountID: accountID, ShiftID: shiftID, Balance: amount,
		}); err != nil {
			return err
		}
	}
	return nil
}
