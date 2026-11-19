package main

import (
	"context"
	"database/sql"

	db "kashi-pos/db/sqlc"
)

// invoiceLBPTotal is the LBP total the till shows and prints for an invoice
// total in cents -- the LBP currency's rate rounded to its cash unit by the
// branch's rounding mode, the same roundCurrencyUnitsFromCents the receipt
// uses -- plus that rate in LBP per USD. A refund rounds its size and keeps
// its sign. ok is false when there's no usable LBP currency.
func invoiceLBPTotal(ctx context.Context, q *db.Queries, totalCents int64, roundingMode string) (total, rate int64, ok bool) {
	lbp, err := q.GetCurrencyByCode(ctx, "LBP")
	if err != nil || lbp.UnitsPerUsdMicros <= 0 || lbp.CashRoundingUnit <= 0 {
		return 0, 0, false
	}
	units, err := roundCurrencyUnitsFromCents(absInt64(totalCents), lbp.UnitsPerUsdMicros, lbp.CashRoundingUnit, roundingMode)
	if err != nil {
		return 0, 0, false
	}
	if totalCents < 0 {
		units = -units
	}
	return units, lbp.UnitsPerUsdMicros / unitsPerUsdMicroScale, true
}

// stampInvoiceLBPTotal records invoiceLBPTotal on the invoice at checkout
// and returns it for the sync payload (nil, nil when there's none).
func stampInvoiceLBPTotal(ctx context.Context, q *db.Queries, invoiceID, totalCents int64, roundingMode string) (*int64, *int64, error) {
	total, rate, ok := invoiceLBPTotal(ctx, q, totalCents, roundingMode)
	if !ok {
		return nil, nil, nil
	}
	if err := q.SetInvoiceLBPTotal(ctx, db.SetInvoiceLBPTotalParams{
		LbpTotal: sql.NullInt64{Int64: total, Valid: true},
		LbpRate:  sql.NullInt64{Int64: rate, Valid: true},
		ID:       invoiceID,
	}); err != nil {
		return nil, nil, err
	}
	return &total, &rate, nil
}

// storedInvoiceLBPTotal is what stampInvoiceLBPTotal recorded, for a
// payload rebuilt later (backfill); nil for an invoice from before it.
func storedInvoiceLBPTotal(inv db.Invoice) (*int64, *int64) {
	if !inv.LbpTotal.Valid || !inv.LbpRate.Valid {
		return nil, nil
	}
	total, rate := inv.LbpTotal.Int64, inv.LbpRate.Int64
	return &total, &rate
}

// paymentLBPUnits is an LBP payment as the cashier counted it, from the USD
// cents it was stored as and the invoice's rate (LBP per USD): the nearest
// 1,000 L.L., since a cent (about 900 L.L.) loses the rest and cash comes in
// thousands. 60,000 L.L. stored as 67 cents at 89,500 is 59,965 before the
// rounding. A refund keeps its sign. ok is false without a rate.
func paymentLBPUnits(cents, rate int64) (units int64, ok bool) {
	if rate <= 0 {
		return 0, false
	}
	exact := absInt64(cents) * rate // LBP x 100
	units = (exact + 50_000) / 100_000 * 1_000
	if cents < 0 {
		units = -units
	}
	return units, true
}
