package main

import (
	"encoding/json"
	"math/rand"
	"testing"
)

// A till is French until told otherwise; central's command and Settings both
// change it, and anything else leaves it alone.
func TestTillLanguage(t *testing.T) {
	app, _, _, _, _ := newExchangeReplacementPricingFixture(t)
	check := func(want string) {
		t.Helper()
		settings, err := app.GetAppSettings()
		if err != nil {
			t.Fatal(err)
		}
		if settings["tillLanguage"] != want {
			t.Fatalf("language = %v, want %v", settings["tillLanguage"], want)
		}
	}
	check("fr")
	for _, tc := range []struct{ payload, want string }{
		{`{"till_language":"en"}`, "en"},
		{`{}`, "en"},
		{`{"till_language":null}`, "en"},
		{`{"till_language":""}`, "en"},
		{`{"till_language":"fr"}`, "fr"},
		{`{"till_language":"fr"}`, "fr"},
		{`{"till_language":" EN "}`, "en"},
	} {
		// Central always sends the whole settings; these are its required ones.
		full := map[string]any{"branch_name": "Test", "rounding_mode": "half_up", "rounding_currency": "USD", "exchange_rate": 1, "social_platforms": []string{}, "social_handles": map[string]string{}}
		if err := json.Unmarshal([]byte(tc.payload), &full); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(full)
		if err := app.executeUpdateSettingsCommand(app.ctx, raw); err != nil {
			t.Fatalf("%s: %v", tc.payload, err)
		}
		check(tc.want)
	}
	if err := app.SetTillLanguage("ar"); err == nil {
		t.Fatal("an unknown language was accepted")
	}
	check("en")

	rng := rand.New(rand.NewSource(1))
	want := "en"
	for i := 0; i < 200; i++ {
		next := []string{"fr", "en", "de", ""}[rng.Intn(4)]
		if err := app.SetTillLanguage(next); (err == nil) != (next == "fr" || next == "en") {
			t.Fatalf("%q: %v", next, err)
		} else if err == nil {
			want = next
		}
		check(want)
	}
}
