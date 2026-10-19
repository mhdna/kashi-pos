package main

import (
	"encoding/base64"
	"fmt"

	db "kashi-pos/db/sqlc"
)

// receiptLook is the theme and layout ids the printed invoice uses, always a
// pair the renderer will honour.
func (a *App) receiptLook() (string, string) {
	if a.store == nil {
		return defaultReceiptTheme, defaultReceiptLayout
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return defaultReceiptTheme, defaultReceiptLayout
	}
	return normalizeReceiptLook(settings.ReceiptTheme, settings.ReceiptLayout)
}

// SetReceiptLook chooses the printed invoice's theme and layout. Called by
// the settings page and by an update_settings command from kashi.
func (a *App) SetReceiptLook(theme, layout string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	theme, layout = normalizeReceiptLook(theme, layout)
	if err := a.store.SetReceiptLook(a.ctx, db.SetReceiptLookParams{ReceiptTheme: theme, ReceiptLayout: layout}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

// GetReceiptLooks lists every theme with its layouts and says which pair is
// in use, for the settings page.
func (a *App) GetReceiptLooks() map[string]any {
	theme, layout := a.receiptLook()
	return map[string]any{
		"theme":          theme,
		"layout":         layout,
		"default_theme":  defaultReceiptTheme,
		"default_layout": defaultReceiptLayout,
		"themes":         receiptThemeCatalog(),
	}
}

// PreviewReceiptLook renders a sample sale in the given look as a PNG data
// URL. It uses the branch's own store name, phone, fonts, paper width,
// return policy and socials, so the preview is what this till would print.
func (a *App) PreviewReceiptLook(theme, layout string) (string, error) {
	base := printReceiptData{
		ReceiptWidth: 80, TitleSize: 21, BodySize: 11,
		BodyFont: "Printer Font A", Font: "Belleza",
	}
	if a.store != nil {
		if settings, err := a.store.GetSettings(a.ctx); err == nil {
			base.StoreName = settings.MarketName
			base.Branch = settings.BranchName
			base.Phone = settings.MarketPhone
			base.BranchPhone = settings.BranchPhone
			base.ReceiptWidth = settings.ReceiptWidth
			base.Font = settings.ReceiptFont
			base.BodyFont = settings.ReceiptBodyFont
			base.TitleSize = settings.ReceiptTitleSize
			base.BodySize = settings.ReceiptBodySize
			base.ReturnPolicy = settings.ReturnPolicy
			base.Website = settings.Website
			base.SocialPlatforms = decodeSocialPlatforms(settings.SocialPlatforms)
			base.SocialHandles = decodeSocialHandles(settings.SocialHandles)
		}
	}
	data, err := renderReceiptPreview(sampleReceiptData(base), theme, layout)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data), nil
}
