package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	db "kashi-pos/db/sqlc"
)

// Editing a return or exchange after the fact, from kashi (command
// edit_return_invoice): the credit of its returned lines is set to a new
// amount, e.g. a $15 hoodie credited -$12 (a deal charged back $3) is given
// its full -$15. The return is changed in place, with the money that moves:
//
//   - each edited line's line_total;
//   - the invoice's total, by the difference (delta = new - old);
//   - its payments: the difference goes through the account that settled
//     most of it (Cash $$ for a cash refund), entry and cashbox balance
//     included, in the return's own shift;
//   - its LBP total, a note saying what changed and why;
//   - a deal's recorded charge back, by what the edit gave back, so a later
//     exchange of the sale doesn't charge it again.
//
// It then syncs to kashi (return_invoice_edit), which applies the same to its
// copy. Setting a line to an amount is idempotent: replaying the command
// changes nothing the second time.

// editReturnLine names a line as kashi lists it: its position among the
// return's lines in the order they were recorded, and its barcode as a check.
type editReturnLine struct {
	Position  int    `json:"position"`
	Barcode   string `json:"barcode"`
	LineTotal int64  `json:"line_total"`
}

type editReturnInvoicePayload struct {
	ClientRef string           `json:"client_ref"`
	Lines     []editReturnLine `json:"lines"`
	Reason    string           `json:"reason"`
	EditedBy  string           `json:"edited_by"`
}

// branchReturnEditPayload is POSTed to kashi's /branch/return_invoice_edits
// (api/branch_return_edit.go there; keep the two in step).
type branchReturnEditPayload struct {
	EditRef    string                        `json:"edit_ref"`
	ClientRef  string                        `json:"client_ref"`
	GrandTotal int64                         `json:"grand_total"`
	LBPTotal   *int64                        `json:"lbp_total,omitempty"`
	LBPRate    *int64                        `json:"lbp_rate,omitempty"`
	Lines      []branchReturnEditLine        `json:"lines"`
	Payments   []branchInvoicePaymentPayload `json:"payments"`
	Notes      string                        `json:"notes"`
	Reason     string                        `json:"reason"`
	EditedBy   string                        `json:"edited_by"`
	OccurredAt time.Time                     `json:"occurred_at"`
}

type branchReturnEditLine struct {
	Position  int   `json:"position"`
	LineTotal int64 `json:"line_total"`
}

func (a *App) executeEditReturnInvoiceCommand(ctx context.Context, payload json.RawMessage) error {
	var p editReturnInvoicePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("decode edit_return_invoice payload: %w", err)
	}
	_, err := a.editReturnInvoice(ctx, p)
	return err
}

