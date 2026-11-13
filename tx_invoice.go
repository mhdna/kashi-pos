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
	// The sale option the cashier picked for a product that asks one
	// (Summer/Winter); it picks the line's discount. Empty otherwise.
	SaleOption string `json:"saleOption,omitempty"`
	// The salesperson turned the line's list discount (discount_toggle) or
	// its deal (deal_toggle) off for this sale. Either only takes a saving
	// away.
	NoDiscount bool `json:"noDiscount,omitempty"`
	NoDeal     bool `json:"noDeal,omitempty"`
	// An unknown item line's deal: b2g1_30, b2g1_20 or none (the names stay
	// whatever the deal); empty is the
	// default (unknown_item_deal.go). Ignored on any other line.
	UnknownDeal string `json:"unknownDeal,omitempty"`
	// The cart's own name for the line, echoed on its quote lines so the
	// cart can tell apart lines of one product (every unknown item).
	LineRef string `json:"lineRef,omitempty"`
	// The name the cashier gave an "Unknown item" when ringing it up, shown on
	// the invoice and the receipt in place of "Unknown item". Only an unknown
	// line takes one; any other line's name is its product's.
	Name string `json:"name,omitempty"`
}

type checkoutPayment struct {
	AccountID    int64  `json:"accountId"`
	AmountCents  int64  `json:"amountCents"`
	EntryType    string `json:"entryType"`
	CouponCode   string `json:"couponCode,omitempty"`
	CouponTypeID int64  `json:"couponTypeId,omitempty"`
	// AmountUnits is a cash-rounded currency's entry (LBP) as counted, in
	// that currency's own units and unsigned. Checkout needs it to net an
	// account's change against its payments in LBP before turning that
	// into cents once; see normalizeCashRoundedPayments.
	AmountUnits int64 `json:"amountUnits,omitempty"`
}

type SalesInvoiceParams struct {
	ItemsJSON    string `json:"itemsJson"`
	PaymentsJSON string `json:"paymentsJson"`
	ClientID     int64  `json:"clientId"`
	Notes        string `json:"notes"`
	// The invoice header's salesperson_id isn't set until after this tx
	// commits (see CheckoutInvoice's post-checkout UpdateInvoiceSalesperson
	// backfill), so the sync payload built inside this tx can't read it
	// back from the invoice row -- the caller already has the name
	// (selected once per invoice at the till), so it's threaded straight
	// through instead.
	Salesperson string `json:"salesperson"`
	// UnknownSessionRef is the picture session of a cart holding "Unknown
	// item" lines, sent to central with the sale (see unknown_items.go).
	UnknownSessionRef string `json:"unknownSessionRef,omitempty"`
}

type SalesInvoiceResult struct {
	InvoiceID   int64
	InvoiceCode string
	Items       []invoiceItem
	GrandTotal  int64
	TaxCents    int64
	Payments    []checkoutPayment
	Notes       string
	// For the receipt: Items are priced before it.
	InvoiceDiscount invoiceDiscount
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
	// The terms of the deals the lines name, copied onto the invoice.
	Deals []dealTerms
	// The sale's invoice discount (invoice_discounts.go): DiscountedSubtotal
	// and the lines already have it off.
	InvoiceDiscount invoiceDiscount
}

