package main

import "testing"

func TestNormalizeCentralAPIURL(t *testing.T) {
	for in, want := range map[string]string{
		"":                               "",
		"cloud.example.com":              "https://cloud.example.com/api",
		"  cloud.example.com/  ":         "https://cloud.example.com/api",
		"https://cloud.example.com":      "https://cloud.example.com/api",
		"https://cloud.example.com/":     "https://cloud.example.com/api",
		"https://cloud.example.com/api":  "https://cloud.example.com/api",
		"https://cloud.example.com/api/": "https://cloud.example.com/api",
		"cloud.example.com/api":          "https://cloud.example.com/api",
		"https://example.com/kashi/api":  "https://example.com/kashi/api",
		"http://localhost:8080":          "http://localhost:8080",
		"http://192.168.1.10:8080/":      "http://192.168.1.10:8080",
	} {
		if got := normalizeCentralAPIURL(in); got != want {
			t.Errorf("normalizeCentralAPIURL(%q) = %q, want %q", in, got, want)
		}
	}
}
