package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// The printed invoice's own wording and the parts it leaves out
// (settings.receipt_texts, set from kashi's Receipt Designer). Every field
// is optional: empty is the default wording, and a till with nothing set
// prints exactly as before. The store name, description and exchange policy
// are their own settings; these are the rest.

// Sections a receipt can leave out.
const (
	receiptSectionPayments = "payments"
	receiptSectionPolicy   = "policy"
	receiptSectionReferral = "referral"
	receiptSectionSocials  = "socials"
	receiptSectionThanks   = "thanks"
)

var receiptSections = []string{receiptSectionPayments, receiptSectionPolicy, receiptSectionReferral, receiptSectionSocials, receiptSectionThanks}

// receiptTexts is the stored JSON.
type receiptTexts struct {
	PaymentsTitle string   `json:"payments_title,omitempty"`
	PolicyTitle   string   `json:"policy_title,omitempty"`
	ReferralTitle string   `json:"referral_title,omitempty"`
	ReferralNote  string   `json:"referral_note,omitempty"`
	Thanks        string   `json:"thanks,omitempty"`
	Hide          []string `json:"hide,omitempty"`
}

const (
	defaultPaymentsTitle = "PAYMENT DETAILS"
	defaultPolicyTitle   = "EXCHANGE POLICY"
	defaultReferralTitle = "YOUR CODE:"
	// {percent} is the reward, like "15%".
	defaultReferralNote = "Give this code to a friend. When they shop with us for the first time, you get {percent} off your next visit."
	receiptTextMaxLen   = 300
)

// parseReceiptTexts reads the stored JSON. Anything it can't read is the
// defaults: a bad setting never stops a receipt.
func parseReceiptTexts(raw string) receiptTexts {
	var t receiptTexts
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &t) != nil {
		return receiptTexts{}
	}
	return t
}

// normalizeReceiptTexts trims every text, cuts it to receiptTextMaxLen
// characters and keeps only known sections, once each.
func normalizeReceiptTexts(t receiptTexts) receiptTexts {
	clean := func(s string) string {
		s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
		if utf8.RuneCountInString(s) > receiptTextMaxLen {
			s = string([]rune(s)[:receiptTextMaxLen])
		}
		return s
	}
	out := receiptTexts{
		PaymentsTitle: clean(t.PaymentsTitle), PolicyTitle: clean(t.PolicyTitle),
		ReferralTitle: clean(t.ReferralTitle), ReferralNote: clean(t.ReferralNote), Thanks: clean(t.Thanks),
	}
	seen := map[string]bool{}
	for _, section := range receiptSections {
		for _, h := range t.Hide {
			if strings.TrimSpace(h) == section && !seen[section] {
				seen[section] = true
				out.Hide = append(out.Hide, section)
			}
		}
	}
	return out
}

func (t receiptTexts) hidden(section string) bool {
	for _, h := range t.Hide {
		if h == section {
			return true
		}
	}
	return false
}

func (t receiptTexts) paymentsTitle() string { return orDefault(t.PaymentsTitle, defaultPaymentsTitle) }
func (t receiptTexts) policyTitle() string   { return orDefault(t.PolicyTitle, defaultPolicyTitle) }
func (t receiptTexts) referralTitle() string { return orDefault(t.ReferralTitle, defaultReferralTitle) }

// referralNote is the line under the code, with the reward put in.
func (t receiptTexts) referralNote(percent int64) string {
	return strings.ReplaceAll(orDefault(t.ReferralNote, defaultReferralNote), "{percent}", fmt.Sprintf("%d%%", percent))
}

// thanks is the last line: the branch's own, else the theme's, else the
// default.
func (t receiptTexts) thanks(themeThanks string) string {
	return orDefault(t.Thanks, orDefault(themeThanks, "Thank you for shopping with us!"))
}

// SetReceiptTexts stores the receipt's wording from central's
// update_settings; raw is the JSON, '' for every default.
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