func (a *App) priceInvoiceTotals(q *db.Queries, inventoryID int64, items []salesInvoiceItem, settings db.Setting, roundingMode string, customPricingEnabled, perUnitPricingEnabled bool) (invoiceTotals, error) {
	var totals invoiceTotals
	listPricing, err := loadActivePricing(a.ctx, q)
	if err != nil {
		return totals, fmt.Errorf("load active price/discount lists: %w", err)
	}
	products, subtotal, discountedSubtotal, receiptItems, err := a.validateAndPriceItems(q, inventoryID, items, roundingMode, customPricingEnabled, listPricing, perUnitPricingEnabled)
	if err != nil {
		return totals, err
	}
	totals.Products, totals.DiscountedSubtotal = products, discountedSubtotal
	if err := a.applyInvoiceDiscount(q, &totals, roundingMode); err != nil {
		return totals, err
	}
	products, discountedSubtotal = totals.Products, totals.DiscountedSubtotal
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
	totals.Deals = invoiceDealTerms(products, listPricing.deals)
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

// customPricingFlags reads the custom-price/per-unit-pricing feature flags
// shared by normal checkout and exchange replacement pricing, so both paths
// can never disagree about whether a cashier is allowed to override a line's
// price/discount.
func (a *App) customPricingFlags() (customPricingEnabled, perUnitPricingEnabled bool) {
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, customItemDiscountsFlag); flagErr == nil {
		customPricingEnabled = enabled != 0 // Compatibility with databases upgrading from the combined flag.
	}
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, customItemPricesFlag); flagErr == nil {
		customPricingEnabled = enabled != 0
	}
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, perUnitItemPricesFlag); flagErr == nil {
		perUnitPricingEnabled = enabled != 0
	}
	customDiscountsEnabled := false
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, customItemDiscountsFlag); flagErr == nil {
		customDiscountsEnabled = enabled != 0
	}
	perUnitPricingEnabled = perUnitPricingEnabled || customPricingEnabled || customDiscountsEnabled
	return customPricingEnabled, perUnitPricingEnabled
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
	customPricingEnabled, perUnitPricingEnabled := a.customPricingFlags()

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

	items, err := parseSalesItems(params.ItemsJSON, a.unknownBarcode(), a.unknownCode())
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
	err = a.checkoutTx(func(q *db.Queries) error {
		cashbox, err := q.GetCashboxAccountByID(a.ctx, payments[0].AccountID)
		if err != nil {
			return fmt.Errorf("payment account: %w", err)
		}
		totals, err := a.priceInvoiceTotals(q, inventoryID, items, settings, roundingMode, customPricingEnabled, perUnitPricingEnabled)
		if err != nil {
			return err
		}
		products, subtotal, discountedSubtotal, receiptItems, grandTotal := totals.Products, totals.Subtotal, totals.DiscountedSubtotal, totals.ReceiptItems, totals.GrandTotal
		// "warn" is the sale screen's confirm (CheckOversell); only "block"
		// is enforced here.
		if settings.OversellMode == oversellBlock {
			short, err := oversoldLines(a.ctx, q, settings, products)
			if err != nil {
				return err
			}
			if len(short) > 0 {
				return oversellError(short)
			}
		}
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
				SalespersonID: product.salespersonID, Note: product.note, ItemName: product.itemName,
				PriceListID:     sql.NullInt64{Int64: product.priceListID, Valid: product.priceListID != 0},
				DiscountListID:  sql.NullInt64{Int64: product.discountListID, Valid: product.discountListID != 0},
				SaleOption:      product.saleOption,
				DealUid:         product.dealUID,
				DealDiscount:    product.dealDiscount,
				InvoiceDiscount: product.invoiceDiscount,
				// An exchange judges the deal by the fallback the unit was sold with.
				DealFallbackPercent: product.dealFallback,
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
		if err := recordInvoiceDeals(a.ctx, q, invoice.ID, totals.Deals); err != nil {
			return err
		}
		if totals.InvoiceDiscount.Cents > 0 {
			if err := q.SetInvoiceDiscount(a.ctx, db.SetInvoiceDiscountParams{
				InvoiceDiscount: totals.InvoiceDiscount.Cents, InvoiceDiscountName: totals.InvoiceDiscount.Name, ID: invoice.ID,
			}); err != nil {
				return fmt.Errorf("record invoice discount: %w", err)
			}
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
		// Anonymous sales are booked against the "Walk-in" pseudo-client, which
		// is not a real customer: it earns no loyalty points, and the branch
		// reports a zero delta to central so it is never credited there either.
		loyaltyPointsDelta := grandTotal / 100
		if client, err := q.GetClientByID(a.ctx, clientID); err == nil && isWalkInClient(client) {
			loyaltyPointsDelta = 0
		}
		if loyaltyPointsDelta != 0 {
			if err := q.AddClientLoyaltyPoints(a.ctx, db.AddClientLoyaltyPointsParams{
				TotalLoyaltyPoints: loyaltyPointsDelta, ValidLoyaltyPoints: loyaltyPointsDelta, ID: clientID,
			}); err != nil {
				return fmt.Errorf("add loyalty points: %w", err)
			}
		}

		if settings.CentralApiUrl != "" {
			if err := queueBranchClientSyncForPurchase(a.ctx, q, settings.CentralApiUrl, clientID); err != nil {
				return fmt.Errorf("queue client sync: %w", err)
			}
			clientRef, err := getOrCreateInvoiceClientRef(a.ctx, q, invoice.ID)
			if err != nil {
				return fmt.Errorf("resolve invoice client ref: %w", err)
			}

			payloadItems := make([]branchInvoicePayloadItem, 0, len(products))
			for _, product := range products {
				payloadItems = append(payloadItems, branchInvoicePayloadItem{
					BranchProductID: product.product.ID,
					Barcode:         product.product.Barcode,
					ProductUID:      product.product.Uid.String,
					UnitPrice:       product.unitPrice,
					LineTotal:       product.lineTotal,
					Discount:        int16(product.discount),
					Quantity:        product.qty,
					Deal:            product.dealName,
					DealDiscount:    product.dealDiscount,
					InvoiceDiscount: product.invoiceDiscount,
				})
			}
			payloadPayments, err := buildBranchInvoicePayments(a.ctx, q, invoice.ID)
			if err != nil {
				return err
			}
			lbpTotal, lbpRate, err := stampInvoiceLBPTotal(a.ctx, q, invoice.ID, grandTotal, roundingMode)
			if err != nil {
				return fmt.Errorf("record LBP total: %w", err)
			}
			if err := queueBranchInvoiceSync(a.ctx, q, settings.CentralApiUrl, "sales_invoice", branchInvoicePayload{
				ClientRef:              clientRef,
				BranchInvoiceCode:      invoiceCode,
				BranchCashboxAccountID: cashbox.ID,
				BranchShiftID:          shift.ID,
				BranchInventoryID:      inventoryID,
				BranchClientID:         clientID,
				ClientUID:              clientUIDForSync(a.ctx, q, clientID),
				GrandTotal:             grandTotal,
				Subtotal:               subtotal,
				DiscountedTotal:        discountedSubtotal,
				LoyaltyPointsDelta:     loyaltyPointsDelta,
				OccurredAt:             time.Now().UTC(),
				SalespersonName:        params.Salesperson,
				SalespersonID:          salespersonRemoteIDByName(a.ctx, q, params.Salesperson),
				Payments:               payloadPayments,
				Items:                  payloadItems,
				UnknownSessionRef:      params.UnknownSessionRef,
				LbpTotal:               lbpTotal,
				LbpRate:                lbpRate,
				InvoiceDiscount:        totals.InvoiceDiscount.Cents,
				InvoiceDiscountName:    totals.InvoiceDiscount.Name,
			}); err != nil {
				return fmt.Errorf("queue branch invoice sync: %w", err)
			}
		}

		result.InvoiceID, result.InvoiceCode, result.Items, result.GrandTotal, result.Payments, result.Notes = invoice.ID, invoiceCode, receiptItems, grandTotal, payments, strings.TrimSpace(params.Notes)
		result.TaxCents = totals.TaxCents
		result.InvoiceDiscount = totals.InvoiceDiscount
		return nil
	})
	if err == nil && settings.CentralApiUrl != "" {
		a.kickSyncWorker()
	}
	return result, err
}

