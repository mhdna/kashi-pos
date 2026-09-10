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

func TestRenderWhatsAppMessage(t *testing.T) {
	settings := db.Setting{
		MarketName:        "Green Basket",
		BranchName:        "Hamra",
		WhatsappMessage:   "Hi {name}, thanks for shopping with {market} ({branch})! Invoice {invoice} = {total}. Rate us: {link}",
		WhatsappReviewUrl: "https://rate.example/x",
	}
	client := db.Client{Name: "Sara", Phone: "+96170123456"}

	got := renderWhatsAppMessage(settings, "INV-42", 1550, client)
	want := "Hi Sara, thanks for shopping with Green Basket (Hamra)! Invoice INV-42 = $15.50. Rate us: https://rate.example/x"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}

	// Blank review URL drops " {link}" cleanly.
	settings.WhatsappReviewUrl = ""
	got = renderWhatsAppMessage(settings, "INV-42", 1550, client)
	want = "Hi Sara, thanks for shopping with Green Basket (Hamra)! Invoice INV-42 = $15.50. Rate us:"
	if got != want {
		t.Errorf("blank link: got %q\nwant %q", got, want)
	}

	// Empty template -> empty output (feature no-ops).
	settings.WhatsappMessage = "   "
	if got := renderWhatsAppMessage(settings, "INV-42", 1550, client); got != "" {
		t.Errorf("empty template should render empty, got %q", got)
	}

	// Missing client name falls back to "there".
	settings.WhatsappMessage = "Hi {name}"
	if got := renderWhatsAppMessage(settings, "INV-1", 100, db.Client{}); got != "Hi there" {
		t.Errorf("name fallback: got %q", got)
	}
}
