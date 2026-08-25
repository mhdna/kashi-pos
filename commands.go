package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

	return nil
}
