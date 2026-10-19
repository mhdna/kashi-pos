package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

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
	BranchName       string  `json:"branch_name"`
	TaxRate          float64 `json:"tax_rate"`
	RoundingMode     string  `json:"rounding_mode"`
	RoundingCurrency string  `json:"rounding_currency"`
	// The two halves of the till's split total: SplitPrimaryCurrency leads,
	// RoundingCurrency follows, unless SplitPrimaryFirst says otherwise.
	SplitPrimaryCurrency string          `json:"split_primary_currency"`
	SplitPrimaryFirst    bool            `json:"split_primary_first"`
	ExchangeRate         int64           `json:"exchange_rate"`
	ExchangeWindowHours  int64           `json:"exchange_window_hours"`
	MarketName           string          `json:"market_name"`
	MarketPhone          string          `json:"market_phone"`
	MarketDescription    string          `json:"market_description"`
	ReturnPolicy         string          `json:"return_policy"`
	Website              string          `json:"website"`
	Instagram            string          `json:"instagram"`
	SocialPlatforms      json.RawMessage `json:"social_platforms"`
	SocialHandles        json.RawMessage `json:"social_handles"`
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
	SalespersonPerInvoiceRequired bool `json:"salesperson_per_invoice_required"`
	NonAdminMonthNavEnabled       bool `json:"non_admin_month_nav_enabled"`
	OpeningFloatEnabled           bool `json:"opening_float_enabled"`
	ShiftReportSignatures         bool `json:"shift_report_signatures"`
	ShiftReportUppercaseHeader    bool `json:"shift_report_uppercase_header"`
	AutoCoverNegativeLBP          bool `json:"auto_cover_negative_lbp"`
	CountOpeningCashOnClose       bool `json:"count_opening_cash_on_close"`
	OverchangeConfirmation        bool `json:"overchange_confirmation"`
	CloseShiftRequiresSync        bool `json:"close_shift_requires_sync"`
	ScanByCodeEnabled             bool `json:"scan_by_code_enabled"`
	ProductSearchCodeOnly         bool `json:"product_search_code_only"`
	PriceEditRestricted           bool `json:"price_edit_restricted"`
	SoundsEnabled                 bool `json:"sounds_enabled"`

	// Device numbers typed in here for kashi's salespersons, keyed by kashi's
	// salesperson id. kashi adopts them for any salesperson it hasn't set one
	// for (see applySyncedSalesperson for the other direction).
	SalespersonAttendanceIDs map[string]string `json:"salesperson_attendance_ids,omitempty"`

	// non-admin page-access locks (page_unlock_* flags)
	PageUnlockClients      bool `json:"page_unlock_clients"`
	PageUnlockInventory    bool `json:"page_unlock_inventory"`
	PageUnlockProducts     bool `json:"page_unlock_products"`
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
	// The printed invoice's look: theme id and that theme's layout id.
	ReceiptTheme  string `json:"receipt_theme"`
	ReceiptLayout string `json:"receipt_layout"`

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
	AttendanceManagerApprovalEnabled   bool   `json:"attendance_manager_approval_enabled"`

	OversellMode string `json:"oversell_mode"`
	// Comma-separated codes editable by hand while PriceEditRestricted is on.
	PriceEditCodes string `json:"price_edit_codes"`
	// Barcode of the product the "Unknown item" button adds; empty means the
	// built-in placeholder.
	UnknownProductBarcode string `json:"unknown_product_barcode"`
	// Product search columns left out, comma-separated.
	ProductSearchHiddenFields string `json:"product_search_hidden_fields"`

	HidePaymentMethodCurrency bool   `json:"hide_payment_method_currency"`
	AutoAppendLbpZeros        bool   `json:"auto_append_lbp_zeros"`
	AutoCloseStaleShifts      bool   `json:"auto_close_stale_shifts"`
	VisitorCounterEnabled     bool   `json:"visitor_counter_enabled"`
	ShiftReceiptUploadMode    int64  `json:"shift_receipt_upload_mode"`
	BranchPhone               string `json:"branch_phone"`
	MaxOverpaymentCents       int64  `json:"max_overpayment_cents"`
	DefaultOpeningFloatUsd    int64  `json:"default_opening_float_usd"`
	DefaultOpeningFloatLbp    int64  `json:"default_opening_float_lbp"`

	// The screen's display prefs as it last reported them (display_prefs.go).
	DisplayPrefs json.RawMessage `json:"display_prefs"`
}

// flagEnabled reads a feature_flags row as a bool, treating any read error
// (missing row, no DB) as "off" -- the same lenient handling every other
// GetFeatureFlag caller in this codebase uses.
func (a *App) flagEnabled(ctx context.Context, key string) bool {
	v, err := a.store.GetFeatureFlag(ctx, key)
	return err == nil && v != 0
}

