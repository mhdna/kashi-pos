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
	case "set_branch_user_pin":
		return a.executeSetBranchUserPinCommand(ctx, cmd.Payload)
	case "print_shift_report":
		return a.executePrintShiftReportCommand(cmd.Payload)
	case "reprint_invoice":
		return a.executeReprintInvoiceCommand(ctx, cmd.Payload)
	case "sync_now":
		a.kickSyncWorker()
		return nil
	case "transfer_decision":
		return a.executeTransferDecisionCommand(ctx, cmd.Payload)
	case "review_attendance_request":
		return a.executeReviewAttendanceRequestCommand(cmd.Payload)
	default:
		return fmt.Errorf("unknown command type %q", cmd.Type)
	}
}

// updateSettingsCommandPayload mirrors branchSettingsPayload in
// sync_settings.go / branchSettingsRequest in kashi's api/branch_settings.go.
// Every field is a pointer so an absent one leaves the local value exactly
// as it is rather than zeroing it: kashi sends partial payloads (a
// register's own printer fields, the "apply defaults" subset), and older
// queued commands predate later fields.
//
// Still never carried, so never touched: branch_code, api_key, theme,
// counter number, and the POS admin login password.
type updateSettingsCommandPayload struct {
	BranchName       *string  `json:"branch_name"`
	TaxRate          *float64 `json:"tax_rate"`
	RoundingMode     *string  `json:"rounding_mode"`
	RoundingCurrency *string  `json:"rounding_currency"`
	// Pointers: added after the first version, so an older queued command
	// that predates them must leave the local split alone rather than blank
	// the primary currency and force the order back to default.
	SplitPrimaryCurrency *string `json:"split_primary_currency"`
	SplitPrimaryFirst    *bool   `json:"split_primary_first"`
	// The split's own denomination, independent of the primary currency's
	// cash_rounding_unit (see 000100). Nil leaves it as it is; 0 means "fall
	// back to cash_rounding_unit" and is a valid, explicit value.
	SplitPrimaryStep    *int64          `json:"split_primary_step"`
	ExchangeRate        *int64          `json:"exchange_rate"`
	ExchangeWindowHours *int64          `json:"exchange_window_hours"`
	MarketName          *string         `json:"market_name"`
	MarketPhone         *string         `json:"market_phone"`
	MarketDescription   *string         `json:"market_description"`
	ReturnPolicy        *string         `json:"return_policy"`
	Website             *string         `json:"website"`
	Instagram           *string         `json:"instagram"`
	SocialPlatforms     json.RawMessage `json:"social_platforms"`
	SocialHandles       json.RawMessage `json:"social_handles"`
	SearchButtonEnabled *bool           `json:"search_button_enabled"`

	// behaviour feature flags
	CustomItemDiscountsEnabled    *bool `json:"custom_item_discounts_enabled"`
	CustomItemPricesEnabled       *bool `json:"custom_item_prices_enabled"`
	PerUnitItemPricesEnabled      *bool `json:"per_unit_item_prices_enabled"`
	PriceChangeManualOverrideMode *bool `json:"price_change_manual_override_mode"`
	InvoiceKeyboardMode           *bool `json:"invoice_keyboard_mode"`
	ClientRequired                *bool `json:"client_required"`
	SalespersonPerInvoiceRequired *bool `json:"salesperson_per_invoice_required"`
	NonAdminMonthNavEnabled       *bool `json:"non_admin_month_nav_enabled"`
	OpeningFloatEnabled           *bool `json:"opening_float_enabled"`
	CountOpeningCashOnClose       *bool `json:"count_opening_cash_on_close"`
	OverchangeConfirmation        *bool `json:"overchange_confirmation"`
	CloseShiftRequiresSync        *bool `json:"close_shift_requires_sync"`
	ScanByCodeEnabled             *bool `json:"scan_by_code_enabled"`
	SoundsEnabled                 *bool `json:"sounds_enabled"`
	HidePaymentMethodCurrency     *bool `json:"hide_payment_method_currency"`
	AutoAppendLbpZeros            *bool `json:"auto_append_lbp_zeros"`
	AutoCloseStaleShifts          *bool `json:"auto_close_stale_shifts"`
	VisitorCounterEnabled         *bool `json:"visitor_counter_enabled"`

	// Cash handling. MaxOverpaymentCents caps how much a payment may exceed
	// the total; the default opening float is what a new shift starts with.
	BranchPhone            *string `json:"branch_phone"`
	MaxOverpaymentCents    *int64  `json:"max_overpayment_cents"`
	DefaultOpeningFloatUsd *int64  `json:"default_opening_float_usd"`
	DefaultOpeningFloatLbp *int64  `json:"default_opening_float_lbp"`

	// DisplayPrefs is a partial object of screen prefs (quick access rows,
	// card text size, theme, ...) -- see displayPrefKeys in display_prefs.go.
	DisplayPrefs json.RawMessage `json:"display_prefs"`

	// OversellMode: "allow" | "warn" | "block" (see checkOversell).
	OversellMode *string `json:"oversell_mode"`

	// ShiftReceiptUploadMode is an enum, not a flag: 0=off, 1=central,
	// 2=hybrid, 3=local (see shiftUploadMode* in shifts.go).
	ShiftReceiptUploadMode *int64 `json:"shift_receipt_upload_mode"`

	// non-admin page-access locks
	PageUnlockClients      *bool `json:"page_unlock_clients"`
	PageUnlockInventory    *bool `json:"page_unlock_inventory"`
	PageUnlockProducts     *bool `json:"page_unlock_products"`
	PageUnlockTransfers    *bool `json:"page_unlock_transfers"`
	PageUnlockAttendance   *bool `json:"page_unlock_attendance"`
	PageUnlockSalespersons *bool `json:"page_unlock_salespersons"`

	// printing
	PrinterSize      *string  `json:"printer_size"`
	ReceiptWidth     *int64   `json:"receipt_width"`
	ReceiptHeight    *int64   `json:"receipt_height"`
	ReceiptEnabled   *bool    `json:"receipt_enabled"`
	ReceiptFont      *string  `json:"receipt_font"`
	ReceiptBodyFont  *string  `json:"receipt_body_font"`
	ReceiptTitleSize *float64 `json:"receipt_title_size"`
	ReceiptBodySize  *float64 `json:"receipt_body_size"`
	ReceiptCutoff    *bool    `json:"receipt_cutoff"`

	// per-machine device identifiers
	PrinterID      *string `json:"printer_id"`
	ReceiptPrinter *string `json:"receipt_printer"`
	ScreenPort     *string `json:"screen_port"`

	// attendance device config
	AkuvoxIP                           *string `json:"akuvox_ip"`
	AkuvoxUsername                     *string `json:"akuvox_username"`
	AkuvoxPassword                     *string `json:"akuvox_password"`
	AttendanceApproverPassword         *string `json:"attendance_approver_password"`
	AttendanceEnabled                  *bool   `json:"attendance_enabled"`
	AttendanceDuplicateIntervalSeconds *int64  `json:"attendance_duplicate_interval_seconds"`
	AttendanceCashierHistory           *bool   `json:"attendance_cashier_history"`
	AttendanceManagerApprovalEnabled   *bool   `json:"attendance_manager_approval_enabled"`

	// AdminPassword force-sets the local "admin" account's password (plaintext
	// -- kashi's bcrypt hash is unusable here). Absent/empty = leave it alone.
	AdminPassword *string `json:"admin_password"`
}

