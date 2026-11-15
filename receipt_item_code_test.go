package main

import (
	"bytes"
	"strings"
	"testing"
)

// Two products can share a name -- "Lady Leather Skirt" was both TNV551219
// and TNV551219-16 -- so every receipt prints each item's code under its
// name, the barcode when it has no code, and nothing when it has neither.
func itemCodeTestItems() []invoiceItem {
	return []invoiceItem{
		{Name: "Lady Leather Skirt", Code: "TNV551219-16", Barcode: "8000000113603", Price: 45, Discount: 30, Qty: 2},
		{Name: "Lady Leather Skirt", Code: "TNV551219", Barcode: "8000000114129", Color: "Black", Size: "M", Price: 45, Qty: 1},
		{Name: "Wool Scarf", Code: "  ", Barcode: "8000000000777", Price: 25, Qty: 1},
		{Name: "Gift Wrap", Price: 1, Qty: 1},
	}
}

func TestItemCodeText(t *testing.T) {
	cases := []struct {
		item invoiceItem
		want string
	}{
		{invoiceItem{Code: "TNV551219-16", Barcode: "8000000113603"}, "TNV551219-16"},
		{invoiceItem{Code: " TNV551219 ", Barcode: "8000000114129"}, "TNV551219"},
		{invoiceItem{Code: "", Barcode: "8000000000777"}, "8000000000777"},
		{invoiceItem{Code: "   ", Barcode: " 8000000000777 "}, "8000000000777"},
		{invoiceItem{}, ""},
	}
	for _, tc := range cases {
		if got := itemCodeText(tc.item); got != tc.want {
			t.Errorf("itemCodeText(%+v) = %q, want %q", tc.item, got, tc.want)
		}
	}
}

type drawnText struct {
	value string
	y     int
}

// codeOf strips the decoration a layout puts around a code ("Code: ", the
// tapes' indent) so it can be compared exactly.
func codeOf(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "Code: ")
}

// isNameLine reports whether drawn text is (the start of) an item's name;
// tapes print names in capitals and long names wrap.
func isNameLine(value, name string) bool {
	value = strings.TrimSpace(value)
	return value != "" && strings.HasPrefix(strings.ToUpper(name), strings.ToUpper(value)) && strings.EqualFold(strings.Fields(value)[0], strings.Fields(name)[0])
}

func TestEveryReceiptLookPrintsItemCodeUnderName(t *testing.T) {
	items := itemCodeTestItems()
	wantCodes := []string{"TNV551219-16", "TNV551219", "8000000000777", ""}
	for _, width := range []int64{80, 58} {
		for _, theme := range receiptThemeCatalog() {
			for _, layout := range theme.Layouts {
				var drawn []drawnText
				receiptTextDrawn = func(value string, y int) { drawn = append(drawn, drawnText{value, y}) }
				data := sampleReceiptData(printReceiptData{ReceiptWidth: width, TitleSize: 21, BodySize: 11, BodyFont: "Printer Font A", Font: "Belleza"})
				data.Items = items
				_, err := renderReceiptPreview(data, theme.ID, layout.ID)
				receiptTextDrawn = nil
				if err != nil {
					t.Fatalf("%dmm %s/%s: %v", width, theme.ID, layout.ID, err)
				}
				// Walk the drawn text in order: each item's name, then its
				// code before the next item's name.
				cursor := 0
				for i, item := range items {
					nameAt := -1
					for j := cursor; j < len(drawn); j++ {
						if isNameLine(drawn[j].value, item.Name) {
							nameAt = j
							break
						}
					}
					if nameAt < 0 {
						t.Fatalf("%dmm %s/%s: item %d name %q not drawn", width, theme.ID, layout.ID, i, item.Name)
					}
					end := len(drawn)
					if i+1 < len(items) {
						for j := nameAt + 1; j < len(drawn); j++ {
							if isNameLine(drawn[j].value, items[i+1].Name) {
								end = j
								break
							}
						}
					}
					var found []drawnText
					for _, d := range drawn[nameAt+1 : end] {
						if c := codeOf(d.value); c != "" && (c == "TNV551219-16" || c == "TNV551219" || c == "8000000000777") {
							found = append(found, d)
						}
					}
					want := wantCodes[i]
					if want == "" {
						if len(found) != 0 {
							t.Errorf("%dmm %s/%s: item %d has no code but printed %q", width, theme.ID, layout.ID, i, found[0].value)
						}
					} else if len(found) != 1 || codeOf(found[0].value) != want {
						t.Errorf("%dmm %s/%s: item %d code: printed %v, want exactly %q", width, theme.ID, layout.ID, i, found, want)
					} else if found[0].y <= drawn[nameAt].y {
						t.Errorf("%dmm %s/%s: item %d code %q at y=%d is not below its name at y=%d", width, theme.ID, layout.ID, i, want, found[0].y, drawn[nameAt].y)
					}
					cursor = nameAt + 1
				}
				for _, d := range drawn {
					if strings.TrimSpace(d.value) == "Code:" {
						t.Errorf("%dmm %s/%s: printed an empty code label", width, theme.ID, layout.ID)
					}
				}
			}
		}
	}
}

func TestESCPosReceiptPrintsItemCodeUnderName(t *testing.T) {
	payload := buildESCPosReceipt(printReceiptData{
		StoreName: "Centro", ReceiptNo: "SA/CT/2026/03191", Items: itemCodeTestItems(),
		GrandTotalCents: 9900, RoundingCurrency: testLBPReceiptCurrency, ReceiptWidth: 80,
	})
	text := string(payload)
	for _, want := range []string{
		"Lady Leather Skirt", "  TNV551219-16\n",
		"Lady Leather Skirt", "  TNV551219\n",
		"Wool Scarf", "  8000000000777\n",
		"Gift Wrap",
	} {
		at := strings.Index(text, want)
		if at < 0 {
			t.Fatalf("receipt is missing %q in order: %q", want, payload)
		}
		text = text[at+len(want):]
	}
	// The code-less item prints its quantity line straight after its name.
	giftAt := bytes.Index(payload, []byte("Gift Wrap"))
	rest := string(payload[giftAt:])
	lines := strings.Split(rest, "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[1], "  1 x $1.00") {
		t.Fatalf("code-less item should go straight to its quantity line, got %q", lines[:2])
	}
}
