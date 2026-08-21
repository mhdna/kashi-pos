package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	SalespersonID    int64   `json:"salespersonId,omitempty"`
	Note             string  `json:"note,omitempty"`
}

type checkoutPayment struct {
	AccountID    int64  `json:"accountId"`
	AmountCents  int64  `json:"amountCents"`
	EntryType    string `json:"entryType"`
	CouponCode   string `json:"couponCode,omitempty"`
	CouponTypeID int64  `json:"couponTypeId,omitempty"`
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

// invoiceTotals is the priced result of an item list before persistence.
// salesInvoiceTx (mutating checkout) and QuoteInvoiceTotal (read-only
// preview) both call priceInvoiceTotals so they can never diverge.
type invoiceTotals struct {
	Products           []pricedInvoiceProduct
	Subtotal           int64
	DiscountedSubtotal int64
	TaxCents           int64
	GrandTotal         int64
	ReceiptItems       []invoiceItem
}

func (a *App) priceInvoiceTotals(q *db.Queries, inventoryID int64, items []salesInvoiceItem, settings db.Setting, roundingMode string, customPricingEnabled, perUnitPricingEnabled bool) (invoiceTotals, error) {
	var totals invoiceTotals
	overrides, err := loadActiveOverrides(a.ctx, q)
	if err != nil {
		return totals, fmt.Errorf("load active price/discount overrides: %w", err)
	}
	products, subtotal, discountedSubtotal, receiptItems, err := a.validateAndPriceItems(q, inventoryID, items, roundingMode, customPricingEnabled, overrides, perUnitPricingEnabled)
	if err != nil {
		return totals, err
	}
	taxCents, err := roundFloatToCents(float64(discountedSubtotal)/100*settings.TaxRate, roundingMode)
	if err != nil {
		return totals, fmt.Errorf("tax: %w", err)
	}
	grandTotal := discountedSubtotal + taxCents
	if grandTotal < discountedSubtotal {
		return totals, fmt.Errorf("invoice total is too large")
	}
	roundingCurrency, err := resolveCurrency(a.ctx, q, settings.RoundingCurrency)
	if err != nil {
		return totals, fmt.Errorf("rounding currency: %w", err)
	}
	grandTotal, err = roundCentsByCurrency(grandTotal, roundingCurrency.UnitsPerUsdMicros, roundingCurrency.CashRoundingUnit, roundingMode)
	if err != nil {
		return totals, fmt.Errorf("round invoice total: %w", err)
	}
	totals.Products = products
	totals.Subtotal = subtotal
	totals.DiscountedSubtotal = discountedSubtotal
	totals.TaxCents = taxCents
	totals.GrandTotal = grandTotal
	totals.ReceiptItems = receiptItems
	return totals, nil
}

// pricingContext bundles the settings/feature-flag state needed to price a
// cart. salesInvoiceTx (checkout) and QuoteInvoiceTotal/QuotePaymentStatus
// (read-only preview) all load it the same way so they can never disagree
// about which rounding mode, sales inventory, or pricing flags are active.
type pricingContext struct {
	Settings              db.Setting
	RoundingMode          string
	InventoryID           int64
	CustomPricingEnabled  bool
	PerUnitPricingEnabled bool
}

func (a *App) loadPricingContext() (pricingContext, error) {
	var pricing pricingContext
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return pricing, fmt.Errorf("settings: %w", err)
	}
	if math.IsNaN(settings.TaxRate) || math.IsInf(settings.TaxRate, 0) || settings.TaxRate < 0 || settings.TaxRate > 1 {
		return pricing, fmt.Errorf("invalid tax rate")
	}
	roundingMode, err := normalizeRoundingMode(settings.RoundingMode)
	if err != nil {
		return pricing, err
	}
	inventoryID, err := a.salesInventoryID(settings.SalesInventoryID)
	if err != nil {
		return pricing, err
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

	pricing.Settings = settings
	pricing.RoundingMode = roundingMode
	pricing.InventoryID = inventoryID
	pricing.CustomPricingEnabled = customPricingEnabled
	pricing.PerUnitPricingEnabled = perUnitPricingEnabled
	return pricing, nil
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
	pricing, err := a.loadPricingContext()
	if err != nil {
		return result, err
	}
	settings, roundingMode, inventoryID, customPricingEnabled, perUnitPricingEnabled :=
		pricing.Settings, pricing.RoundingMode, pricing.InventoryID, pricing.CustomPricingEnabled, pricing.PerUnitPricingEnabled
	shift, err := a.store.GetOpenShift(a.ctx)
	if err != nil {
		return result, fmt.Errorf("no open shift: %w", err)
	}
	clientID, err := a.resolveClientID(params.ClientID)
	if err != nil {
		return result, err
	}

	year := int64(time.Now().Year())
	err = a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		cashbox, err := q.GetCashboxAccountByID(a.ctx, payments[0].AccountID)
		if err != nil {
			return fmt.Errorf("payment account: %w", err)
		}
		totals, err := a.priceInvoiceTotals(q, inventoryID, items, settings, roundingMode, customPricingEnabled, perUnitPricingEnabled)
		if err != nil {
			return err
		}
		products, subtotal, discountedSubtotal, receiptItems, grandTotal := totals.Products, totals.Subtotal, totals.DiscountedSubtotal, totals.ReceiptItems, totals.GrandTotal
		roundingCurrency, err := resolveCurrency(a.ctx, q, settings.RoundingCurrency)
		if err != nil {
			return fmt.Errorf("rounding currency: %w", err)
		}
		if _, err := normalizeCashRoundedPayments(a.ctx, q, payments, roundingMode); err != nil {
			return err
		}
		if err := validatePayments(a.ctx, q, payments, grandTotal, settings.MaxOverpaymentCents, roundingCurrency, roundingMode); err != nil {
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
				SalespersonID: product.salespersonID, Note: product.note,
				PriceListID:    sql.NullInt64{Int64: product.priceListID, Valid: product.priceListID != 0},
				DiscountListID: sql.NullInt64{Int64: product.discountListID, Valid: product.discountListID != 0},
			}); err != nil {
				return fmt.Errorf("add product %s: %w", product.product.Code, err)
			}
			// Inventory mutation is temporarily disabled during checkout.
			// Stock will be reconciled by the inventory workflow separately.
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
		if err := recordSpecialPaymentMetadata(a, q, invoice.ID, clientID, payments); err != nil {
			return err
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

func normalizeCashRoundedPayments(ctx context.Context, q *db.Queries, payments []checkoutPayment, roundingMode string) (bool, error) {
	changed := false
	for i := range payments {
		payment := &payments[i]
		if payment.AmountCents <= 0 {
			continue
		}
		account, err := q.GetCashboxAccountByID(ctx, payment.AccountID)
		if err != nil {
			return false, fmt.Errorf("payment account %d: %w", payment.AccountID, err)
		}
		currency, err := resolveCurrency(ctx, q, account.CurrencyCode)
		if err != nil {
			return false, fmt.Errorf("payment currency: %w", err)
		}
		// Only currencies configured with a cash-rounding unit coarser than
		// 1 (e.g. LBP rounded to the nearest 10,000) physically lose
		// precision when tendered - a currency whose minor unit already
		// matches the accounting cent 1:1 (like USD) never needs this
		// normalization, regardless of account name.
		if currency.CashRoundingUnit <= 1 {
			continue
		}
		negative := payment.AmountCents < 0
		amount := absInt64(payment.AmountCents)
		canonical, err := roundCashEquivalentCents(amount, currency.UnitsPerUsdMicros, currency.CashRoundingUnit, roundingMode)
		if err != nil {
			return false, fmt.Errorf("round cash payment: %w", err)
		}
		if negative {
			canonical = -canonical
		}
		if canonical != payment.AmountCents {
			payment.AmountCents = canonical
			changed = true
		}
	}
	return changed, nil
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
		if item.SalespersonID < 0 {
			return nil, fmt.Errorf("invalid salesperson for item %q", item.Code)
		}
		if len([]rune(item.Note)) > 2000 {
			return nil, fmt.Errorf("note is too long for item %q", item.Code)
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

func validatePayments(ctx context.Context, q *db.Queries, payments []checkoutPayment, total, maxOverpaymentCents int64, roundingCurrency db.Currency, roundingMode string) error {
	var paid int64
	hasForeignCashPayment := false
	for i := range payments {
		payment := &payments[i]
		if _, err := q.GetCashboxAccountByID(ctx, payment.AccountID); err != nil {
			return fmt.Errorf("payment account %d: %w", payment.AccountID, err)
		}
		account, err := q.GetCashboxAccountByID(ctx, payment.AccountID)
		if err != nil {
			return fmt.Errorf("payment account %d: %w", payment.AccountID, err)
		}
		if strings.EqualFold(account.Name, "On Account") && payment.AmountCents < 0 {
			return fmt.Errorf("on account cannot be used as change")
		}
		if strings.EqualFold(account.Name, "On Coupon") {
			if payment.AmountCents < 0 || strings.TrimSpace(payment.CouponCode) == "" || payment.CouponTypeID <= 0 {
				return fmt.Errorf("coupon code and type are required")
			}
			couponType, typeErr := q.GetCouponTypeByID(ctx, payment.CouponTypeID)
			if typeErr != nil {
				return fmt.Errorf("coupon type: %w", typeErr)
			}
			if couponType.IsActive == 0 {
				return fmt.Errorf("coupon type is inactive")
			}
			if couponType.ValueCents != payment.AmountCents {
				return fmt.Errorf("coupon value or type does not match")
			}
			if couponType.ReferenceOnly == 0 {
				coupon, couponErr := q.GetActiveCouponByCode(ctx, strings.TrimSpace(payment.CouponCode))
				if couponErr != nil {
					if errors.Is(couponErr, sql.ErrNoRows) {
						return fmt.Errorf("coupon %q is invalid, expired, or already redeemed", strings.TrimSpace(payment.CouponCode))
					}
					return fmt.Errorf("coupon: %w", couponErr)
				}
				if coupon.TypeID.Int64 != payment.CouponTypeID || coupon.ValueCents.Int64 != payment.AmountCents {
					return fmt.Errorf("coupon value or type does not match")
				}
				// Keep the database's canonical code for the payment foreign key and redemption update.
				payments[i].CouponCode = coupon.Code
				if coupon.ValidUntil != "" {
					expires, parseErr := time.Parse("2006-01-02", coupon.ValidUntil)
					if parseErr == nil && time.Now().After(expires.Add(24*time.Hour)) {
						return fmt.Errorf("coupon has expired")
					}
				}
			}
		}
		// Only a payment or change entry in a currency configured with a
		// cash-rounding unit coarser than 1 (e.g. LBP rounded to the
		// nearest 10,000) can fail to reliably hit exact-cents parity with
		// a total that may itself be rounded to the invoice's configured
		// rounding currency's cash unit — give those, and only those,
		// rounding tolerance. Change (AmountCents < 0) is just as subject
		// to that rounding grid as a payment, so it qualifies too.
		if payment.AmountCents != 0 {
			if paymentCurrency, currErr := resolveCurrency(ctx, q, account.CurrencyCode); currErr == nil && paymentCurrency.CashRoundingUnit > 1 {
				hasForeignCashPayment = true
			}
		}
		if payment.AmountCents > math.MaxInt64-paid {
			return fmt.Errorf("payment total is too large")
		}
		paid += payment.AmountCents
	}
	ok, _, err := evaluatePaymentSufficiency(paid, total, maxOverpaymentCents, roundingCurrency, roundingMode, hasForeignCashPayment)
	if err != nil {
		return err
	}
	if !ok {
		if paid < total {
			return fmt.Errorf("payments total %d is less than invoice total %d", paid, total)
		}
		return fmt.Errorf("overpayment %d exceeds maximum allowed %d", paid-total, maxOverpaymentCents)
	}
	return nil
}

// evaluatePaymentSufficiency decides whether a tendered amount satisfies an
// invoice total, accounting for the fact that a payment in a non-default
// currency may legitimately never hit exact-cents parity with a total that
// was itself rounded to the invoice's configured rounding currency's cash
// unit. Used by both validatePayments (the mutating checkout path) and
// QuotePaymentStatus (the read-only preview), so the calculator's "ready to
// checkout" state can never diverge from what checkout will actually accept.
func evaluatePaymentSufficiency(paid, total, maxOverpaymentCents int64, roundingCurrency db.Currency, roundingMode string, hasForeignCashPayment bool) (ok bool, roundedEquivalent bool, err error) {
	if hasForeignCashPayment && paid != total {
		paidRounded, paidErr := roundCurrencyUnitsFromCents(absInt64(paid), roundingCurrency.UnitsPerUsdMicros, roundingCurrency.CashRoundingUnit, roundingMode)
		totalRounded, totalErr := roundCurrencyUnitsFromCents(absInt64(total), roundingCurrency.UnitsPerUsdMicros, roundingCurrency.CashRoundingUnit, roundingMode)
		roundedEquivalent = paidErr == nil && totalErr == nil && paidRounded == totalRounded
	}
	if paid < total && !roundedEquivalent {
		return false, roundedEquivalent, nil
	}
	if paid > total && !roundedEquivalent && paid-total > maxOverpaymentCents {
		return false, roundedEquivalent, nil
	}
	return true, roundedEquivalent, nil
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func recordSpecialPaymentMetadata(a *App, q *db.Queries, invoiceID, clientID int64, payments []checkoutPayment) error {
	for _, payment := range payments {
		account, err := q.GetCashboxAccountByID(a.ctx, payment.AccountID)
		if err != nil {
			return fmt.Errorf("payment account: %w", err)
		}
		switch {
		case strings.EqualFold(account.Name, "On Account"):
			client, clientErr := q.GetClientByID(a.ctx, clientID)
			if clientErr != nil || strings.Contains(strings.ToLower(client.Name), "walk-in") {
				return fmt.Errorf("a named client is required for on account payments")
			}
			if _, err := q.CreateClientAccountEntry(a.ctx, db.CreateClientAccountEntryParams{ClientID: clientID, InvoiceID: invoiceID, Amount: payment.AmountCents}); err != nil {
				return fmt.Errorf("save client account: %w", err)
			}
		case strings.EqualFold(account.Name, "On Coupon"):
			code := strings.TrimSpace(payment.CouponCode)
			if _, err := q.CreateInvoiceCouponPayment(a.ctx, db.CreateInvoiceCouponPaymentParams{InvoiceID: invoiceID, CouponCode: code, CouponTypeID: payment.CouponTypeID, Amount: payment.AmountCents}); err != nil {
				return fmt.Errorf("save coupon payment: %w", err)
			}
			couponType, err := q.GetCouponTypeByID(a.ctx, payment.CouponTypeID)
			if err != nil {
				return fmt.Errorf("coupon type: %w", err)
			}
			if couponType.ReferenceOnly != 0 {
				continue
			}
			if err := q.RedeemCoupon(a.ctx, db.RedeemCouponParams{RedeemedInvoiceID: sql.NullInt64{Int64: invoiceID, Valid: true}, Code: code}); err != nil {
				return fmt.Errorf("redeem coupon: %w", err)
			}
		}
	}
	return nil
}
