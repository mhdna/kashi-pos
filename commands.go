package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	db "kashi-pos/db/sqlc"
)

// runPendingCommandsOnce fetches every command kashi has queued for this
// branch and not yet acked, executes each in turn, and reports the outcome
// back. One command failing to execute doesn't block the others, and a
// decode/fetch failure here is logged and retried on the next tick or WS
// push -- there's no local queue to get stuck, kashi's branch_commands
// table (status='pending') is the only queue.
func (a *App) runPendingCommandsOnce(ctx context.Context, client *centralClient) {
	commands, err := client.FetchPendingCommands(ctx)
	if err != nil {
		log.Printf("sync commands: fetch pending: %v", err)
		return
	}

	for _, cmd := range commands {
		execErr := a.executeCommand(ctx, cmd)
		status := "success"
		if execErr != nil {
			status = "failed"
			log.Printf("sync commands: command %d (%s) failed: %v", cmd.ID, cmd.Type, execErr)
		}
		if err := client.AckCommand(ctx, cmd.ID, status, execErr); err != nil {
			log.Printf("sync commands: ack command %d: %v", cmd.ID, err)
		}
	}
}

// executeCommand dispatches by type to a real, validated local function --
// never raw SQL -- the whole point of routing settings edits through this
// channel instead of kashi writing to the branch's database directly.
func (a *App) executeCommand(ctx context.Context, cmd branchCommand) error {
	switch cmd.Type {
	case "update_settings":
		return a.executeUpdateSettingsCommand(ctx, cmd.Payload)
	case "remote_return_invoice":
		return a.executeRemoteReturnInvoiceCommand(ctx, cmd.Payload)
	default:
		return fmt.Errorf("unknown command type %q", cmd.Type)
	}
}

// updateSettingsCommandPayload mirrors branchSettingsPayload in
// sync_settings.go / branchSettingsRequest in kashi's api/branch_settings.go
// -- the same business-identity field set used for the read side, now used
// to build a write. Fields kashi has no opinion on (branch_code, api_key,
// printer/screen/theme config, receipt font/size, counter number) are left
// exactly as this install already has them -- a remote settings edit can
// never touch local-only/hardware fields, whether or not the sender even
// knows they exist.
type updateSettingsCommandPayload struct {
	BranchName          string          `json:"branch_name"`
	TaxRate             float64         `json:"tax_rate"`
	RoundingMode        string          `json:"rounding_mode"`
	RoundingCurrency    string          `json:"rounding_currency"`
	ExchangeRate        int64           `json:"exchange_rate"`
	ExchangeWindowHours int64           `json:"exchange_window_hours"`
	MarketName          string          `json:"market_name"`
	MarketPhone         string          `json:"market_phone"`
	MarketDescription   string          `json:"market_description"`
	ReturnPolicy        string          `json:"return_policy"`
	Website             string          `json:"website"`
	Instagram           string          `json:"instagram"`
	SocialPlatforms     json.RawMessage `json:"social_platforms"`
	SocialHandles       json.RawMessage `json:"social_handles"`
	// SearchButtonEnabled gates the sale screen's product-search button. A
	// pointer so an absent field (an older queued command, or any other
	// sender that doesn't know about it) leaves the local flag untouched
	// rather than forcing it false.
	SearchButtonEnabled *bool `json:"search_button_enabled"`
}

