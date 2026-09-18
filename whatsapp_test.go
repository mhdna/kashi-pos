package main

import (
	"testing"

	db "kashi-pos/db/sqlc"
)

func TestWhatsAppDigits(t *testing.T) {
	cases := map[string]string{
		"+96170123456":    "96170123456",
		"+961 70 123 456": "96170123456",
		"0096170123456":   "0096170123456",
		"123":             "",
		"":                "",
	}
	for in, want := range cases {
		if got := whatsAppDigits(in); got != want {
			t.Errorf("whatsAppDigits(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWhatsAppTemplateParams(t *testing.T) {
	settings := db.Setting{
		MarketName:             "Green Basket",
		BranchName:             "Hamra",
		WhatsappTemplateParams: "name,branch,invoice,total",
	}
	client := db.Client{Name: "Sara", Phone: "+96170123456"}

	got := whatsAppTemplateParams(settings, "INV-42", 1550, client)
	want := []map[string]any{
		{"type": "text", "text": "Sara"},
		{"type": "text", "text": "Hamra"},
		{"type": "text", "text": "INV-42"},
		{"type": "text", "text": "$15.50"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d params, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i]["text"] != want[i]["text"] {
			t.Errorf("param %d: got %v, want %v", i, got[i], want[i])
		}
	}

	// Missing client name falls back to "there".
	settings.WhatsappTemplateParams = "name"
	got = whatsAppTemplateParams(settings, "INV-1", 100, db.Client{})
	if got[0]["text"] != "there" {
		t.Errorf("name fallback: got %v", got[0])
	}

	// Unknown placeholder names are skipped.
	settings.WhatsappTemplateParams = "name,bogus,total"
	got = whatsAppTemplateParams(settings, "INV-1", 100, client)
	if len(got) != 2 {
		t.Errorf("expected unknown placeholder to be skipped, got %v", got)
	}
}
