package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	db "kashi-pos/db/sqlc"
)

// branchSettingsPayload mirrors branchSettingsRequest in kashi's
// api/branch_settings.go. It carries the per-branch config kashi mirrors so
// an admin can view and remotely edit it: business identity, behaviour
// feature-flags, non-admin page-access locks, printing, and the attendance
// device config.
//
// Still never leaves this install: api_key, theme, counter/register number,
// and the attendance secrets (akuvox_password, approver_password_hash) --
// kashi has no column for them and an update_settings command can't touch
// what it can't name.
type branchSettingsPayload struct {
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
	// SearchButtonEnabled is the one original field kashi is allowed to
	// write back as well as read (productSearchEnabledFlag in products.go).
	SearchButtonEnabled bool `json:"search_button_enabled"`

	// behaviour feature flags (feature_flags rows)
	CustomItemDiscountsEnabled    bool `json:"custom_item_discounts_enabled"`
	CustomItemPricesEnabled       bool `json:"custom_item_prices_enabled"`
	PerUnitItemPricesEnabled      bool `json:"per_unit_item_prices_enabled"`
	PriceChangeManualOverrideMode bool `json:"price_change_manual_override_mode"`
	InvoiceKeyboardMode           bool `json:"invoice_keyboard_mode"`
	ClientRequired                bool `json:"client_required"`

	// non-admin page-access locks (page_unlock_* flags)
	PageUnlockClients      bool `json:"page_unlock_clients"`
	PageUnlockInventory    bool `json:"page_unlock_inventory"`
	PageUnlockTransfers    bool `json:"page_unlock_transfers"`
	PageUnlockAttendance   bool `json:"page_unlock_attendance"`
	PageUnlockSalespersons bool `json:"page_unlock_salespersons"`

	// printing (settings columns)
	PrinterSize      string  `json:"printer_size"`
	ReceiptWidth     int64   `json:"receipt_width"`
	ReceiptHeight    int64   `json:"receipt_height"`
	ReceiptEnabled   bool    `json:"receipt_enabled"`
	ReceiptFont      string  `json:"receipt_font"`
	ReceiptBodyFont  string  `json:"receipt_body_font"`
	ReceiptTitleSize float64 `json:"receipt_title_size"`
	ReceiptBodySize  float64 `json:"receipt_body_size"`
	ReceiptCutoff    bool    `json:"receipt_cutoff"`

	// per-machine device identifiers -- reported for display only; the
	// admin UI defaults these to "managed at the branch".
	PrinterID      string `json:"printer_id"`
	ReceiptPrinter string `json:"receipt_printer"`
	ScreenPort     string `json:"screen_port"`

	// attendance device config (attendance_config row + one flag)
	AkuvoxIP                           string `json:"akuvox_ip"`
	AkuvoxUsername                     string `json:"akuvox_username"`
	AttendanceEnabled                  bool   `json:"attendance_enabled"`
	AttendanceDuplicateIntervalSeconds int64  `json:"attendance_duplicate_interval_seconds"`
	AttendanceCashierHistory           bool   `json:"attendance_cashier_history"`
}

// flagEnabled reads a feature_flags row as a bool, treating any read error
// (missing row, no DB) as "off" -- the same lenient handling every other
// GetFeatureFlag caller in this codebase uses.
func (a *App) flagEnabled(ctx context.Context, key string) bool {
	v, err := a.store.GetFeatureFlag(ctx, key)
	return err == nil && v != 0
}

// setFeatureFlagBool upserts a feature_flags row. Used for the behaviour
// flags that have no dedicated App setter of their own.
func (a *App) setFeatureFlagBool(ctx context.Context, key string, enabled bool) error {
	return a.store.SetFeatureFlag(ctx, db.SetFeatureFlagParams{Key: key, Enabled: boolInt(enabled)})
}

