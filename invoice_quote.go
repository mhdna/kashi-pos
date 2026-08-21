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
	payments, err := optionalPayments(paymentsJSON)
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
		if payment.AmountCents == 0 {
			continue
		}
		account, err := a.store.GetCashboxAccountByID(a.ctx, payment.AccountID)
		if err != nil {
			continue
		}
		// Change given back in a coarse-rounding currency (e.g. LBP) is just
		// as subject to the cash-rounding grid as a payment received in it,
		// so it must grant the same tolerance - not just positive amounts.
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

// ExchangeQuote is the read-only, authoritative preview of what an exchange
// (returned lines + replacement lines) will net out to, reusing
// validateAndPriceExchange - the exact function checkoutReturnExchangeInvoice
// prices with - so the calculator's displayed total can never drift from
// what checkout will actually charge. GrandTotalCents is floored at 0: if
// the replacement value doesn't yet cover the returned value, there is
// nothing to collect yet (see NeedsMoreReplacementValue) rather than a
// negative amount.
type ExchangeQuote struct {
	ReturnedTotalCents        int64 `json:"returnedTotalCents"`
	ReplacementTotalCents     int64 `json:"replacementTotalCents"`
	TaxCents                  int64 `json:"taxCents"`
	GrandTotalCents           int64 `json:"grandTotalCents"`
	NeedsMoreReplacementValue bool  `json:"needsMoreReplacementValue"`
}

func (a *App) quoteExchange(itemsJSON string, originalInvoiceID int64) (ExchangeQuote, pricedExchange, error) {
	var quote ExchangeQuote
	if a.store == nil {
		return quote, pricedExchange{}, fmt.Errorf("database not initialized")
	}
	// returnOnly relaxes "at least one replacement" while the cashier is
	// still only picking returned items - this is a live preview, not a
	// checkout, so a partially-built exchange must still quote cleanly.
	request, err := parseExchangeRequest(itemsJSON, true)
	if err != nil {
		return quote, pricedExchange{}, err
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return quote, pricedExchange{}, fmt.Errorf("settings: %w", err)
	}
	roundingMode, err := normalizeRoundingMode(settings.RoundingMode)
	if err != nil {
		return quote, pricedExchange{}, err
	}
	customPricingEnabled, perUnitPricingEnabled := a.customPricingFlags()
	priced, err := a.validateAndPriceExchange(a.store.Queries, originalInvoiceID, request, roundingMode, customPricingEnabled, perUnitPricingEnabled)
	if err != nil {
		return quote, pricedExchange{}, err
	}
	netDifference := priced.replacementTotal - priced.returnedTotal
	taxCents, err := signedAdjustmentTax(netDifference, settings.TaxRate, roundingMode)
	if err != nil {
		return quote, pricedExchange{}, err
	}
	total := netDifference + taxCents
	roundingCurrency, err := resolveCurrency(a.ctx, a.store.Queries, settings.RoundingCurrency)
	if err != nil {
		return quote, pricedExchange{}, fmt.Errorf("rounding currency: %w", err)
	}
	total, err = roundCentsByCurrency(total, roundingCurrency.UnitsPerUsdMicros, roundingCurrency.CashRoundingUnit, roundingMode)
	if err != nil {
		return quote, pricedExchange{}, fmt.Errorf("round exchange total: %w", err)
	}
	needsMoreReplacementValue := priced.replacementTotal < priced.returnedTotal
	if total < 0 {
		total = 0
	}
	quote = ExchangeQuote{
		ReturnedTotalCents:        priced.returnedTotal,
		ReplacementTotalCents:     priced.replacementTotal,
		TaxCents:                  taxCents,
		GrandTotalCents:           total,
		NeedsMoreReplacementValue: needsMoreReplacementValue,
	}
	return quote, priced, nil
}

// QuoteExchangeTotal prices an in-progress exchange cart exactly as checkout
// would, without persisting anything.
func (a *App) QuoteExchangeTotal(itemsJSON string, originalInvoiceID int64) (ExchangeQuote, error) {
	quote, _, err := a.quoteExchange(itemsJSON, originalInvoiceID)
	return quote, err
}

// QuoteExchangePaymentStatus prices the exchange (as QuoteExchangeTotal does)
// and evaluates a proposed set of payments against it, using the same
// sufficiency check checkout will apply. CheckoutReady additionally requires
// the replacement value to already cover the returned value, since checkout
// will reject an exchange that doesn't.
func (a *App) QuoteExchangePaymentStatus(itemsJSON, paymentsJSON string, originalInvoiceID int64) (PaymentStatus, error) {
	var status PaymentStatus
	quote, _, err := a.quoteExchange(itemsJSON, originalInvoiceID)
	if err != nil {
		return status, err
	}
	payments, err := optionalPayments(paymentsJSON)
	if err != nil {
		return status, err
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return status, fmt.Errorf("settings: %w", err)
	}
	roundingMode, err := normalizeRoundingMode(settings.RoundingMode)
	if err != nil {
		return status, err
	}
	roundingCurrency, err := resolveCurrency(a.ctx, a.store.Queries, settings.RoundingCurrency)
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
		if payment.AmountCents == 0 {
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

	ok, roundedEquivalent, err := evaluatePaymentSufficiency(paid, quote.GrandTotalCents, settings.MaxOverpaymentCents, roundingCurrency, roundingMode, hasForeignCashPayment)
	if err != nil {
		return status, err
	}

	status = PaymentStatus{
		GrandTotalCents: quote.GrandTotalCents,
		PaidCents:       paid,
		DiffCents:       paid - quote.GrandTotalCents,
		IsPaidInFull:    paid == quote.GrandTotalCents,
		IsCashRounded:   roundedEquivalent,
		CheckoutReady:   ok && !quote.NeedsMoreReplacementValue,
	}
	return status, nil
}

// QuoteSettlementPaymentStatus evaluates a proposed set of payments against
// an *existing* invoice's already-persisted GrandTotal, instead of
// re-pricing an items JSON. It backs "change settlement" mode, where the
// items/discounts are frozen and only the payments may change - re-pricing
// via QuotePaymentStatus would risk drifting from the original sale if tax
// rate, rounding, or discount-list settings changed since checkout.
func (a *App) QuoteSettlementPaymentStatus(invoiceID int64, paymentsJSON string) (PaymentStatus, error) {
	var status PaymentStatus
	if a.store == nil {
		return status, fmt.Errorf("database not initialized")
	}
	invoice, err := a.store.GetSalesInvoiceByID(a.ctx, invoiceID)
	if err != nil {
		return status, fmt.Errorf("sales invoice not found")
	}
	payments, err := optionalPayments(paymentsJSON)
	if err != nil {
		return status, err
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return status, fmt.Errorf("settings: %w", err)
	}
	roundingMode, err := normalizeRoundingMode(settings.RoundingMode)
	if err != nil {
		return status, err
	}
	roundingCurrency, err := resolveCurrency(a.ctx, a.store.Queries, settings.RoundingCurrency)
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
		if payment.AmountCents == 0 {
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

	ok, roundedEquivalent, err := evaluatePaymentSufficiency(paid, invoice.GrandTotal, settings.MaxOverpaymentCents, roundingCurrency, roundingMode, hasForeignCashPayment)
	if err != nil {
		return status, err
	}

	status = PaymentStatus{
		GrandTotalCents: invoice.GrandTotal,
		PaidCents:       paid,
		DiffCents:       paid - invoice.GrandTotal,
		IsPaidInFull:    paid == invoice.GrandTotal,
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
