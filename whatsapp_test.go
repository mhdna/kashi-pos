package main

import (
	"testing"

	db "kashi-pos/db/sqlc"
)

func TestWhatsAppChatID(t *testing.T) {
	cases := map[string]string{
		"+96170123456":    "96170123456@c.us",
		"+961 70 123 456": "96170123456@c.us",
		"0096170123456":   "0096170123456@c.us",
		"123":             "",
		"":                "",
	}
	for in, want := range cases {
		if got := whatsAppChatID(in); got != want {
			t.Errorf("whatsAppChatID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWhatsAppItemLines(t *testing.T) {
	items := []invoiceItem{
		{Name: "Blue Shirt", Price: 15, Qty: 2},        // 30.00
		{Name: "Cap", Price: 10, Qty: 1, Discount: 50}, // 5.00
		{Code: "SKU-9", Price: 4, Qty: 1.5},            // 6.00, name from code
		{Price: 99, Qty: 1},                            // no name/code -> skipped
	}
	got := whatsAppItemLines(items)
	want := "• 2 x Blue Shirt - $30.00\n• 1 x Cap - $5.00\n• 1.5 x SKU-9 - $6.00"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestRenderWhatsAppMessage(t *testing.T) {
	settings := db.Setting{
		MarketName:        "Green Basket",
		BranchName:        "Hamra",
		WhatsappMessage:   "Hi {name}, thanks for shopping with {market} ({branch})!\n{items}\nInvoice {invoice} = {total}. Rate us: {link}",
		WhatsappReviewUrl: "https://rate.example/x",
	}
	client := db.Client{Name: "Sara", Phone: "+96170123456"}
	items := []invoiceItem{{Name: "Blue Shirt", Price: 15, Qty: 1}}

	got := renderWhatsAppMessage(settings, "INV-42", 1550, client, items)
	want := "Hi Sara, thanks for shopping with Green Basket (Hamra)!\n• 1 x Blue Shirt - $15.00\nInvoice INV-42 = $15.50. Rate us: https://rate.example/x"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}

	// Blank review URL and no items collapse their placeholders (with the
	// preceding space/newline) instead of leaving gaps.
	settings.WhatsappReviewUrl = ""
	got = renderWhatsAppMessage(settings, "INV-42", 1550, client, nil)
	want = "Hi Sara, thanks for shopping with Green Basket (Hamra)!\nInvoice INV-42 = $15.50. Rate us:"
	if got != want {
		t.Errorf("collapse: got %q\nwant %q", got, want)
	}

	// Empty template -> empty output (feature no-ops).
	settings.WhatsappMessage = "   "
	if got := renderWhatsAppMessage(settings, "INV-42", 1550, client, items); got != "" {
		t.Errorf("empty template should render empty, got %q", got)
	}

	// Missing client name falls back to "there".
	settings.WhatsappMessage = "Hi {name}"
	if got := renderWhatsAppMessage(settings, "INV-1", 100, db.Client{}, nil); got != "Hi there" {
		t.Errorf("name fallback: got %q", got)
	}
}