func deref[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}

// executeUpdateSettingsCommand applies a remote settings edit by calling
// the exact same App methods the local Settings page's "Save" button calls
// -- so the same validation (tax rate bounds, known/active rounding
// currency, positive exchange rate, branch code normalization, etc.)
// applies whether the edit came from the till or from kashi. Anything the
// payload doesn't carry is read from the current row first and passed
// through unchanged.
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
		deref(p.BranchName, current.BranchName),
		current.BranchCode, // identity/routing, never remotely editable
		current.ApiKey,     // credential, never remotely editable
		deref(p.PrinterID, current.PrinterID),
		deref(p.PrinterSize, current.PrinterSize),
		deref(p.ReceiptPrinter, current.ReceiptPrinter),
		deref(p.ScreenPort, current.ScreenPort),
		deref(p.ReceiptWidth, current.ReceiptWidth),
		deref(p.ReceiptHeight, current.ReceiptHeight),
		deref(p.TaxRate, current.TaxRate),
		deref(p.RoundingMode, current.RoundingMode),
		deref(p.RoundingCurrency, current.RoundingCurrency),
		deref(p.ReceiptEnabled, current.ReceiptEnabled != 0),
		current.SalesInventoryID,
		deref(p.ExchangeRate, current.ExchangeRate),
		deref(p.MaxOverpaymentCents, current.MaxOverpaymentCents),
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
		deref(p.MarketName, current.MarketName),
		deref(p.MarketPhone, current.MarketPhone),
		deref(p.BranchPhone, current.BranchPhone),
		deref(p.MarketDescription, current.MarketDescription),
		deref(p.ReturnPolicy, current.ReturnPolicy),
		deref(p.Website, current.Website),
		deref(p.Instagram, current.Instagram),
		socialPlatforms, socialHandles,
		current.CounterNumber,
		deref(p.ReceiptFont, current.ReceiptFont),
		deref(p.ReceiptBodyFont, current.ReceiptBodyFont),
		deref(p.ReceiptTitleSize, current.ReceiptTitleSize),
		deref(p.ReceiptBodySize, current.ReceiptBodySize),
		deref(p.ReceiptCutoff, current.ReceiptCutoff != 0),
	); err != nil {
		return fmt.Errorf("update receipt/market settings: %w", err)
	}

	if p.ExchangeWindowHours != nil {
		if err := a.SetExchangeWindowHours(*p.ExchangeWindowHours); err != nil {
			return fmt.Errorf("update exchange window: %w", err)
		}
	}

	// All three move together or not at all: the query writes all three
	// columns, so a command naming only some still has to carry the rest
	// through unchanged rather than resetting them.
	if p.SplitPrimaryCurrency != nil || p.SplitPrimaryFirst != nil || p.SplitPrimaryStep != nil {
		if err := a.SetSplitDisplay(
			deref(p.SplitPrimaryCurrency, current.SplitPrimaryCurrency),
			deref(p.SplitPrimaryFirst, current.SplitPrimaryFirst != 0),
			deref(p.SplitPrimaryStep, current.SplitPrimaryStep),
		); err != nil {
			return fmt.Errorf("update split display: %w", err)
		}
	}

	// --- feature flags: only touch a flag the command actually carries ---
	flagUpdates := []struct {
		v  *bool
		fn func(bool) error
	}{
		{p.SearchButtonEnabled, a.SetProductSearchEnabled},
		{p.CustomItemDiscountsEnabled, func(b bool) error { return a.setFeatureFlagBool(ctx, customItemDiscountsFlag, b) }},
		{p.CustomItemPricesEnabled, func(b bool) error { return a.setFeatureFlagBool(ctx, customItemPricesFlag, b) }},
		{p.PerUnitItemPricesEnabled, func(b bool) error { return a.setFeatureFlagBool(ctx, perUnitItemPricesFlag, b) }},
		{p.PriceChangeManualOverrideMode, func(b bool) error { return a.setFeatureFlagBool(ctx, priceChangeManualOverrideFlag, b) }},
		{p.InvoiceKeyboardMode, func(b bool) error { return a.setFeatureFlagBool(ctx, invoiceKeyboardModeFlag, b) }},
		{p.ClientRequired, func(b bool) error { return a.setFeatureFlagBool(ctx, clientRequiredFlag, b) }},
		{p.SalespersonPerInvoiceRequired, func(b bool) error {
			return a.setFeatureFlagBool(ctx, salespersonPerInvoiceRequiredFlag, b)
		}},
		{p.NonAdminMonthNavEnabled, func(b bool) error {
			return a.setFeatureFlagBool(ctx, nonAdminMonthNavEnabledFlag, b)
		}},
		{p.OpeningFloatEnabled, func(b bool) error { return a.setFeatureFlagBool(ctx, openingFloatEnabledFlag, b) }},
		{p.CountOpeningCashOnClose, func(b bool) error { return a.setFeatureFlagBool(ctx, countOpeningCashOnCloseFlag, b) }},
		{p.OverchangeConfirmation, func(b bool) error { return a.setFeatureFlagBool(ctx, overchangeConfirmationFlag, b) }},
		{p.CloseShiftRequiresSync, a.SetCloseShiftRequiresSyncEnabled},
		{p.ScanByCodeEnabled, a.SetScanByCodeEnabled},
		{p.SoundsEnabled, a.SetSoundsEnabled},
		{p.HidePaymentMethodCurrency, a.SetHidePaymentMethodCurrency},
		{p.AutoAppendLbpZeros, a.SetAutoAppendLbpZerosEnabled},
		{p.AutoCloseStaleShifts, a.SetAutoCloseStaleShiftsEnabled},
		{p.VisitorCounterEnabled, a.SetVisitorCounterEnabled},
		{p.PageUnlockClients, func(b bool) error { return a.SetPageUnlocked("clients", b) }},
		{p.PageUnlockInventory, func(b bool) error { return a.SetPageUnlocked("inventory", b) }},
		{p.PageUnlockProducts, func(b bool) error { return a.SetPageUnlocked("products", b) }},
		{p.PageUnlockTransfers, func(b bool) error { return a.SetPageUnlocked("transfers", b) }},
		{p.PageUnlockAttendance, func(b bool) error { return a.SetPageUnlocked("attendance", b) }},
		{p.PageUnlockSalespersons, func(b bool) error { return a.SetPageUnlocked("salespersons", b) }},
		{p.AttendanceCashierHistory, a.SetAttendanceCashierHistoryEnabled},
		{p.AttendanceManagerApprovalEnabled, a.SetAttendanceManagerApprovalEnabled},
	}
	for _, u := range flagUpdates {
		if u.v == nil {
			continue
		}
		if err := u.fn(*u.v); err != nil {
			return fmt.Errorf("update feature flag: %w", err)
		}
	}

	if p.OversellMode != nil {
		if err := a.SetOversellMode(*p.OversellMode); err != nil {
			return fmt.Errorf("update oversell mode: %w", err)
		}
	}

	if p.ShiftReceiptUploadMode != nil {
		if err := a.SetShiftReceiptUploadMode(*p.ShiftReceiptUploadMode); err != nil {
			return fmt.Errorf("update shift receipt upload mode: %w", err)
		}
	}

	// Not through UpdateDefaultOpeningFloat: that asks for the manager
	// password a person at the till types, which kashi's authenticated
	// command channel stands in for. Its other rule (never negative) holds.
	if p.DefaultOpeningFloatUsd != nil || p.DefaultOpeningFloatLbp != nil {
		usd := deref(p.DefaultOpeningFloatUsd, current.DefaultOpeningFloatUsd)
		lbp := deref(p.DefaultOpeningFloatLbp, current.DefaultOpeningFloatLbp)
		if usd < 0 || lbp < 0 {
			return fmt.Errorf("opening float cannot be negative")
		}
		if err := a.store.UpdateDefaultOpeningFloat(ctx, db.UpdateDefaultOpeningFloatParams{
			DefaultOpeningFloatUsd: usd,
			DefaultOpeningFloatLbp: lbp,
		}); err != nil {
			return fmt.Errorf("update default opening float: %w", err)
		}
	}

	if len(p.DisplayPrefs) > 0 && string(p.DisplayPrefs) != "null" {
		if err := a.applyRemoteDisplayPrefs(p.DisplayPrefs); err != nil {
			return err
		}
	}

	// --- attendance device config ---
	if p.AkuvoxIP != nil || p.AkuvoxUsername != nil || p.AkuvoxPassword != nil || p.AttendanceEnabled != nil {
		curIP, curUser, _, curEnabled, _, acErr := a.attendanceConfig()
		if acErr != nil {
			curIP, curUser, curEnabled = "", "", false
		}
		if err := a.UpdateAttendanceConfig(
			deref(p.AkuvoxIP, curIP),
			deref(p.AkuvoxUsername, curUser),
			deref(p.AkuvoxPassword, ""), // "" tells UpdateAttendanceConfig to keep the current password
			deref(p.AttendanceEnabled, curEnabled),
		); err != nil {
			return fmt.Errorf("update attendance config: %w", err)
		}
	}
	if p.AttendanceDuplicateIntervalSeconds != nil {
		if err := a.SetAttendanceDuplicateInterval(*p.AttendanceDuplicateIntervalSeconds); err != nil {
			return fmt.Errorf("update attendance duplicate interval: %w", err)
		}
	}
	if p.AttendanceApproverPassword != nil && *p.AttendanceApproverPassword != "" {
		if err := a.UpdateAttendanceApproverPassword(*p.AttendanceApproverPassword); err != nil {
			return fmt.Errorf("update attendance approver password: %w", err)
		}
	}

	if p.AdminPassword != nil && *p.AdminPassword != "" {
		if err := a.ForceSetAdminPassword(*p.AdminPassword); err != nil {
			return fmt.Errorf("update admin password: %w", err)
		}
	}

	// Most setters above don't announce themselves; the open sale screen,
	// nav counter and the like reload on this.
	a.emitSettingsChanged()
	return nil
}

