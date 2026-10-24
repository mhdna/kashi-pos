package main

import "testing"

// Colour/size variants of one model share a code; sent with their barcodes
// they are two lines, and only the same barcode twice is a duplicate.
func TestParseSalesItemsVariantsShareCode(t *testing.T) {
	two := `[{"code":"C6KSKE29","barcode":"8000000000001","qty":1},{"code":"C6KSKE29","barcode":"8000000000002","qty":1}]`
	if _, err := parseSalesItems(two, "UNKNOWN", "UNKNOWN"); err != nil {
		t.Fatalf("variants with different barcodes rejected: %v", err)
	}
	same := `[{"code":"C6KSKE29","barcode":"8000000000001","qty":1},{"code":"C6KSKE29","barcode":"8000000000001","qty":1}]`
	if _, err := parseSalesItems(same, "UNKNOWN", "UNKNOWN"); err == nil {
		t.Fatal("the same barcode twice should be a duplicate")
	}
}

// One product sold as two sale options is two lines; the same option twice
// is still a duplicate.
func TestParseSalesItemsSaleOptionsAreSeparateLines(t *testing.T) {
	two := `[{"code":"CA21BGSWE2","barcode":"8000000000003","qty":1,"saleOption":"Winter"},{"code":"CA21BGSWE2","barcode":"8000000000003","qty":1,"saleOption":"Summer"}]`
	if _, err := parseSalesItems(two, "UNKNOWN", "UNKNOWN"); err != nil {
		t.Fatalf("Winter and Summer lines of one product rejected: %v", err)
	}
	same := `[{"code":"CA21BGSWE2","barcode":"8000000000003","qty":1,"saleOption":"Winter"},{"code":"CA21BGSWE2","barcode":"8000000000003","qty":1,"saleOption":"Winter"}]`
	if _, err := parseSalesItems(same, "UNKNOWN", "UNKNOWN"); err == nil {
		t.Fatal("the same product and option twice should be a duplicate")
	}
}
