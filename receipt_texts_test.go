package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"image/png"
	"path/filepath"
	"strings"
	"testing"

	db "kashi-pos/db/sqlc"
)

func TestReceiptTextsDefaultsAndParsing(t *testing.T) {
	var none receiptTexts
	if none.paymentsTitle() != "PAYMENT DETAILS" || none.policyTitle() != "EXCHANGE POLICY" || none.referralTitle() != "YOUR CODE:" {
		t.Fatalf("defaults: %q %q %q", none.paymentsTitle(), none.policyTitle(), none.referralTitle())
	}
	if got := none.referralNote(15); got != "Give this code to a friend. When they shop with us for the first time, you get 15% off your next visit." {
		t.Fatalf("note = %q", got)
	}
	if none.thanks("") != "Thank you for shopping with us!" || none.thanks("Merci!") != "Merci!" {
		t.Fatal("thanks fallbacks")
	}
	own := receiptTexts{Thanks: "See you soon", ReferralNote: "Share it: {percent} for you"}
	if own.thanks("Merci!") != "See you soon" || own.referralNote(20) != "Share it: 20% for you" {
		t.Fatal("the branch's own wording didn't win")
	}
	for _, raw := range []string{"", "not json", "[1,2]"} {
		if got := parseReceiptTexts(raw); got.Thanks != "" || len(got.Hide) != 0 {
			t.Fatalf("%q -> %+v", raw, got)
		}
	}
	n := normalizeReceiptTexts(receiptTexts{Thanks: "  hi \r\n there ", Hide: []string{"policy", "nope", "policy", " socials "}})
	if n.Thanks != "hi \n there" || strings.Join(n.Hide, ",") != "policy,socials" {
		t.Fatalf("normalized = %+v", n)
	}
	long := normalizeReceiptTexts(receiptTexts{PolicyTitle: strings.Repeat("é", 400)})
	if len([]rune(long.PolicyTitle)) != receiptTextMaxLen {
		t.Fatalf("not cut: %d", len([]rune(long.PolicyTitle)))
	}
}

func sampleTextsReceipt(texts receiptTexts) printReceiptData {
	return printReceiptData{
		ReceiptNo: "SA-1", StoreName: "Shop", Items: []invoiceItem{{Name: "Shirt", Price: 10, Qty: 1}},
		GrandTotalCents: 1000, Payments: []receiptPayment{{AccountName: "Cash", AmountCents: 1000}},
		SocialPlatforms: []string{"instagram"}, SocialHandles: map[string]string{"instagram": "shop"},
		ReferralCode: "K7M4QH", ReferralNote: texts.referralNote(15), Texts: texts,
	}
}

// The printed (ESC/POS) receipt: no texts prints as before; the branch's
// wording replaces each default; a hidden part is gone.
func TestESCPosReceiptTexts(t *testing.T) {
	plain := string(buildESCPosReceipt(sampleTextsReceipt(receiptTexts{})))
	for _, want := range []string{"PAYMENT DETAILS", "EXCHANGE POLICY", "Exchanges accepted within 3 days.", "YOUR CODE: K7M4QH", "Follow us on:", "Thank you for shopping with us!"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("default receipt lacks %q", want)
		}
	}
	own := string(buildESCPosReceipt(sampleTextsReceipt(receiptTexts{
		PaymentsTitle: "PAID WITH", PolicyTitle: "EXCHANGES", ReferralTitle: "SHARE:", ReferralNote: "{percent} off for you", SocialsTitle: "Find us:", Thanks: "See you soon",
	})))
	for _, want := range []string{"PAID WITH", "EXCHANGES", "SHARE: K7M4QH", "15% off for you", "Find us:", "See you soon"} {
		if !strings.Contains(own, want) {
			t.Fatalf("own wording lacks %q", want)
		}
	}
	for _, gone := range []string{"PAYMENT DETAILS", "EXCHANGE POLICY", "YOUR CODE", "Follow us", "Thank you for shopping"} {
		if strings.Contains(own, gone) {
			t.Fatalf("default %q still printed", gone)
		}
	}
	hidden := string(buildESCPosReceipt(sampleTextsReceipt(receiptTexts{Hide: receiptSections})))
	for _, gone := range []string{"PAYMENT DETAILS", "CASH RECEIVED", "EXCHANGE POLICY", "Exchanges accepted", "Follow us on:", "Thank you"} {
		if strings.Contains(hidden, gone) {
			t.Fatalf("hidden part %q printed", gone)
		}
	}
	if !strings.Contains(hidden, "TOTAL") {
		t.Fatal("the total went with the hidden parts")
	}
}

