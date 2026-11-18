package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SetReceiptTexts stores the receipt's wording from central's
// update_settings; raw is the JSON, ” for every default.
func (a *App) SetReceiptTexts(raw string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	stored := ""
	if strings.TrimSpace(raw) != "" {
		var t receiptTexts
		if err := json.Unmarshal([]byte(raw), &t); err != nil {
			return fmt.Errorf("receipt texts: %w", err)
		}
		t = normalizeReceiptTexts(t)
		if b, err := json.Marshal(t); err == nil && string(b) != "{}" {
			stored = string(b)
		}
	}
	if err := a.store.SetReceiptTexts(a.ctx, stored); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}
