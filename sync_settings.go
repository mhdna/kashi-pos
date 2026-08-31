package main

import (
	"context"
	"encoding/json"
	"fmt"

	db "kashi-pos/db/sqlc"
)

// branchSettingsPayload mirrors branchSettingsRequest in kashi's
// api/branch_settings.go -- only the "business-identity" subset of the
// local settings table. Deliberately excludes hardware/device fields
// (printer, screen port, theme, counter number) and api_key: those never
// leave this install, kashi has no business modeling them centrally.
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
	// SearchButtonEnabled is the one field here kashi is allowed to write
	// back (via an update_settings command) as well as read -- see
	// productSearchEnabledFlag in products.go. It's stored locally as a
	// feature flag, not a settings column, so it's passed in separately
	// rather than read off db.Setting.
	SearchButtonEnabled bool `json:"search_button_enabled"`
}

// pushSettings is called every sync tick (see runSyncTick in
// sync_worker.go), not just on save, so kashi converges on this branch's
// current settings even if a push was missed while offline -- there's no
// outbox/idempotency tracking needed since it's a plain upsert, not an
// event to deduplicate.
func pushSettings(ctx context.Context, client *centralClient, s db.Setting, searchButtonEnabled bool) error {
	socialPlatforms := json.RawMessage(s.SocialPlatforms)
	if len(socialPlatforms) == 0 {
		socialPlatforms = json.RawMessage("[]")
	}
	socialHandles := json.RawMessage(s.SocialHandles)
	if len(socialHandles) == 0 {
		socialHandles = json.RawMessage("{}")
	}

	body, err := json.Marshal(branchSettingsPayload{
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
		SearchButtonEnabled: searchButtonEnabled,
	})
	if err != nil {
		return fmt.Errorf("marshal settings payload: %w", err)
	}

	if err := client.PutSettings(ctx, body); err != nil {
		return fmt.Errorf("push settings: %w", err)
	}
	return nil
}