// The raster receipt draws without error either way, and hiding parts
// makes it shorter.
func TestRasterReceiptTexts(t *testing.T) {
	full, err := renderReceiptPreview(sampleReceiptData(sampleTextsReceipt(receiptTexts{})), "", "")
	if err != nil {
		t.Fatal(err)
	}
	short, err := renderReceiptPreview(sampleReceiptData(sampleTextsReceipt(receiptTexts{Hide: receiptSections})), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if pngHeight(t, short) >= pngHeight(t, full) {
		t.Fatalf("hiding parts didn't shorten it: %d vs %d", pngHeight(t, short), pngHeight(t, full))
	}
}

// A hidden referral block prints no code at all.
func TestHiddenReferralBlock(t *testing.T) {
	program := db.ReferralProgram{Enabled: 1, RewardPercent: 15}
	client := db.Client{Phone: "+96171000001", ReferralCode: "K7M4QH"}
	if code, note := referralReceiptNote(program, client, receiptTexts{ReferralNote: "Share: {percent}"}); code != "K7M4QH" || note != "Share: 15%" {
		t.Fatalf("%q %q", code, note)
	}
	if code, _ := referralReceiptNote(program, client, receiptTexts{Hide: []string{receiptSectionReferral}}); code != "" {
		t.Fatal("hidden block still printed the code")
	}
}

// Central's update_settings stores the texts cleaned up, reports them back,
// and "" puts every default back.
func TestUpdateSettingsReceiptTexts(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "texts.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, rounding_currency, receipt_enabled, sales_inventory_id, receipt_font, receipt_body_font)
		VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 'USD', 0, 0, '', '')`); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	app := &App{ctx: ctx, store: store, conn: database}
	send := func(texts string) {
		t.Helper()
		raw, _ := json.Marshal(updateSettingsCommandPayload{ReceiptTexts: &texts})
		if err := app.executeUpdateSettingsCommand(ctx, raw); err != nil {
			t.Fatal(err)
		}
	}
	send(`{"thanks":"  See you soon ","hide":["policy","bogus"],"policy_title":""}`)
	if got := app.receiptTexts(); got.Thanks != "See you soon" || len(got.Hide) != 1 || got.Hide[0] != "policy" {
		t.Fatalf("stored = %+v", got)
	}
	settings, err := store.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if settings.ReceiptTexts != `{"thanks":"See you soon","hide":["policy"]}` {
		t.Fatalf("stored JSON = %s", settings.ReceiptTexts)
	}
	// Absent from a later command: left as it is.
	other := "Shop"
	raw, _ := json.Marshal(updateSettingsCommandPayload{MarketName: &other})
	if err := app.executeUpdateSettingsCommand(ctx, raw); err != nil {
		t.Fatal(err)
	}
	if app.receiptTexts().Thanks != "See you soon" {
		t.Fatal("an update without texts wiped them")
	}
	send("")
	if got := app.receiptTexts(); got.Thanks != "" || len(got.Hide) != 0 {
		t.Fatalf("reset = %+v", got)
	}
	raw, _ = json.Marshal(updateSettingsCommandPayload{ReceiptTexts: func() *string { s := "not json"; return &s }()})
	if err := app.executeUpdateSettingsCommand(ctx, raw); err == nil {
		t.Fatal("bad JSON was taken")
	}
}

func pngHeight(t *testing.T, data []byte) int {
	t.Helper()
	img, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return img.Height
}