type printShiftReportPayload struct {
	// BranchShiftID is this till's own shifts.id, as it reported it in
	// shift_close (kashi's branch_shifts.branch_shift_id).
	BranchShiftID int64 `json:"branch_shift_id"`
}

// executePrintShiftReportCommand reprints a closed shift's end-of-shift
// receipt on this till's printer -- the same PrintClosedShiftReceipt the
// Shifts page's print button calls, so it prints exactly what that would.
func (a *App) executePrintShiftReportCommand(payload json.RawMessage) error {
	var p printShiftReportPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("decode print_shift_report payload: %w", err)
	}
	result, err := a.PrintClosedShiftReceipt(p.BranchShiftID)
	if err != nil {
		return err
	}
	if printed, _ := result["receiptPrinted"].(bool); !printed {
		return fmt.Errorf("receipt printing is turned off on this till")
	}
	return nil
}

type reprintInvoicePayload struct {
	ClientRef string `json:"client_ref"`
}

// executeReprintInvoiceCommand reprints a sale or return receipt, found by
// the client_ref this till gave it, through the same ReprintInvoice the
// invoice list uses.
func (a *App) executeReprintInvoiceCommand(ctx context.Context, payload json.RawMessage) error {
	var p reprintInvoicePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("decode reprint_invoice payload: %w", err)
	}
	p.ClientRef = strings.TrimSpace(p.ClientRef)
	if p.ClientRef == "" {
		return fmt.Errorf("client_ref is required")
	}
	invoiceID, err := a.store.GetInvoiceIDByClientRef(ctx, p.ClientRef)
	if err != nil {
		return fmt.Errorf("invoice not found for client_ref %q: %w", p.ClientRef, err)
	}
	return a.ReprintInvoice(invoiceID)
}