// flagEnabledDefaultOn is flagEnabled for a flag the till treats as on
// until it's set (see GetAppSettings). Reading such a flag with flagEnabled
// reports a never-touched till as "off" while it behaves as "on", and kashi
// then shows -- and on the next save, pushes -- the wrong value.
func (a *App) flagEnabledDefaultOn(ctx context.Context, key string) bool {
	v, err := a.store.GetFeatureFlag(ctx, key)
	return err != nil || v != 0
}

// setFeatureFlagBool upserts a feature_flags row. Used for the behaviour
// flags that have no dedicated App setter of their own.
func (a *App) setFeatureFlagBool(ctx context.Context, key string, enabled bool) error {
	return a.store.SetFeatureFlag(ctx, db.SetFeatureFlagParams{Key: key, Enabled: boolInt(enabled)})
}

// settingsResendInterval re-sends an unchanged settings report this often
// anyway, so kashi's copy heals itself even if it was lost without the
// connection dropping (e.g. kashi's database was restored).
const settingsResendInterval = time.Hour

// settingsReport remembers the last settings report kashi accepted, so
// pushSettings only sends one when something changed. It's a report, not a
// sync: kashi stores it per register to show what each till runs and warn
// on drift, and answers with the till's branch code.
type settingsReport struct {
	mu     sync.Mutex
	sum    [sha256.Size]byte
	sentAt time.Time
}

// unchanged reports whether sum is what kashi last accepted, recently
// enough that re-sending it would tell kashi nothing new.
func (r *settingsReport) unchanged(sum [sha256.Size]byte, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.sentAt.IsZero() && r.sum == sum && now.Sub(r.sentAt) < settingsResendInterval
}

func (r *settingsReport) accepted(sum [sha256.Size]byte, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sum, r.sentAt = sum, now
}

// forget makes the next pushSettings send regardless -- called when a new
// WebSocket session starts, since kashi may have restarted meanwhile.
func (r *settingsReport) forget() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sentAt = time.Time{}
}

// pushSettings is called every pull tick (see runPullTick in
// sync_worker.go), so a change is reported within one tick however it was
// made -- the settings page, a remote update_settings command, a feature
// toggle -- but it only sends when the report differs from the one kashi
// last accepted (see settingsReport), on a new session, or hourly.
// salespersonAttendanceIDsForReport lists the device numbers set here for
// salespersons kashi knows (remote_id), keyed by kashi's id. A failed read
// just leaves them out of this report; the next one carries them.
func salespersonAttendanceIDsForReport(ctx context.Context, q *db.Queries) map[string]string {
	salespersons, err := q.ListSalespersons(ctx)
	if err != nil {
		return nil
	}
	ids := map[string]string{}
	for _, s := range salespersons {
		if s.RemoteID.Valid && strings.TrimSpace(s.AttendanceUserID) != "" {
			ids[strconv.FormatInt(s.RemoteID.Int64, 10)] = strings.TrimSpace(s.AttendanceUserID)
		}
	}
	return ids
}

