package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// openInvoiceExchangePayload is central's open_invoice_exchange (kashi
// api/exchange_opening.go): the sale may be exchanged on this till until the
// end of UntilDate, the shop's own day, past the branch's exchange window.
type openInvoiceExchangePayload struct {
	ClientRef string `json:"client_ref"`
	UntilDate string `json:"until_date"` // YYYY-MM-DD
	Reason    string `json:"reason"`
	OpenedBy  string `json:"opened_by"`
}

// exchangeOpenUntil is the last moment of untilDate on the till's clock. A
// date already gone (the command waited while the till was offline) opens
// nothing.
func exchangeOpenUntil(untilDate string, now time.Time, loc *time.Location) (time.Time, error) {
	day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(untilDate), loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("until_date must be a date like 2026-11-22")
	}
	// Not day+24h-1s: a daylight-saving change makes some days 23 or 25 hours.
	end := time.Date(day.Year(), day.Month(), day.Day(), 23, 59, 59, 0, loc)
	if now.After(end) {
		return time.Time{}, fmt.Errorf("%s has already passed on this till", day.Format("2 Jan 2006"))
	}
	return end.UTC(), nil
}

func (a *App) executeOpenInvoiceExchangeCommand(ctx context.Context, payload json.RawMessage) error {
	if a.store == nil || a.conn == nil {
		return fmt.Errorf("database not initialized")
	}
	var p openInvoiceExchangePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("decode open_invoice_exchange payload: %w", err)
	}
	ref := strings.TrimSpace(p.ClientRef)
	if ref == "" {
		return fmt.Errorf("client_ref is required")
	}
	until, err := exchangeOpenUntil(p.UntilDate, time.Now(), time.Local)
	if err != nil {
		return err
	}
	invoiceID, err := a.store.GetInvoiceIDByClientRef(ctx, ref)
	if err != nil {
		return fmt.Errorf("sale not found for client_ref %q: %w", ref, err)
	}
	invoice, err := a.store.GetSalesInvoiceByID(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("only a sale can be opened for exchange")
	}
	if invoice.CancelledAt.Valid {
		return fmt.Errorf("this sale was cancelled")
	}
	_, err = a.conn.ExecContext(ctx, `
		INSERT INTO exchange_openings (invoice_id, open_until, opened_by, reason, created_at)
		VALUES (?, ?, ?, ?, datetime('now'))
		ON CONFLICT(invoice_id) DO UPDATE SET
			open_until = excluded.open_until, opened_by = excluded.opened_by,
			reason = excluded.reason, created_at = excluded.created_at`,
		invoice.ID, until.Format(time.RFC3339), strings.TrimSpace(p.OpenedBy), strings.TrimSpace(p.Reason))
	if err != nil {
		return fmt.Errorf("open sale for exchange: %w", err)
	}
	return nil
}

// exchangeOpening is when central's opening of a sale ends, and whether it
// has one at all.
func (a *App) exchangeOpening(ctx context.Context, invoiceID int64) (time.Time, bool, error) {
	if a.conn == nil {
		return time.Time{}, false, nil
	}
	var raw string
	err := a.conn.QueryRowContext(ctx, `SELECT open_until FROM exchange_openings WHERE invoice_id = ?`, invoiceID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	until, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false, nil
	}
	return until, true, nil
}

// checkExchangeAllowed passes a sale still inside the exchange window, or
// one central opened for exchange that is still open.
func (a *App) checkExchangeAllowed(ctx context.Context, invoiceID int64, createdAt string, now time.Time, window time.Duration) error {
	windowErr := validateExchangeWindow(createdAt, now, window)
	if windowErr == nil {
		return nil
	}
	until, ok, err := a.exchangeOpening(ctx, invoiceID)
	if err != nil {
		return err
	}
	if ok && !now.After(until) {
		return nil
	}
	return windowErr
}
