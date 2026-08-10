package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

type exchangePolicy struct {
	allowLate       bool
	allowLowerTotal bool
	returnOnly      bool
}

var posExchangePolicy = exchangePolicy{}
var adminReturnExchangePolicy = exchangePolicy{allowLate: true, allowLowerTotal: true}

// CheckoutExchangeInvoice records one returned line and one or more
// replacement lines. Only the positive difference is paid by the customer.
func (a *App) CheckoutExchangeInvoice(itemsJSON, paymentsJSON string, originalInvoiceID, _ int64, salespersonID int64, notes string) (map[string]any, error) {
	return a.checkoutReturnExchangeInvoice(itemsJSON, paymentsJSON, originalInvoiceID, salespersonID, notes, 0, posExchangePolicy)
}

func (a *App) checkoutReturnExchangeInvoice(itemsJSON, paymentsJSON string, originalInvoiceID, salespersonID int64, notes string, settlementAccountID int64, policy exchangePolicy) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if salespersonID <= 0 {
		return nil, fmt.Errorf("salesperson is required")
	}
	if _, err := a.store.GetSalespersonByID(a.ctx, salespersonID); err != nil {
		return nil, fmt.Errorf("salesperson: %w", err)
	}
	request, err := parseExchangeRequest(itemsJSON, policy.returnOnly)
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
	var exchangeInvoiceID int64
	err = a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		source, err := q.GetSalesInvoiceByID(a.ctx, originalInvoiceID)
		if err != nil {
			return fmt.Errorf("source invoice: %w", err)
		}
		if !policy.allowLate {
			if err := validateExchangeWindow(source.CreatedAt, time.Now()); err != nil {
				return err
			}
		}
		priced, err := a.validateAndPriceExchange(q, source.ID, request, roundingMode)
		if err != nil {
			return err
		}
		if !policy.allowLowerTotal && priced.replacementTotal < priced.returnedTotal {
			return fmt.Errorf("combined replacement total must equal or exceed the returned item")
		}
		netDifference := priced.replacementTotal - priced.returnedTotal
		taxCents, err := signedAdjustmentTax(netDifference, settings.TaxRate, roundingMode)
		if err != nil {
			return err
		}
		total := netDifference + taxCents
		if policy.allowLate && strings.TrimSpace(paymentsJSON) == "" && total != 0 {
			if settlementAccountID <= 0 {
				return fmt.Errorf("settlement account is required for a non-zero admin adjustment")
			}
			entryType := "payment"
			if total < 0 {
				entryType = "change"
			}
			payments = []checkoutPayment{{AccountID: settlementAccountID, AmountCents: total, EntryType: entryType}}
		}
		total, err = roundCentsByCurrency(total, settings.ExchangeRate, settings.RoundingCurrency, roundingMode)
		if err != nil {
			return fmt.Errorf("round exchange total: %w", err)
		}
		if _, err := normalizeCashRoundedPayments(a.ctx, q, payments, settings.ExchangeRate, roundingMode); err != nil {
			return err
		}
		if err := validatePayments(a.ctx, q, payments, total, settings.MaxOverpaymentCents, settings.ExchangeRate, roundingMode); err != nil {
			return err
		}
		cashbox, err := exchangeCashbox(a, q, payments)
		if err != nil {
			return err
		}
		invoice, err := createExchangeInvoice(
			a, q, cashbox, shift, source, inventoryID, salespersonID, total, settings.BranchCode, notes, policy.returnOnly,
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
		exchangeInvoiceID = invoice.ID
		result = map[string]any{
			"invoiceCode":    invoice.InvoiceCode,
			"difference":     float64(total) / 100,
			"receiptPrinted": false,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if settings.ReceiptEnabled != 0 {
		if err := a.printInvoiceReceipt(exchangeInvoiceID, payments); err != nil {
			result["receiptError"] = err.Error()
		} else {
			result["receiptPrinted"] = true
		}
	}
	return result, err
}

func signedAdjustmentTax(netDifference int64, taxRate float64, roundingMode string) (int64, error) {
	if netDifference == 0 || taxRate == 0 {
		return 0, nil
	}
	tax, err := roundFloatToCents(math.Abs(float64(netDifference))/100*taxRate, roundingMode)
	if err != nil {
		return 0, err
	}
	if netDifference < 0 {
		return -tax, nil
	}
	return tax, nil
}

// CheckoutAdminReturnInvoice is the privileged admin entry point. It uses the
// same transaction and validation as POS exchanges, but intentionally bypasses
// the customer-facing time and value restrictions.
func (a *App) CheckoutAdminReturnInvoice(itemsJSON, paymentsJSON string, originalInvoiceID, salespersonID int64, notes string) (map[string]any, error) {
	return a.checkoutReturnExchangeInvoice(itemsJSON, paymentsJSON, originalInvoiceID, salespersonID, notes, 0, exchangePolicy{allowLate: true, allowLowerTotal: true, returnOnly: true})
}

func (a *App) checkoutAdminReturnInvoice(itemsJSON string, originalInvoiceID, salespersonID, settlementAccountID int64, notes string) (map[string]any, error) {
	return a.checkoutReturnExchangeInvoice(itemsJSON, "", originalInvoiceID, salespersonID, notes, settlementAccountID, exchangePolicy{allowLate: true, allowLowerTotal: true, returnOnly: true})
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

func createExchangeInvoice(a *App, q *db.Queries, cashbox db.CashboxAccount, shift db.Shift, source db.Invoice, inventoryID, salespersonID, total int64, branchCode, notes string, returnOnlyFlag ...bool) (db.Invoice, error) {
	returnOnly := len(returnOnlyFlag) > 0 && returnOnlyFlag[0]
	year := int64(time.Now().Year())
	invoiceCode, index, err := a.allocateInvoiceCode(
		q, invoiceTypeForAdjustment(returnOnly), branchCode, year,
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
		SalespersonID: salespersonID,
		Notes:         strings.TrimSpace(notes),
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
	for _, returned := range priced.returned {
		if err := q.CreateInvoiceProduct(a.ctx, db.CreateInvoiceProductParams{
			InvoiceID: invoiceID, ProductID: returned.product.ID,
			UnitPrice: returned.line.UnitPrice, LineTotal: -returned.lineTotal,
			Discount: returned.line.Discount, Quantity: -returned.qty,
		}); err != nil {
			return err
		}
	}
	// Inventory mutation is temporarily disabled during checkout.
	// Stock will be reconciled by the inventory workflow separately.
	// if err := q.AddInventoryProductQuantity(a.ctx, db.AddInventoryProductQuantityParams{
	// 	InventoryID: inventoryID, ProductID: priced.returnedProduct.ID, Quantity: 1,
	// }); err != nil {
	// 	return err
	// }
	for _, replacement := range priced.replacements {
		if err := q.CreateInvoiceProduct(a.ctx, db.CreateInvoiceProductParams{
			InvoiceID: invoiceID, ProductID: replacement.product.ID,
			UnitPrice: replacement.unitPrice, LineTotal: replacement.lineTotal,
			Discount: replacement.discount, Quantity: replacement.qty,
		}); err != nil {
			return err
		}
		// updated, err := q.DecreaseInventoryProductQuantity(a.ctx, db.DecreaseInventoryProductQuantityParams{
		// 	Quantity: replacement.qty, InventoryID: inventoryID,
		// 	ProductID: replacement.product.ID, Quantity_2: replacement.qty,
		// })
		// if err != nil {
		// 	return err
		// }
		// if count, err := updated.RowsAffected(); err != nil || count != 1 {
		// 	return fmt.Errorf("insufficient stock for replacement product %s", replacement.product.Code)
		// }
	}
	return nil
}

func invoiceTypeForAdjustment(returnOnly bool) string {
	if returnOnly {
		return "return"
	}
	return "exchange"
}

func recordExchangePayments(a *App, q *db.Queries, invoiceID, shiftID int64, payments []checkoutPayment) error {
	netByAccount := netPaymentsByAccount(payments)
	for _, payment := range payments {
		if err := q.CreateInvoicePaymentEntry(a.ctx, db.CreateInvoicePaymentEntryParams{
			InvoiceID: invoiceID, CashboxID: payment.AccountID,
			Amount: payment.AmountCents, EntryType: payment.EntryType,
		}); err != nil {
			return err
		}
	}
	for accountID, amount := range netByAccount {
		if amount == 0 {
			continue
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
