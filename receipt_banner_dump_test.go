package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// TestDumpBanners draws every preset into $BANNER_OUT to look at them.
func TestDumpBanners(t *testing.T) {
	dir := os.Getenv("BANNER_OUT")
	if dir == "" {
		t.Skip()
	}
	for i, b := range receiptBannerPresets {
		raw, _ := json.Marshal(b)
		png, err := RenderBannerPreview(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(fmt.Sprintf("%s/%02d-%s.png", dir, i, b.Style), png, 0o644)
	}
}