func netPaymentsByAccount(payments []checkoutPayment) map[int64]int64 {
	netByAccount := make(map[int64]int64)
	for _, payment := range payments {
		netByAccount[payment.AccountID] += payment.AmountCents
	}
	return netByAccount
}

// normalizeCashRoundedPayments turns the entries made in a cash-rounded
// currency (LBP: notes come in 10,000s) into the cents they're really worth.
//
// A payment snaps to the currency's cash grid, as it always has. Then each
// such account's entries are added up in the currency itself and converted
// to cents once: the cents of each entry are the step that entry makes in
// the running total. Converting every entry on its own and adding the cents
// could be a cent out: LL 200,000 paid (222¢) less LL 60,000 change (67¢)
// gave 155¢ for the LL 140,000 kept, which is 156¢, so a $41.56 sale
// recorded $41.55 of payments (SA/CT/2026/03167, docs/FIXED_BUGS.md).
//
// That needs the LBP of each change entry (AmountUnits); an account with a
// change entry that lacks it, or whose units don't give its cents, keeps
// the entries' own cents (older callers, remote commands).
func normalizeCashRoundedPayments(ctx context.Context, q *db.Queries, payments []checkoutPayment, roundingMode string) (bool, error) {
	type cashAccount struct {
		currency db.Currency
		indexes  []int
	}
	accounts := make(map[int64]*cashAccount)
	var order []int64
	for i := range payments {
		payment := &payments[i]
		if payment.AmountCents == 0 {
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
		cash := accounts[payment.AccountID]
		if cash == nil {
			cash = &cashAccount{currency: currency}
			accounts[payment.AccountID] = cash
			order = append(order, payment.AccountID)
		}
		cash.indexes = append(cash.indexes, i)
	}

	changed := false
	for _, accountID := range order {
		cash := accounts[accountID]
		entries := make([]cashEntry, len(cash.indexes))
		for j, i := range cash.indexes {
			entries[j] = cashEntry{Cents: payments[i].AmountCents, Units: payments[i].AmountUnits}
		}
		cents, err := settleCashEntryCents(entries, cash.currency.UnitsPerUsdMicros, cash.currency.CashRoundingUnit, roundingMode)
		if err != nil {
			return false, fmt.Errorf("round cash payment: %w", err)
		}
		for j, i := range cash.indexes {
			if payments[i].AmountCents != cents[j] {
				payments[i].AmountCents = cents[j]
				changed = true
			}
		}
	}
	return changed, nil
}

// cashEntry is one payment (Cents > 0) or change (Cents < 0) entry in a
// cash-rounded currency, with its unsigned amount in that currency's units
// (0 when the caller didn't send it).
type cashEntry struct {
	Cents int64 `json:"cents"`
	Units int64 `json:"units"`
}

// settleCashEntryCents is the cents each of one account's entries records;
// see normalizeCashRoundedPayments. Payments are snapped to the cash grid,
// from their LL when they carry it and from their cents otherwise. When every change entry carries units that give its
// cents, the entries are netted in units and the result is converted once,
// so the cents add up to exactly the cents of the currency kept. Otherwise
// each payment is snapped on its own and change keeps its cents.
func settleCashEntryCents(entries []cashEntry, unitsPerUsdMicros, cashRoundingUnit int64, mode string) ([]int64, error) {
	out := make([]int64, len(entries))
	signedUnits := make([]int64, len(entries))
	netted := true
	for i, entry := range entries {
		out[i] = entry.Cents
		switch {
		case entry.Cents > 0:
			units, err := roundCurrencyUnitsFromCents(entry.Cents, unitsPerUsdMicros, cashRoundingUnit, mode)
			if err != nil {
				return nil, err
			}
			// Snap the LL itself when it came with the entry: going through
			// the cents first can land a note on the wrong side of the grid
			// under round down/up (LL 140,000 -> 155¢ -> LL 130,000).
			if entry.Units > 0 {
				if cents, err := roundCentsFromCurrencyUnits(entry.Units, unitsPerUsdMicros, mode); err == nil && cents == entry.Cents {
					units = roundRatio(entry.Units, cashRoundingUnit, mode) * cashRoundingUnit
				}
			}
			signedUnits[i] = units
			if out[i], err = roundCentsFromCurrencyUnits(units, unitsPerUsdMicros, mode); err != nil {
				return nil, err
			}
		case entry.Cents < 0:
			if entry.Units <= 0 {
				netted = false
				continue
			}
			cents, err := roundCentsFromCurrencyUnits(entry.Units, unitsPerUsdMicros, mode)
			if err != nil || cents != -entry.Cents {
				netted = false
				continue
			}
			signedUnits[i] = -entry.Units
		}
	}
	if !netted {
		return out, nil
	}
	var running, before int64
	for i := range entries {
		if entries[i].Cents == 0 {
			out[i] = 0
			continue
		}
		next := running + signedUnits[i]
		if (signedUnits[i] > 0 && next < running) || (signedUnits[i] < 0 && next > running) {
			return nil, fmt.Errorf("currency value is too large")
		}
		running = next
		after, err := signedCentsFromCurrencyUnits(running, unitsPerUsdMicros, mode)
		if err != nil {
			return nil, err
		}
		out[i] = after - before
		before = after
	}
	return out, nil
}

// signedCentsFromCurrencyUnits is roundCentsFromCurrencyUnits for an amount
// that may be negative (change given back), rounding its size.
func signedCentsFromCurrencyUnits(units, unitsPerUsdMicros int64, mode string) (int64, error) {
	cents, err := roundCentsFromCurrencyUnits(absInt64(units), unitsPerUsdMicros, mode)
	if units < 0 {
		cents = -cents
	}
	return cents, err
}

func parseSalesItems(raw, unknownBarcode, unknownCode string) ([]salesInvoiceItem, error) {
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
		// A product sold as Summer and as Winter is two lines: each option
		// has its own discount.
		key += "|" + item.SaleOption
		// Every "Unknown item" is its own line (each gets its own pictures),
		// so that one product may repeat; any other product may not. The
		// checkout payload carries only the code, so match on either.
		repeatable := item.Barcode == unknownBarcode || (item.Barcode == "" && item.Code == unknownCode)
		if (item.Code == "" && item.Barcode == "") || item.Qty <= 0 || math.Trunc(item.Qty) != item.Qty || item.Qty > math.MaxInt64 || (seen[key] && !repeatable) {
			return nil, fmt.Errorf("invalid or duplicate item %q", item.Code)
		}
		if item.SalespersonID < 0 {
			return nil, fmt.Errorf("invalid salesperson for item %q", item.Code)
		}
		if len([]rune(item.Note)) > 2000 {
			return nil, fmt.Errorf("note is too long for item %q", item.Code)
		}
		if len([]rune(strings.TrimSpace(item.Name))) > maxItemNameRunes {
			return nil, fmt.Errorf("name is too long for item %q", item.Code)
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
			if payment.AmountCents < 0 || payment.CouponTypeID <= 0 {
				return fmt.Errorf("coupon type is required")
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
			if couponType.CodeOptional == 0 && strings.TrimSpace(payment.CouponCode) == "" {
				return fmt.Errorf("coupon code is required")
			}
			if couponType.CodeOptional == 0 && couponType.ReferenceOnly == 0 {
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
				if coupon.ValidUntil.Valid && coupon.ValidUntil.String != "" {
					expires, parseErr := time.Parse("2006-01-02", coupon.ValidUntil.String)
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
			if clientErr != nil || isWalkInClient(client) {
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
			if couponType.ReferenceOnly != 0 || couponType.CodeOptional != 0 {
				continue
			}
			if err := q.RedeemCoupon(a.ctx, db.RedeemCouponParams{RedeemedInvoiceID: sql.NullInt64{Int64: invoiceID, Valid: true}, Code: code}); err != nil {
				return fmt.Errorf("redeem coupon: %w", err)
			}
		}
	}
	return nil
}