type setBranchUserPinPayload struct {
	Username string `json:"username"`
	Pin      string `json:"pin"`
}

// executeSetBranchUserPinCommand sets a synced POS user's PIN. kashi never
// sends the PIN in the roster feed (see applySyncedBranchUser) -- it comes
// here as plaintext over the authenticated command channel and is hashed
// locally. If the roster pull hasn't created the user yet, create it now
// with a cashier role; the next pull adopts it by username and fills in the
// real role / remote_id.
func (a *App) executeSetBranchUserPinCommand(ctx context.Context, payload json.RawMessage) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	var p setBranchUserPinPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("decode set_branch_user_pin payload: %w", err)
	}
	p.Username = strings.TrimSpace(p.Username)
	if p.Username == "" || strings.TrimSpace(p.Pin) == "" {
		return fmt.Errorf("username and pin are required")
	}

	rows, err := a.store.SetUserPasswordHashByUsername(ctx, db.SetUserPasswordHashByUsernameParams{
		PasswordHash: hashPassword(p.Pin),
		Username:     p.Username,
	})
	if err != nil {
		return fmt.Errorf("set pin: %w", err)
	}
	if rows > 0 {
		return nil
	}

	role, err := a.store.GetRoleByName(ctx, "cashier")
	if err != nil {
		return fmt.Errorf("resolve cashier role for new synced user: %w", err)
	}
	_, err = a.store.CreateUser(ctx, db.CreateUserParams{
		Username:     p.Username,
		PasswordHash: hashPassword(p.Pin),
		RoleID:       role.ID,
		Active:       1,
	})
	return err
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
	// Cancel asks for the whole sale to be cancelled (see cancelInvoice)
	// rather than returned; Items is ignored then.
	Cancel bool `json:"cancel"`
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

	if p.Cancel {
		notes := strings.TrimSpace(p.Reason)
		if notes == "" {
			notes = "Cancelled from kashi"
		}
		if err := a.cancelInvoice(ctx, invoiceID, notes); err != nil {
			return fmt.Errorf("cancel invoice: %w", err)
		}
		return nil
	}

	salespersonID, err := a.returnSalespersonID(ctx, lines)
	if err != nil {
		return err
	}

	var linesToReturn []returnLine
	if len(p.Items) > 0 {
		// Item-level selection: only the requested products, at the
		// requested (possibly partial) quantity each. Every requested line
		// must exist on the original invoice -- invoice_products' PK is
		// (invoice_id, product_id), so a product appears at most once,
		// there's no ambiguity in this match.
		linesToReturn = make([]returnLine, 0, len(p.Items))
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
			linesToReturn = append(linesToReturn, returnLine{line: *found, quantity: requested.Quantity})
		}
	} else {
		// No items specified -- fall back to returning every line at full
		// quantity, same as before item-level selection existed.
		linesToReturn = fullReturnLines(lines)
	}

	itemsJSON, err := a.returnItemsJSON(ctx, linesToReturn)
	if err != nil {
		return err
	}

	accounts, err := a.store.ListCashboxAccounts(ctx)
	if err != nil || len(accounts) == 0 {
		return fmt.Errorf("no payment account configured for settlement")
	}

	notes := strings.TrimSpace(p.Reason)
	if notes == "" {
		notes = "Remote return"
	}

	_, err = a.checkoutAdminReturnInvoice(itemsJSON, invoiceID, salespersonID, accounts[0].ID, notes)
	if err != nil {
		return fmt.Errorf("checkout return: %w", err)
	}
	return nil
}

