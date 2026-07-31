package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

type salesInvoiceItem struct {
	Code             string  `json:"code"`
	Barcode          string  `json:"barcode"`
	Qty              float64 `json:"qty"`
	CustomPriceCents *int64  `json:"customPriceCents,omitempty"`
	UnitPricesCents  []int64 `json:"unitPricesCents,omitempty"`
	UnitDiscounts    []int64 `json:"unitDiscounts,omitempty"`
}

type checkoutPayment struct {
	AccountID   int64  `json:"accountId"`
	AmountCents int64  `json:"amountCents"`
	EntryType   string `json:"entryType"`
}

type SalesInvoiceParams struct {
	ItemsJSON    string `json:"itemsJson"`
	PaymentsJSON string `json:"paymentsJson"`
	ClientID     int64  `json:"clientId"`
	Notes        string `json:"notes"`
}

type SalesInvoiceResult struct {
	InvoiceID   int64
	InvoiceCode string
	Items       []invoiceItem
	GrandTotal  int64
	Payments    []checkoutPayment
	Notes       string
}

func (a *App) salesInvoiceTx(params SalesInvoiceParams) (SalesInvoiceResult, error) {
	var result SalesInvoiceResult
	if a.store == nil {
		return result, fmt.Errorf("database not initialized")
	}

	items, err := parseSalesItems(params.ItemsJSON)
	if err != nil {
		return result, err
	}
	payments, err := parsePayments(params.PaymentsJSON)
	if err != nil {
		return result, err
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return result, fmt.Errorf("settings: %w", err)
	}
	if math.IsNaN(settings.TaxRate) || math.IsInf(settings.TaxRate, 0) || settings.TaxRate < 0 || settings.TaxRate > 1 {
		return result, fmt.Errorf("invalid tax rate")
	}
	roundingMode, err := normalizeRoundingMode(settings.RoundingMode)
	if err != nil {
		return result, err
	}
	shift, err := a.store.GetOpenShift(a.ctx)
	if err != nil {
		return result, fmt.Errorf("no open shift: %w", err)
	}
	inventoryID, err := a.salesInventoryID(settings.SalesInventoryID)
	if err != nil {
		return result, err
	}
	clientID, err := a.resolveClientID(params.ClientID)
	if err != nil {
		return result, err
	}
	customPricingEnabled := false
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, customItemDiscountsFlag); flagErr == nil {
		customPricingEnabled = enabled != 0 // Compatibility with databases upgrading from the combined flag.
	}
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, customItemPricesFlag); flagErr == nil {
		customPricingEnabled = enabled != 0
	}
	perUnitPricingEnabled := false
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, perUnitItemPricesFlag); flagErr == nil {
		perUnitPricingEnabled = enabled != 0
	}
	customDiscountsEnabled := false
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, customItemDiscountsFlag); flagErr == nil {
		customDiscountsEnabled = enabled != 0
	}
	perUnitPricingEnabled = perUnitPricingEnabled || customPricingEnabled || customDiscountsEnabled

	year := int64(time.Now().Year())
	err = a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		cashbox, err := q.GetCashboxAccountByID(a.ctx, payments[0].AccountID)
		if err != nil {
			return fmt.Errorf("payment account: %w", err)
		}
		products, subtotal, discountedSubtotal, receiptItems, err := a.validateAndPriceItems(q, inventoryID, items, roundingMode, customPricingEnabled, perUnitPricingEnabled)
		if err != nil {
			return err
		}
		taxCents, err := roundFloatToCents(float64(discountedSubtotal)/100*settings.TaxRate, roundingMode)
		if err != nil {
			return fmt.Errorf("tax: %w", err)
		}
		grandTotal := discountedSubtotal + taxCents
		if grandTotal < discountedSubtotal {
			return fmt.Errorf("invoice total is too large")
		}
		if err := validatePayments(a.ctx, q, payments, grandTotal, settings.MaxOverpaymentCents); err != nil {
			return err
		}

		invoiceCode, invoiceIndex, err := a.allocateInvoiceCode(
			q, invoiceTypeSales, settings.BranchCode, year,
		)
		if err != nil {
			return fmt.Errorf("allocate invoice code: %w", err)
		}
		invoice, err := q.CreateInvoice(a.ctx, db.CreateInvoiceParams{
			CashboxID: cashbox.ID, ShiftID: shift.ID, InvoiceCode: invoiceCode,
			InvoiceIndex: invoiceIndex, Year: year, ClientID: clientID,
			InventoryID: inventoryID, Discount: 0,
			Subtotal: subtotal, DiscountedTotal: discountedSubtotal, GrandTotal: grandTotal,
			SalespersonID: 0,
			Notes:         strings.TrimSpace(params.Notes),
		})
		if err != nil {
			return fmt.Errorf("create invoice: %w", err)
		}
		if err := q.CreateSalesInvoice(a.ctx, invoice.ID); err != nil {
			return fmt.Errorf("create sales invoice: %w", err)
		}

		for _, product := range products {
			if err := q.CreateInvoiceProduct(a.ctx, db.CreateInvoiceProductParams{
				InvoiceID: invoice.ID, ProductID: product.product.ID, UnitPrice: product.unitPrice,
				LineTotal: product.lineTotal, Discount: product.discount, Quantity: product.qty,
			}); err != nil {
				return fmt.Errorf("add product %s: %w", product.product.Code, err)
			}
			// updated, err := q.DecreaseInventoryProductQuantity(a.ctx, db.DecreaseInventoryProductQuantityParams{
			// 	Quantity: product.qty, InventoryID: inventoryID, ProductID: product.product.ID, Quantity_2: product.qty,
			// })
			// if err != nil {
			// 	return fmt.Errorf("inventory update %s: %w", product.product.Code, err)
			// }
			// if count, err := updated.RowsAffected(); err != nil || count != 1 {
			// 	return fmt.Errorf("insufficient stock for product %s", product.product.Code)
			// }
		}
		netByAccount := netPaymentsByAccount(payments)
		for _, payment := range payments {
			if err := q.CreateInvoicePaymentEntry(a.ctx, db.CreateInvoicePaymentEntryParams{InvoiceID: invoice.ID, CashboxID: payment.AccountID, Amount: payment.AmountCents, EntryType: payment.EntryType}); err != nil {
				return fmt.Errorf("record payment entry: %w", err)
			}
		}
		for accountID, amount := range netByAccount {
			if amount == 0 {
				continue
			}
			if err := q.CreateInvoicePayment(a.ctx, db.CreateInvoicePaymentParams{InvoiceID: invoice.ID, CashboxID: accountID, Amount: amount}); err != nil {
				return fmt.Errorf("record net payment: %w", err)
			}
			if _, err := q.AddCashboxAccountBalance(a.ctx, db.AddCashboxAccountBalanceParams{
				AccountID: accountID, ShiftID: shift.ID, Balance: amount,
			}); err != nil {
				return fmt.Errorf("add cashbox balance: %w", err)
			}
		}
		if err := q.AddClientLoyaltyPoints(a.ctx, db.AddClientLoyaltyPointsParams{
			TotalLoyaltyPoints: grandTotal / 100, ValidLoyaltyPoints: grandTotal / 100, ID: clientID,
		}); err != nil {
			return fmt.Errorf("add loyalty points: %w", err)
		}
		result.InvoiceID, result.InvoiceCode, result.Items, result.GrandTotal, result.Payments, result.Notes = invoice.ID, invoiceCode, receiptItems, grandTotal, payments, strings.TrimSpace(params.Notes)
		return nil
	})
	return result, err
}