func (a *App) pushSettings(ctx context.Context, client *centralClient) error {
	s, err := a.store.GetSettings(ctx)
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	receiptThemeID, receiptLayoutID := normalizeReceiptLook(s.ReceiptTheme, s.ReceiptLayout)
	socialPlatforms := json.RawMessage(s.SocialPlatforms)
	if len(socialPlatforms) == 0 {
		socialPlatforms = json.RawMessage("[]")
	}
	socialHandles := json.RawMessage(s.SocialHandles)
	if len(socialHandles) == 0 {
		socialHandles = json.RawMessage("{}")
	}
	// The screen reports its own prefs; the theme is stored in settings.theme
	// instead, so it's added here.
	prefs := map[string]any{}
	_ = json.Unmarshal([]byte(s.DisplayPrefs), &prefs)
	if s.Theme == "light" || s.Theme == "dark" {
		prefs["theme"] = s.Theme
	}
	displayPrefs, err := json.Marshal(prefs)
	if err != nil {
		return fmt.Errorf("marshal display prefs: %w", err)
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
		BranchName:           s.BranchName,
		TaxRate:              s.TaxRate,
		RoundingMode:         s.RoundingMode,
		RoundingCurrency:     s.RoundingCurrency,
		SplitPrimaryCurrency: s.SplitPrimaryCurrency,
		SplitPrimaryFirst:    s.SplitPrimaryFirst != 0,
		ExchangeRate:         s.ExchangeRate,
		ExchangeWindowHours:  s.ExchangeWindowHours,
		MarketName:           s.MarketName,
		MarketPhone:          s.MarketPhone,
		MarketDescription:    s.MarketDescription,
		ReturnPolicy:         s.ReturnPolicy,
		Website:              s.Website,
		Instagram:            s.Instagram,
		SocialPlatforms:      socialPlatforms,
		SocialHandles:        socialHandles,
		SearchButtonEnabled:  a.flagEnabledDefaultOn(ctx, productSearchEnabledFlag),

		CustomItemDiscountsEnabled:    a.flagEnabled(ctx, customItemDiscountsFlag),
		CustomItemPricesEnabled:       a.flagEnabled(ctx, customItemPricesFlag),
		PerUnitItemPricesEnabled:      a.flagEnabled(ctx, perUnitItemPricesFlag),
		PriceChangeManualOverrideMode: a.flagEnabled(ctx, priceChangeManualOverrideFlag),
		InvoiceKeyboardMode:           a.flagEnabled(ctx, invoiceKeyboardModeFlag),
		ClientRequired:                a.flagEnabledDefaultOn(ctx, clientRequiredFlag),
		SalespersonPerInvoiceRequired: a.flagEnabled(ctx, salespersonPerInvoiceRequiredFlag),
		NonAdminMonthNavEnabled:       a.flagEnabled(ctx, nonAdminMonthNavEnabledFlag),
		OpeningFloatEnabled:           a.openingFloatEnabled(),
		ShiftReportSignatures:         a.shiftReportSignatures(),
		ShiftReportUppercaseHeader:    a.shiftReportUppercaseHeader(),
		AutoCoverNegativeLBP:          a.autoCoverNegativeLBP(),
		CountOpeningCashOnClose:       a.countOpeningCashOnClose(),
		OverchangeConfirmation:        a.overchangeConfirmationEnabled(),
		CloseShiftRequiresSync:        a.closeShiftRequiresSync(),
		ScanByCodeEnabled:             a.scanByCodeEnabled(),
		ProductSearchCodeOnly:         a.flagEnabled(ctx, productSearchCodeOnlyFlag),
		PriceEditRestricted:           a.flagEnabled(ctx, priceEditRestrictedFlag),
		SoundsEnabled:                 a.soundsEnabled(),

		PageUnlockClients:      a.flagEnabled(ctx, pageUnlockFlag("clients")),
		PageUnlockInventory:    a.flagEnabled(ctx, pageUnlockFlag("inventory")),
		PageUnlockProducts:     a.flagEnabled(ctx, pageUnlockFlag("products")),
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
		ReceiptTheme:     receiptThemeID,
		ReceiptLayout:    receiptLayoutID,

		PrinterID:      strings.TrimSpace(s.PrinterID),
		ReceiptPrinter: strings.TrimSpace(s.ReceiptPrinter),
		ScreenPort:     strings.TrimSpace(s.ScreenPort),

		AkuvoxIP:                           akuvoxIP,
		AkuvoxUsername:                     akuvoxUsername,
		AttendanceEnabled:                  attendanceEnabled,
		AttendanceDuplicateIntervalSeconds: dupInterval,
		AttendanceCashierHistory:           a.flagEnabled(ctx, attendanceCashierHistoryFlag),
		AttendanceManagerApprovalEnabled:   a.attendanceManagerApprovalEnabled(),
		OversellMode:                       s.OversellMode,
		PriceEditCodes:                     s.PriceEditCodes,
		UnknownProductBarcode:              s.UnknownProductBarcode,
		ProductSearchHiddenFields:          s.ProductSearchHiddenFields,

		HidePaymentMethodCurrency: a.flagEnabled(ctx, hidePaymentMethodCurrencyFlag),
		AutoAppendLbpZeros:        a.flagEnabled(ctx, autoAppendLbpZerosFlag),
		AutoCloseStaleShifts:      a.flagEnabled(ctx, autoCloseStaleShiftsFlag),
		VisitorCounterEnabled:     a.flagEnabled(ctx, visitorCounterFlag),
		ShiftReceiptUploadMode:    a.shiftReceiptUploadMode(),
		BranchPhone:               s.BranchPhone,
		MaxOverpaymentCents:       s.MaxOverpaymentCents,
		DefaultOpeningFloatUsd:    s.DefaultOpeningFloatUsd,
		DefaultOpeningFloatLbp:    s.DefaultOpeningFloatLbp,
		DisplayPrefs:              displayPrefs,
	}
	payload.SalespersonAttendanceIDs = salespersonAttendanceIDsForReport(ctx, a.store.Queries)

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal settings payload: %w", err)
	}

	// The target server is part of the fingerprint: a report kashi A
	// accepted says nothing about kashi B.
	sum := sha256.Sum256(append([]byte(client.baseURL+"\n"), body...))
	if a.settingsReport.unchanged(sum, time.Now()) {
		return nil
	}

	branchCode, err := client.PutSettings(ctx, body)
	if err != nil {
		return fmt.Errorf("push settings: %w", err)
	}
	a.settingsReport.accepted(sum, time.Now())
	if branchCode != "" && branchCode != s.HomeBranchCode {
		if err := a.store.SetHomeBranchCode(ctx, branchCode); err != nil {
			return fmt.Errorf("save branch code: %w", err)
		}
	}
	return nil
}