// executeUpdateSettingsCommand applies a remote settings edit by calling
// the exact same App methods the local Settings page's "Save" button calls
// -- UpdateSettings and UpdateReceiptSettings -- so the same validation
// (tax rate bounds, known/active rounding currency, positive exchange
// rate, branch code normalization) applies whether the edit came from the
// till or from kashi. Both methods take the *entire* settings row as
// positional args, so every field this command doesn't carry is read from
// the current row first and passed through unchanged.
func (a *App) executeUpdateSettingsCommand(ctx context.Context, payload json.RawMessage) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}

	var p updateSettingsCommandPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("decode update_settings payload: %w", err)
	}

	current, err := a.store.GetSettings(ctx)
	if err != nil {
		return fmt.Errorf("load current settings: %w", err)
	}

	if err := a.UpdateSettings(
		p.BranchName,
		current.BranchCode, // identity/routing, never remotely editable
		current.ApiKey,     // credential, never remotely editable
		current.PrinterID, current.PrinterSize, current.ReceiptPrinter, current.ScreenPort,
		current.ReceiptWidth, current.ReceiptHeight,
		p.TaxRate,
		p.RoundingMode,
		p.RoundingCurrency,
		current.ReceiptEnabled != 0,
		current.SalesInventoryID,
		p.ExchangeRate,
		current.MaxOverpaymentCents,
	); err != nil {
		return fmt.Errorf("update settings: %w", err)
	}

	socialPlatforms := string(p.SocialPlatforms)
	if len(p.SocialPlatforms) == 0 {
		socialPlatforms = current.SocialPlatforms
	}
	socialHandles := string(p.SocialHandles)
	if len(p.SocialHandles) == 0 {
		socialHandles = current.SocialHandles
	}

	if err := a.UpdateReceiptSettings(
		p.MarketName, p.MarketPhone, current.BranchPhone, p.MarketDescription, p.ReturnPolicy,
		p.Website, p.Instagram, socialPlatforms, socialHandles,
		current.CounterNumber, current.ReceiptFont, current.ReceiptBodyFont,
		current.ReceiptTitleSize, current.ReceiptBodySize, current.ReceiptCutoff != 0,
	); err != nil {
		return fmt.Errorf("update receipt/market settings: %w", err)
	}

	if err := a.SetExchangeWindowHours(p.ExchangeWindowHours); err != nil {
		return fmt.Errorf("update exchange window: %w", err)
	}

	if p.SearchButtonEnabled != nil {
		if err := a.SetProductSearchEnabled(*p.SearchButtonEnabled); err != nil {
			return fmt.Errorf("update search button: %w", err)
		}
	}

	return nil
}

// remoteReturnInvoiceCommandPayload mirrors the payload kashi's
// requestRemoteReturn builds (see ui/src/composables/useBranchInvoices.js and
// api/branch_command.go in the kashi repo): client_ref identifies the
// original sale to return by this install's own client_ref (not kashi's
// branch_invoice_id, which is only carried along for correlation/logging).
type remoteReturnInvoiceCommandPayload struct {
	ClientRef       string `json:"client_ref"`
	BranchInvoiceID int64  `json:"branch_invoice_id"`
	Reason          string `json:"reason"`
	// Items is optional -- kashi's admin UI sends the specific
	// products/quantities an operator picked to return. Empty/absent
	// preserves the old behavior (return every line at full quantity), so
	// any command already sitting in kashi's pending queue from before
	// item-level selection existed still executes the same way.
	// BranchProductID is this install's own local products.id -- kashi
	// only ever echoes it back, it never assigns or reinterprets it (see
	// branch_invoice_items in kashi's db/migrations).
	Items []remoteReturnInvoiceItemPayload `json:"items"`
}

type remoteReturnInvoiceItemPayload struct {
	BranchProductID int64 `json:"branch_product_id"`
	Quantity        int64 `json:"quantity"`
}

