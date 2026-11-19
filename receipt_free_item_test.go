package main

import (
	"strings"
	"testing"
)

// The reported case: a unit a deal gave free showed only $0.00. Every look
// now prints FREE for it and what it cost, with the deal that made it free.
func TestFreeItemShowsItsPrice(t *testing.T) {
	data := receiptPreviewData(designBase(), receiptModeItemDeal)
	var free invoiceItem
	for _, it := range data.Items {
		if it.Free {
			free = it
		}
	}
	if !free.Free {
		t.Fatal("the sample has no free unit")
	}
	want := priceNoteText(free)
	if want != "Was $20.00, now free (100% off, Buy 2 get 1)" {
		t.Fatalf("note %q", want)
	}
	for _, theme := range receiptThemes() {
		for _, layout := range theme.Layouts {
			drawn, _, _ := drawnTexts(t, data, theme.ID, layout.ID)
			all := strings.ToUpper(strings.Join(drawn, " "))
			if !strings.Contains(all, "FREE") || !strings.Contains(all, "WAS $20.00") || !strings.Contains(all, "100% OFF") || !strings.Contains(all, "BUY 2 GET 1") {
				t.Fatalf("%s/%s lacks the free note:\n%s", theme.ID, layout.ID, all)
			}
			for _, d := range drawn {
				if d == "$0.00" || d == "0.00" {
					t.Fatalf("%s/%s still prints %q", theme.ID, layout.ID, d)
				}
			}
		}
	}
	text := string(buildESCPosReceipt(data))
	if !strings.Contains(text, want) || strings.Contains(text, "$0.00") {
		t.Fatalf("text receipt:\n%s", text)
	}
	// A paid line has no note and keeps its amount.
	paid := invoiceItem{Name: "Shirt", Price: 20, Qty: 1}
	if priceNoteText(paid) != "" || netText(paid, 20) != "$20.00" {
		t.Fatal("a paid line changed")
	}
	// A discounted line shows before and after.
	if got := priceNoteText(invoiceItem{Name: "Jeans", Price: 50, Qty: 1, Discount: 10}); got != "Was $50.00, now $45.00" {
		t.Fatalf("discounted: %q", got)
	}
	if got := priceNoteText(invoiceItem{Free: true, Price: 5, Qty: 2, Discount: 100}); got != "Was $10.00, now free (100% off)" {
		t.Fatalf("no deal name: %q", got)
	}
}
