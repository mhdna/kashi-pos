package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	db "kashi-pos/db/sqlc"
)

// returnLine is one line of a sale and how much of it to take back.
type returnLine struct {
	line     db.InvoiceProduct
	quantity int64
}

func fullReturnLines(lines []db.InvoiceProduct) []returnLine {
	out := make([]returnLine, 0, len(lines))
	for _, line := range lines {
		out = append(out, returnLine{line: line, quantity: line.Quantity})
	}
	return out
}

// returnSalespersonID picks who a return is attributed to. The invoice header
// itself carries no salesperson (sales are recorded per line, see
// salesInvoiceTx) -- fall back to whoever sold the first line, then to this
// install's first configured salesperson, same "pick the first configured
// one" fallback exchangeCashbox uses for the settlement account.
func (a *App) returnSalespersonID(ctx context.Context, lines []db.InvoiceProduct) (int64, error) {
	if len(lines) > 0 && lines[0].SalespersonID > 0 {
		return lines[0].SalespersonID, nil
	}
	salespersons, err := a.store.ListSalespersons(ctx)
	if err != nil || len(salespersons) == 0 {
		return 0, fmt.Errorf("no salesperson configured to attribute the return to")
	}
	return salespersons[0].ID, nil
}

// returnItemsJSON builds the exchange request for taking lines back at the
// price and discount they were sold at.
func (a *App) returnItemsJSON(ctx context.Context, lines []returnLine) (string, error) {
	items := make([]exchangeLine, 0, len(lines))
	for _, lq := range lines {
		product, err := a.store.GetProductByID(ctx, lq.line.ProductID)
		if err != nil {
			return "", fmt.Errorf("load product for invoice line: %w", err)
		}
		unitPrice := lq.line.UnitPrice
		discount := lq.line.Discount
		items = append(items, exchangeLine{
			Code:           product.Code,
			Barcode:        product.Barcode,
			Qty:            -lq.quantity,
			UnitPriceCents: &unitPrice,
			UnitDiscount:   &discount,
		})
	}
	itemsJSON, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("build return items: %w", err)
	}
	return string(itemsJSON), nil
}

// CancelInvoice undoes a sale rung up by mistake -- a $3 item keyed in at
// $3000 -- without deleting anything. It checks out a full return of every
// line through the admin return path, so stock comes back, the refund goes
// through the drawer and both rows reach kashi like any other return; then
// both are stamped cancelled_at so sales statistics, here and in kashi,
// leave them out. Admin only, like an admin return.
func (a *App) CancelInvoice(invoiceID int64, reason string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if !a.currentUserIsAdmin() {
		return fmt.Errorf("only an admin can cancel an invoice")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "Cancelled"
	}
	return a.cancelInvoice(a.ctx, invoiceID, reason)
}

// cancelInvoice is CancelInvoice without the admin check, for kashi's remote
// cancel command (kashi already checked its own user's rights).
func (a *App) cancelInvoice(ctx context.Context, invoiceID int64, notes string) error {
	source, err := a.store.GetSalesInvoiceByID(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("only a sale can be cancelled: %w", err)
	}
	lines, err := a.store.ListInvoiceProducts(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("load invoice lines: %w", err)
	}
	if len(lines) == 0 {
		return fmt.Errorf("invoice has no items to cancel")
	}
	salespersonID, err := a.returnSalespersonID(ctx, lines)
	if err != nil {
		return err
	}
	itemsJSON, err := a.returnItemsJSON(ctx, fullReturnLines(lines))
	if err != nil {
		return err
	}
	// The refund goes back to the sale's own payment account. A sale split
	// across several accounts is refunded whole to its main one; the shift
	// report shows it there.
	_, err = a.checkoutReturnExchangeInvoice(itemsJSON, "", invoiceID, salespersonID, notes, source.CashboxID,
		exchangePolicy{allowLate: true, allowLowerTotal: true, returnOnly: true, cancellation: true})
	return err
}