// executeRemoteReturnInvoiceCommand looks up the original sale by the
// client_ref kashi sent, builds a return of either the requested items
// (p.Items) or every line at full quantity (if none were specified), and
// checks it out through checkoutAdminReturnInvoice -- the same return path
// (and therefore the same validation, pricing, and branch-invoice-sync
// queuing) an admin-triggered return already uses locally. It never
// fabricates the return invoice outside that path.
func (a *App) executeRemoteReturnInvoiceCommand(ctx context.Context, payload json.RawMessage) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}

	var p remoteReturnInvoiceCommandPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("decode remote_return_invoice payload: %w", err)
	}
	p.ClientRef = strings.TrimSpace(p.ClientRef)
	if p.ClientRef == "" {
		return fmt.Errorf("client_ref is required")
	}

	invoiceID, err := a.store.GetInvoiceIDByClientRef(ctx, p.ClientRef)
	if err != nil {
		return fmt.Errorf("original invoice not found for client_ref %q: %w", p.ClientRef, err)
	}
	if _, err := a.store.GetSalesInvoiceByID(ctx, invoiceID); err != nil {
		return fmt.Errorf("load original invoice: %w", err)
	}
	lines, err := a.store.ListInvoiceProducts(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("load original invoice lines: %w", err)
	}
	if len(lines) == 0 {
		return fmt.Errorf("original invoice has no items to return")
	}

	// The invoice header itself carries no salesperson (sales are recorded
	// per line, see salesInvoiceTx) -- fall back to whoever sold the first
	// line, then to this install's first configured salesperson, same
	// "pick the first configured one" fallback exchangeCashbox already uses
	// for the settlement account below.
	salespersonID := lines[0].SalespersonID
	if salespersonID <= 0 {
		salespersons, err := a.store.ListSalespersons(ctx)
		if err != nil || len(salespersons) == 0 {
			return fmt.Errorf("no salesperson configured to attribute the return to")
		}
		salespersonID = salespersons[0].ID
	}

	type lineToReturn struct {
		line     db.InvoiceProduct
		quantity int64
	}
	var linesToReturn []lineToReturn
	if len(p.Items) > 0 {
		// Item-level selection: only the requested products, at the
		// requested (possibly partial) quantity each. Every requested line
		// must exist on the original invoice -- invoice_products' PK is
		// (invoice_id, product_id), so a product appears at most once,
		// there's no ambiguity in this match.
		linesToReturn = make([]lineToReturn, 0, len(p.Items))
		for _, requested := range p.Items {
			var found *db.InvoiceProduct
			for i := range lines {
				if lines[i].ProductID == requested.BranchProductID {
					found = &lines[i]
					break
				}
			}
			if found == nil {
				return fmt.Errorf("product %d is not on the original invoice", requested.BranchProductID)
			}
			if requested.Quantity <= 0 || requested.Quantity > found.Quantity {
				return fmt.Errorf("requested quantity %d for product %d exceeds the original line quantity %d", requested.Quantity, requested.BranchProductID, found.Quantity)
			}
			linesToReturn = append(linesToReturn, lineToReturn{line: *found, quantity: requested.Quantity})
		}
	} else {
		// No items specified -- fall back to returning every line at full
		// quantity, same as before item-level selection existed.
		linesToReturn = make([]lineToReturn, 0, len(lines))
		for _, line := range lines {
			linesToReturn = append(linesToReturn, lineToReturn{line: line, quantity: line.Quantity})
		}
	}

	items := make([]exchangeLine, 0, len(linesToReturn))
	for _, lq := range linesToReturn {
		product, err := a.store.GetProductByID(ctx, lq.line.ProductID)
		if err != nil {
			return fmt.Errorf("load product for invoice line: %w", err)
		}
		unitPrice := lq.line.UnitPrice
		discount := lq.line.Discount
		items = append(items, exchangeLine{
			Code:           product.Code,
			Qty:            -lq.quantity,
			UnitPriceCents: &unitPrice,
			UnitDiscount:   &discount,
		})
	}
	itemsJSON, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("build return items: %w", err)
	}

	accounts, err := a.store.ListCashboxAccounts(ctx)
	if err != nil || len(accounts) == 0 {
		return fmt.Errorf("no payment account configured for settlement")
	}

	notes := strings.TrimSpace(p.Reason)
	if notes == "" {
		notes = "Remote return"
	}

	_, err = a.checkoutAdminReturnInvoice(string(itemsJSON), invoiceID, salespersonID, accounts[0].ID, notes)
	if err != nil {
		return fmt.Errorf("checkout return: %w", err)
	}
	return nil
}