type transferDecisionPayload struct {
	ClientRef string `json:"client_ref"`
	Status    string `json:"status"`
}

// executeTransferDecisionCommand records kashi's approve/cancel of a
// transfer this till proposed, the moment it's made -- the same status
// write the transfer_requests pull does (applySyncedTransferRequest), which
// stays as the fallback for a till that missed the command. A ref this till
// doesn't have fails the command so kashi shows it, rather than
// acknowledging a decision that went nowhere.
func (a *App) executeTransferDecisionCommand(ctx context.Context, payload json.RawMessage) error {
	var p transferDecisionPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("decode transfer_decision payload: %w", err)
	}
	p.ClientRef = strings.TrimSpace(p.ClientRef)
	if p.ClientRef == "" || p.Status == "" {
		return fmt.Errorf("client_ref and status are required")
	}
	if _, err := a.store.GetTransferRequestByClientRef(ctx, p.ClientRef); err != nil {
		return fmt.Errorf("transfer request %q not found on this till: %w", p.ClientRef, err)
	}
	if err := applySyncedTransferRequest(ctx, a.store.Queries, syncedTransferRequest{
		ClientRef: p.ClientRef,
		Status:    p.Status,
	}); err != nil {
		return fmt.Errorf("record transfer decision: %w", err)
	}
	a.emitSyncApplied("transfer_requests")
	return nil
}