// pushSettings is called every sync tick (see runSyncTick in
// sync_worker.go), not just on save, so kashi converges on this branch's
// current settings even if a push was missed while offline -- there's no
// outbox/idempotency tracking needed since it's a plain upsert, not an
// event to deduplicate.
func (a *App) pushSettings(ctx context.Context, client *centralClient) error {
	s, err := a.store.GetSettings(ctx)
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	socialPlatforms := json.RawMessage(s.SocialPlatforms)
	if len(socialPlatforms) == 0 {
		socialPlatforms = json.RawMessage("[]")
	}
	socialHandles := json.RawMessage(s.SocialHandles)
	if len(socialHandles) == 0 {
		socialHandles = json.RawMessage("{}")
	}

	// Attendance device config -- ip/username/enabled/interval only; the
	// akuvox password never leaves the till.
	akuvoxIP, akuvoxUsername, _, attendanceEnabled, dupInterval, acErr := a.attendanceConfig()
	if acErr != nil {
		// A till that has never opened the attendance screen may have no
		// row yet; report zeros rather than failing the whole push.
		akuvoxIP, akuvoxUsername, attendanceEnabled, dupInterval = "", "", false, 0
	}

	payload := branchSettingsPayload{
		BranchName:          s.BranchName,
		TaxRate:             s.TaxRate,
		RoundingMode:        s.RoundingMode,
		RoundingCurrency:    s.RoundingCurrency,
		ExchangeRate:        s.ExchangeRate,
		ExchangeWindowHours: s.ExchangeWindowHours,
		MarketName:          s.MarketName,
		MarketPhone:         s.MarketPhone,
		MarketDescription:   s.MarketDescription,
		ReturnPolicy:        s.ReturnPolicy,
		Website:             s.Website,
		Instagram:           s.Instagram,
		SocialPlatforms:     socialPlatforms,
		SocialHandles:       socialHandles,
		SearchButtonEnabled: a.flagEnabled(ctx, productSearchEnabledFlag),

		CustomItemDiscountsEnabled:    a.flagEnabled(ctx, customItemDiscountsFlag),
		CustomItemPricesEnabled:       a.flagEnabled(ctx, customItemPricesFlag),
		PerUnitItemPricesEnabled:      a.flagEnabled(ctx, perUnitItemPricesFlag),
		PriceChangeManualOverrideMode: a.flagEnabled(ctx, priceChangeManualOverrideFlag),
		InvoiceKeyboardMode:           a.flagEnabled(ctx, invoiceKeyboardModeFlag),
		ClientRequired:                a.flagEnabled(ctx, clientRequiredFlag),

		PageUnlockClients:      a.flagEnabled(ctx, pageUnlockFlag("clients")),
		PageUnlockInventory:    a.flagEnabled(ctx, pageUnlockFlag("inventory")),
		PageUnlockTransfers:    a.flagEnabled(ctx, pageUnlockFlag("transfers")),
		PageUnlockAttendance:   a.flagEnabled(ctx, pageUnlockFlag("attendance")),
		PageUnlockSalespersons: a.flagEnabled(ctx, pageUnlockFlag("salespersons")),

		PrinterSize:      s.PrinterSize,
		ReceiptWidth:     s.ReceiptWidth,
		ReceiptHeight:    s.ReceiptHeight,
		ReceiptEnabled:   s.ReceiptEnabled != 0,
		ReceiptFont:      s.ReceiptFont,
		ReceiptBodyFont:  s.ReceiptBodyFont,
		ReceiptTitleSize: s.ReceiptTitleSize,
		ReceiptBodySize:  s.ReceiptBodySize,
		ReceiptCutoff:    s.ReceiptCutoff != 0,

		PrinterID:      strings.TrimSpace(s.PrinterID),
		ReceiptPrinter: strings.TrimSpace(s.ReceiptPrinter),
		ScreenPort:     strings.TrimSpace(s.ScreenPort),

		AkuvoxIP:                           akuvoxIP,
		AkuvoxUsername:                     akuvoxUsername,
		AttendanceEnabled:                  attendanceEnabled,
		AttendanceDuplicateIntervalSeconds: dupInterval,
		AttendanceCashierHistory:           a.flagEnabled(ctx, attendanceCashierHistoryFlag),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal settings payload: %w", err)
	}

	if err := client.PutSettings(ctx, body); err != nil {
		return fmt.Errorf("push settings: %w", err)
	}
	return nil
}
