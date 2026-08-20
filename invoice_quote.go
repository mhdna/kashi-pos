package main

import (
	"fmt"
	"math"
)

// InvoiceQuote is the read-only, authoritative preview of what a cart will
// cost if checked out right now. It reuses priceInvoiceTotals - the exact
// same code path salesInvoiceTx uses to persist an invoice - so the
// calculator's displayed total can never drift from what checkout actually
// charges.
type InvoiceQuote struct {
	SubtotalCents           int64       `json:"subtotalCents"`
	DiscountedSubtotalCents int64       `json:"discountedSubtotalCents"`
	TaxCents                int64       `json:"taxCents"`
	GrandTotalCents         int64       `json:"grandTotalCents"`
	RoundingCurrencyCode    string      `json:"roundingCurrencyCode"`
	RoundingMode            string      `json:"roundingMode"`
	Lines                   []LineQuote `json:"lines"`
}

// LineQuote is the priced breakdown for one cart line, replacing the
// frontend's local per-line discount recomputation.
type LineQuote struct {
	Code           string `json:"code"`
	Barcode        string `json:"barcode"`
	LineTotalCents int64  `json:"lineTotalCents"`
	DiscountCents  int64  `json:"discountCents"`
}

// QuoteInvoiceTotal prices a cart exactly as checkout would, without
// persisting anything. Call this reactively as the cart changes instead of
// recomputing tax/rounding in the frontend.
func (a *App) QuoteInvoiceTotal(itemsJSON string) (InvoiceQuote, error) {
	var quote InvoiceQuote
	if a.store == nil {
		return quote, fmt.Errorf("database not initialized")
	}
	items, err := parseSalesItems(itemsJSON)
	if err != nil {
		return quote, err
	}
	pricing, err := a.loadPricingContext()
	if err != nil {
		return quote, err
	}
	totals, err := a.priceInvoiceTotals(a.store.Queries, pricing.InventoryID, items, pricing.Settings, pricing.RoundingMode, pricing.CustomPricingEnabled, pricing.PerUnitPricingEnabled)
	if err != nil {
		return quote, err
	}
	roundingCurrency, err := resolveCurrency(a.ctx, a.store.Queries, pricing.Settings.RoundingCurrency)
	if err != nil {
		return quote, fmt.Errorf("rounding currency: %w", err)
	}

	lines := make([]LineQuote, 0, len(totals.Products))
	for _, product := range totals.Products {
		lines = append(lines, LineQuote{
			Code:           product.product.Code,
			Barcode:        product.product.Barcode,
			LineTotalCents: product.lineTotal,
			DiscountCents:  product.discount,
		})
	}

	quote = InvoiceQuote{
		SubtotalCents:           totals.Subtotal,
		DiscountedSubtotalCents: totals.DiscountedSubtotal,
		TaxCents:                totals.TaxCents,
		GrandTotalCents:         totals.GrandTotal,
		RoundingCurrencyCode:    roundingCurrency.Code,
		RoundingMode:            pricing.RoundingMode,
		Lines:                   lines,
	}
	return quote, nil
}

// PaymentStatus is the read-only, authoritative answer to "is this cart
// paid in full" for a given set of tendered payments. It reuses
// evaluatePaymentSufficiency - the exact function validatePayments calls
// during checkout - so a calculator bound to this can never show "ready to
// checkout" for a payment set that checkout would actually reject, or vice
// versa.
type PaymentStatus struct {
	GrandTotalCents int64 `json:"grandTotalCents"`
	PaidCents       int64 `json:"paidCents"`
	DiffCents       int64 `json:"diffCents"`
	IsPaidInFull    bool  `json:"isPaidInFull"`
	IsCashRounded   bool  `json:"isCashRounded"`
	CheckoutReady   bool  `json:"checkoutReady"`
}

// QuotePaymentStatus prices the cart (as QuoteInvoiceTotal does) and
// evaluates a proposed set of payments against it using the same
// sufficiency check checkout will apply.
func (a *App) QuotePaymentStatus(itemsJSON, paymentsJSON string) (PaymentStatus, error) {
	var status PaymentStatus
	if a.store == nil {
		return status, fmt.Errorf("database not initialized")
	}
	items, err := parseSalesItems(itemsJSON)
	if err != nil {
		return status, err
	}
	payments, err := parsePayments(paymentsJSON)
	if err != nil {
		return status, err
	}
	pricing, err := a.loadPricingContext()
	if err != nil {
		return status, err
	}
	totals, err := a.priceInvoiceTotals(a.store.Queries, pricing.InventoryID, items, pricing.Settings, pricing.RoundingMode, pricing.CustomPricingEnabled, pricing.PerUnitPricingEnabled)
	if err != nil {
		return status, err
	}
	roundingCurrency, err := resolveCurrency(a.ctx, a.store.Queries, pricing.Settings.RoundingCurrency)
	if err != nil {
		return status, fmt.Errorf("rounding currency: %w", err)
	}

	var paid int64
	hasForeignCashPayment := false
	for _, payment := range payments {
		if payment.AmountCents > math.MaxInt64-paid {
			return status, fmt.Errorf("payment total is too large")
		}
		paid += payment.AmountCents
		if payment.AmountCents <= 0 {
			continue
		}
		account, err := a.store.GetCashboxAccountByID(a.ctx, payment.AccountID)
		if err != nil {
			continue
		}
		if currency, currErr := resolveCurrency(a.ctx, a.store.Queries, account.CurrencyCode); currErr == nil && currency.CashRoundingUnit > 1 {
			hasForeignCashPayment = true
		}
	}

	ok, roundedEquivalent, err := evaluatePaymentSufficiency(paid, totals.GrandTotal, pricing.Settings.MaxOverpaymentCents, roundingCurrency, pricing.RoundingMode, hasForeignCashPayment)
	if err != nil {
		return status, err
	}

	status = PaymentStatus{
		GrandTotalCents: totals.GrandTotal,
		PaidCents:       paid,
		DiffCents:       paid - totals.GrandTotal,
		IsPaidInFull:    paid == totals.GrandTotal,
		IsCashRounded:   roundedEquivalent,
		CheckoutReady:   ok,
	}
	return status, nil
}

// ConvertCashEntryToCents converts a keypad-entered amount in a currency's
// major units (whole units for a physically-rounded cash currency like
// LBP, or a decimal amount for a cent-precision currency like USD) into
// accounting cents, using the exact same conversion checkout will apply -
// so entering the amount the calculator displayed as due is guaranteed to
// satisfy the paid-in-full check.
func (a *App) ConvertCashEntryToCents(currencyCode string, amount float64) (int64, error) {
	if a.store == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return 0, fmt.Errorf("invalid amount")
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return 0, fmt.Errorf("settings: %w", err)
	}
	roundingMode, err := normalizeRoundingMode(settings.RoundingMode)
	if err != nil {
		return 0, err
	}
	currency, err := resolveCurrency(a.ctx, a.store.Queries, currencyCode)
	if err != nil {
		return 0, fmt.Errorf("currency: %w", err)
	}
	if currency.CashRoundingUnit > 1 {
		units := int64(math.Round(amount))
		return roundCentsFromCurrencyUnits(units, currency.UnitsPerUsdMicros, roundingMode)
	}
	usdAmount := amount * unitsPerUsdMicroScale / float64(currency.UnitsPerUsdMicros)
	return roundFloatToCents(usdAmount, roundingMode)
}
