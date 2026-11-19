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
	// ReferralRewardTo is who the referral program rewards (owner, friend
	// or both), set when a receipt is drawn: the note's default wording
	// follows it. Not part of the branch's design.
	ReferralRewardTo string `json:"-"`

	PaymentsTitle string   `json:"payments_title,omitempty"`
	PolicyTitle   string   `json:"policy_title,omitempty"`
	ReferralTitle string   `json:"referral_title,omitempty"`
	ReferralNote  string   `json:"referral_note,omitempty"`
	SocialsTitle  string   `json:"socials_title,omitempty"`
	Thanks        string   `json:"thanks,omitempty"`
	Hide          []string `json:"hide,omitempty"`
	// Elements change single lines (receipt_design.go); DateFormat picks how
	// the date prints.
	Elements   map[string]receiptElement `json:"elements,omitempty"`
	DateFormat string                    `json:"date_format,omitempty"`
	// Groups change every line of a part at once (capitals, size, weight,
	// alignment); a line's own setting comes first.
	Groups map[string]receiptElement `json:"groups,omitempty"`
	// Decor is each part's lines and box (receipt_decor.go).
	Decor map[string]receiptDecor `json:"decor,omitempty"`
	// Order is the order of the parts under the totals; missing ones keep
	// their place after the ones named.
	Order []string `json:"order,omitempty"`
	// Banners are bands of display lettering (receipt_banner.go).
	Banners []receiptBanner `json:"banners,omitempty"`
}

const (
	defaultPaymentsTitle = "PAYMENT DETAILS"
	defaultPolicyTitle   = "EXCHANGE POLICY"
	defaultReferralTitle = "YOUR CODE:"
	defaultSocialsTitle  = "Follow us on:"
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
		ReferralTitle: clean(t.ReferralTitle), ReferralNote: clean(t.ReferralNote),
		SocialsTitle: clean(t.SocialsTitle), Thanks: clean(t.Thanks),
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
	out.Elements = normalizeReceiptElements(t.Elements, clean)
	out.DateFormat = validReceiptDateFormat(t.DateFormat)
	out.Groups = normalizeReceiptGroups(t.Groups)
	out.Decor = normalizeReceiptDecor(t.Decor)
	out.Order = normalizeReceiptOrder(t.Order)
	out.Banners = normalizeReceiptBanners(t.Banners)
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

// Each wording is its element's (receipt_design.go): the branch's own words
// in its capitals, else the default.
func (t receiptTexts) paymentsTitle() string { return t.word("payments_title", defaultPaymentsTitle) }
func (t receiptTexts) policyTitle() string   { return t.word("policy_title", defaultPolicyTitle) }
func (t receiptTexts) referralTitle() string { return t.word("referral_title", defaultReferralTitle) }
func (t receiptTexts) socialsTitle() string  { return t.word("socials_title", defaultSocialsTitle) }

// referralNote is the line under the code, with the reward put in.
func (t receiptTexts) referralNote(percent int64) string {
	return strings.ReplaceAll(t.word("referral_note", referralNoteDefault(t.ReferralRewardTo)), "{percent}", fmt.Sprintf("%d%%", percent))
}

// Who a referral code rewards (central's program).
const (
	referralRewardOwner  = "owner"
	referralRewardFriend = "friend"
	referralRewardBoth   = "both"
)

// referralNoteDefault is the note's wording for who the program rewards.
func referralNoteDefault(rewardTo string) string {
	switch rewardTo {
	case referralRewardBoth:
		return "Share this code with someone who hasn't shopped with us yet. When they use it, you both get {percent} off the whole invoice."
	case referralRewardFriend:
		return "Share this code with someone who hasn't shopped with us yet: they get {percent} off their first invoice."
	}
	return defaultReferralNote
}

// thanks is the last line: the branch's own, else the theme's, else the
// default.
func (t receiptTexts) thanks(themeThanks string) string {
	return t.word("thanks", orDefault(themeThanks, "Thank you for shopping with us!"))
}