func netPaymentsByAccount(payments []checkoutPayment) map[int64]int64 {
	netByAccount := make(map[int64]int64)
	for _, payment := range payments {
		netByAccount[payment.AccountID] += payment.AmountCents
	}
	return netByAccount
}

func parseSalesItems(raw string) ([]salesInvoiceItem, error) {
	var items []salesInvoiceItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil || len(items) == 0 {
		return nil, fmt.Errorf("invalid invoice items")
	}
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		key := item.Code
		if item.Barcode != "" {
			key = "barcode:" + item.Barcode
		}
		if (item.Code == "" && item.Barcode == "") || item.Qty <= 0 || math.Trunc(item.Qty) != item.Qty || item.Qty > math.MaxInt64 || seen[key] {
			return nil, fmt.Errorf("invalid or duplicate item %q", item.Code)
		}
		if item.CustomPriceCents != nil && (*item.CustomPriceCents <= 0 || *item.CustomPriceCents > 99999999) {
			return nil, fmt.Errorf("invalid custom price for item %q", item.Code)
		}
		if len(item.UnitPricesCents) > 0 {
			if len(item.UnitPricesCents) != int(item.Qty) {
				return nil, fmt.Errorf("unit prices do not match quantity for item %q", item.Code)
			}
			for _, price := range item.UnitPricesCents {
				if price <= 0 || price > 99999999 {
					return nil, fmt.Errorf("invalid unit price for item %q", item.Code)
				}
			}
		}
		if len(item.UnitDiscounts) > 0 {
			if len(item.UnitDiscounts) != int(item.Qty) {
				return nil, fmt.Errorf("unit discounts do not match quantity for item %q", item.Code)
			}
			for _, discount := range item.UnitDiscounts {
				if discount < 0 || discount > 100 {
					return nil, fmt.Errorf("invalid unit discount for item %q", item.Code)
				}
			}
		}
		seen[key] = true
	}
	return items, nil
}

func parsePayments(raw string) ([]checkoutPayment, error) {
	var payments []checkoutPayment
	if err := json.Unmarshal([]byte(raw), &payments); err != nil || len(payments) == 0 {
		return nil, fmt.Errorf("invalid payments")
	}
	for i := range payments {
		payment := &payments[i]
		if payment.AccountID <= 0 || payment.AmountCents == 0 {
			return nil, fmt.Errorf("invalid payment")
		}
		if payment.EntryType == "" && payment.AmountCents > 0 {
			payment.EntryType = "payment"
		}
		if (payment.AmountCents > 0 && payment.EntryType != "payment") || (payment.AmountCents < 0 && payment.EntryType != "change") {
			return nil, fmt.Errorf("invalid payment entry type")
		}
	}
	return payments, nil
}

func validatePayments(ctx context.Context, q *db.Queries, payments []checkoutPayment, total, maxOverpaymentCents int64) error {
	var paid int64
	for _, payment := range payments {
		if _, err := q.GetCashboxAccountByID(ctx, payment.AccountID); err != nil {
			return fmt.Errorf("payment account %d: %w", payment.AccountID, err)
		}
		if payment.AmountCents > math.MaxInt64-paid {
			return fmt.Errorf("payment total is too large")
		}
		paid += payment.AmountCents
	}
	if paid < total {
		return fmt.Errorf("payments total %d is less than invoice total %d", paid, total)
	}
	if paid-total > maxOverpaymentCents {
		return fmt.Errorf("overpayment %d exceeds maximum allowed %d", paid-total, maxOverpaymentCents)
	}
	return nil
}