// editReturnInvoice applies the edit and answers the return's new total.
func (a *App) editReturnInvoice(ctx context.Context, p editReturnInvoicePayload) (int64, error) {
	if a.store == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	p.ClientRef = strings.TrimSpace(p.ClientRef)
	p.Reason = strings.TrimSpace(p.Reason)
	if p.ClientRef == "" {
		return 0, fmt.Errorf("client_ref is required")
	}
	if len(p.Lines) == 0 {
		return 0, fmt.Errorf("nothing to change")
	}
	invoiceID, err := a.store.GetInvoiceIDByClientRef(ctx, p.ClientRef)
	if err != nil {
		return 0, fmt.Errorf("invoice not found for client_ref %q: %w", p.ClientRef, err)
	}
	settings, err := a.store.GetSettings(ctx)
	if err != nil {
		return 0, fmt.Errorf("settings: %w", err)
	}
	roundingMode, err := normalizeRoundingMode(settings.RoundingMode)
	if err != nil {
		return 0, err
	}
	centralURL, err := a.syncCentralAPIURL()
	if err != nil {
		return 0, fmt.Errorf("settings: %w", err)
	}

	var newTotal int64
	changed := false
	err = a.store.ExecTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetReturnSaleID(ctx, invoiceID); err != nil {
			return fmt.Errorf("only a return or an exchange can be edited this way")
		}
		inv, err := q.GetInvoiceByID(ctx, invoiceID)
		if err != nil {
			return err
		}
		lines, err := q.ListInvoiceProductsInOrder(ctx, invoiceID)
		if err != nil {
			return err
		}

		var delta int64
		var notes []string
		seen := map[int]bool{}
		type change struct {
			line     db.InvoiceProduct
			newTotal int64
		}
		var changes []change
		for _, edit := range p.Lines {
			if edit.Position < 0 || edit.Position >= len(lines) {
				return fmt.Errorf("line %d is not on this return (it has %d lines)", edit.Position+1, len(lines))
			}
			if seen[edit.Position] {
				return fmt.Errorf("line %d is edited twice", edit.Position+1)
			}
			seen[edit.Position] = true
			line := lines[edit.Position]
			product, err := q.GetProductByID(ctx, line.ProductID)
			if err != nil {
				return fmt.Errorf("line %d: product: %w", edit.Position+1, err)
			}
			if b := strings.TrimSpace(edit.Barcode); b != "" && b != product.Barcode {
				return fmt.Errorf("line %d is %s, not %s; reload the invoice and try again", edit.Position+1, product.Barcode, b)
			}
			if line.Quantity >= 0 {
				return fmt.Errorf("line %d (%s) isn't a returned item; only what was given back can be changed", edit.Position+1, product.Name)
			}
			shelf := line.UnitPrice * line.Quantity // negative: the most a returned line can credit
			if edit.LineTotal > 0 {
				return fmt.Errorf("%s: a returned item's credit can't be a charge", product.Name)
			}
			if edit.LineTotal < shelf {
				return fmt.Errorf("%s: can't give back more than its price, %s", product.Name, formatUSDCents(-shelf))
			}
			if edit.LineTotal == line.LineTotal {
				continue
			}
			delta += edit.LineTotal - line.LineTotal
			changes = append(changes, change{line, edit.LineTotal})
			notes = append(notes, fmt.Sprintf("%s credit %s -> %s", product.Name, formatUSDCents(-line.LineTotal), formatUSDCents(-edit.LineTotal)))
		}
		newTotal = inv.GrandTotal
		if len(changes) == 0 {
			return nil
		}
		changed = true
		newTotal = inv.GrandTotal + delta

		for _, c := range changes {
			if err := q.SetInvoiceProductLineTotal(ctx, db.SetInvoiceProductLineTotalParams{LineTotal: c.newTotal, ID: c.line.ID, InvoiceID: invoiceID}); err != nil {
				return fmt.Errorf("set line: %w", err)
			}
			// More credit than before gives back part of what a deal charged
			// back on this return; the next exchange of the sale must see it.
			if given := c.line.LineTotal - c.newTotal; given > 0 && c.line.DealUid != "" {
				if err := q.ReduceExchangeDealClawback(ctx, db.ReduceExchangeDealClawbackParams{Clawback: given, InvoiceID: invoiceID, DealUid: c.line.DealUid}); err != nil {
					return fmt.Errorf("deal charge back: %w", err)
				}
			}
		}

		// The difference goes through the account that settled most of it.
		payments, err := q.ListInvoicePayments(ctx, invoiceID)
		if err != nil {
			return err
		}
		amounts := map[int64]int64{}
		account := inv.CashboxID
		var largest int64 = -1
		for _, pay := range payments {
			amounts[pay.CashboxID] += pay.Amount
			if abs := absInt64(pay.Amount); abs > largest {
				largest, account = abs, pay.CashboxID
			}
		}
		if account == 0 {
			return fmt.Errorf("this return has no payment account to give the difference through")
		}
		amounts[account] += delta
		if err := q.DeleteInvoicePayments(ctx, invoiceID); err != nil {
			return err
		}
		if err := q.DeleteInvoicePaymentEntries(ctx, invoiceID); err != nil {
			return err
		}
		for id, amount := range amounts {
			if amount == 0 {
				continue
			}
			if err := q.CreateInvoicePayment(ctx, db.CreateInvoicePaymentParams{InvoiceID: invoiceID, CashboxID: id, Amount: amount}); err != nil {
				return fmt.Errorf("record payment: %w", err)
			}
			entryType := "payment"
			if amount < 0 {
				entryType = "change"
			}
			if err := q.CreateInvoicePaymentEntry(ctx, db.CreateInvoicePaymentEntryParams{InvoiceID: invoiceID, CashboxID: id, Amount: amount, EntryType: entryType}); err != nil {
				return fmt.Errorf("record payment entry: %w", err)
			}
		}
		if _, err := q.AddCashboxAccountBalance(ctx, db.AddCashboxAccountBalanceParams{AccountID: account, ShiftID: inv.ShiftID, Balance: delta}); err != nil {
			return fmt.Errorf("cashbox balance: %w", err)
		}

		note := "Edited from kashi: " + strings.Join(notes, "; ")
		if p.EditedBy != "" {
			note += " (by " + strings.TrimSpace(p.EditedBy) + ")"
		}
		if p.Reason != "" {
			note += ". " + p.Reason
		}
		allNotes := strings.TrimSpace(strings.TrimSpace(inv.Notes) + "\n" + note)
		if err := q.SetReturnInvoiceTotals(ctx, db.SetReturnInvoiceTotalsParams{
			Subtotal: newTotal, DiscountedTotal: newTotal, GrandTotal: newTotal, Notes: allNotes, ID: invoiceID,
		}); err != nil {
			return fmt.Errorf("set totals: %w", err)
		}
		lbpTotal, lbpRate, err := stampInvoiceLBPTotal(ctx, q, invoiceID, newTotal, roundingMode)
		if err != nil {
			return fmt.Errorf("record LBP total: %w", err)
		}

		if centralURL == "" {
			return nil
		}
		syncLines := make([]branchReturnEditLine, 0, len(p.Lines))
		for _, edit := range p.Lines {
			syncLines = append(syncLines, branchReturnEditLine{Position: edit.Position, LineTotal: edit.LineTotal})
		}
		payPayload, err := buildBranchInvoicePayments(ctx, q, invoiceID)
		if err != nil {
			return err
		}
		editRef := uuid.NewString()
		body, err := json.Marshal(branchReturnEditPayload{
			EditRef: editRef, ClientRef: p.ClientRef, GrandTotal: newTotal,
			LBPTotal: lbpTotal, LBPRate: lbpRate,
			Lines: syncLines, Payments: payPayload, Notes: allNotes,
			Reason: p.Reason, EditedBy: strings.TrimSpace(p.EditedBy), OccurredAt: time.Now().UTC(),
		})
		if err != nil {
			return err
		}
		_, err = q.CreateSyncOutboxEntry(ctx, db.CreateSyncOutboxEntryParams{
			EntityType: "return_invoice_edit", ClientRef: editRef, PayloadJson: string(body),
		})
		return err
	})
	if err != nil {
		return 0, err
	}
	if changed && centralURL != "" {
		a.kickSyncWorker()
	}
	return newTotal, nil
}

// formatUSDCents is "$15.00" for 1500 (and "-$3.00" for -300).
func formatUSDCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s$%d.%02d", sign, cents/100, cents%100)
}
