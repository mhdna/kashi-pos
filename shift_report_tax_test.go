package main

import (
	"context"
	"strings"
	"testing"

	db "kashi-pos/db/sqlc"
)

// shiftWithInvoices builds a closed-over shift holding sales invoices with the
// given (discounted_total, grand_total) pairs and returns its report summary.
func shiftReportSummaryForTotals(t *testing.T, taxRate float64, roundingMode string, totals [][2]int64) db.GetShiftReportSummaryRow {
	t.Helper()
	ctx := context.Background()
	database := openTestDatabase(t)
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	settings := defaultSettingsParams()
	settings.TaxRate = taxRate
	settings.RoundingMode = roundingMode
	if err := store.UpsertSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	shift, err := store.CreateShift(ctx, db.CreateShiftParams{})
	if err != nil {
		t.Fatal(err)
	}
	cashbox, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "Cash", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := store.CreateInventory(ctx, "Sales")
	if err != nil {
		t.Fatal(err)
	}
	client, err := store.CreateClient(ctx, db.CreateClientParams{Name: "Walk-in", Phone: "00000000"})
	if err != nil {
		t.Fatal(err)
	}
	for i, total := range totals {
		discounted, grand := total[0], total[1]
		invoice, err := store.CreateInvoice(ctx, db.CreateInvoiceParams{
			CashboxID: cashbox.ID, ShiftID: shift.ID, InvoiceCode: "SA/CN/2026/0000" + string(rune('1'+i)),
			InvoiceIndex: int64(i + 1), Year: 2026, ClientID: client.ID, InventoryID: inventory.ID,
			Subtotal: discounted, DiscountedTotal: discounted, GrandTotal: grand,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.CreateSalesInvoice(ctx, invoice.ID); err != nil {
			t.Fatal(err)
		}
	}
	summary, err := store.GetShiftReportSummary(ctx, shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	return summary
}

// The shift report used to label grand_total - discounted_total as "tax",
// but checkout snaps the grand total onto the rounding currency's cash grid
// after adding tax - so at a 0% rate that line still printed a few stray
// cents (and sometimes a negative "tax") that moved every shift.
func TestShiftReportTaxIsZeroAtZeroRate(t *testing.T) {
	summary := shiftReportSummaryForTotals(t, 0, "half_up", [][2]int64{
		{5000, 5012}, // snapped up 12 cents
		{3333, 3300}, // snapped down 33 cents
	})
	if tax := databaseInt64(summary.Tax); tax != 0 {
		t.Fatalf("tax = %d, want 0 at a 0%% tax rate", tax)
	}
	if summary.CashRounding != 12-33 {
		t.Fatalf("cash rounding = %d, want the -21 cent net snap", summary.CashRounding)
	}
}

// Tax and rounding must still add back up to grand_total - discounted_total,
// or the printed report stops reconciling to NET SALES.
func TestShiftReportTaxAndRoundingSplitGrandTotalDifference(t *testing.T) {
	summary := shiftReportSummaryForTotals(t, 0.11, "half_up", [][2]int64{
		{5000, 5600}, // 550 tax, then +50 snap
		{1234, 1350}, // round(1234 * 0.11) = 136 tax, then -20 snap
	})
	if tax := databaseInt64(summary.Tax); tax != 550+136 {
		t.Fatalf("tax = %d, want 686", tax)
	}
	if summary.CashRounding != 50-20 {
		t.Fatalf("cash rounding = %d, want 30", summary.CashRounding)
	}
	if got, want := databaseInt64(summary.Tax)+summary.CashRounding, int64(600+116); got != want {
		t.Fatalf("tax + rounding = %d, want the %d cent grand_total difference", got, want)
	}
}

// The SQL tax rounding has to agree with roundFloatToCents, which checkout
// uses, including on exact .5 ties - otherwise the rounding line silently
// absorbs a cent that belongs in tax.
func TestShiftReportTaxMatchesCheckoutRounding(t *testing.T) {
	for _, mode := range []string{"half_up", "down", "up"} {
		for _, discounted := range []int64{70, 290, 2070, 1234, 9999, 4607610} {
			want, err := roundFloatToCents(float64(discounted)/100*0.05, mode)
			if err != nil {
				t.Fatal(err)
			}
			summary := shiftReportSummaryForTotals(t, 0.05, mode, [][2]int64{{discounted, discounted + want}})
			if got := databaseInt64(summary.Tax); got != want {
				t.Fatalf("mode %s, discounted %d: tax = %d, want %d", mode, discounted, got, want)
			}
		}
	}
}

// receiptLines renders the ESC/POS text receipt and returns it as plain text
// so the totals block can be asserted on.
func receiptLines(t *testing.T, data printReceiptData) string {
	t.Helper()
	return string(buildESCPosReceipt(data))
}

// At a 0% tax rate the receipt used to print a "Tax" line holding nothing but
// the cash-grid snap on the grand total.
func TestReceiptOmitsZeroTax(t *testing.T) {
	text := receiptLines(t, printReceiptData{
		Items:           []invoiceItem{{Name: "Shirt", Qty: 1, Price: 50.00, Discount: 0}},
		GrandTotalCents: 5012, // snapped up 12 cents by the LBP cash grid
		TaxCents:        0,
		ReceiptWidth:    80,
	})
	if strings.Contains(text, "Tax") {
		t.Fatalf("receipt printed a Tax line at a 0%% rate:\n%s", text)
	}
	if !strings.Contains(text, "Rounding") {
		t.Fatalf("receipt dropped the 12 cent snap instead of showing it as rounding:\n%s", text)
	}
}

// A real tax rate still prints, and the snap stays on its own line.
func TestReceiptPrintsNonZeroTaxSeparatelyFromRounding(t *testing.T) {
	text := receiptLines(t, printReceiptData{
		Items:           []invoiceItem{{Name: "Shirt", Qty: 1, Price: 50.00, Discount: 0}},
		GrandTotalCents: 5600, // 5000 + 550 tax + 50 snap
		TaxCents:        550,
		ReceiptWidth:    80,
	})
	if !strings.Contains(text, "Tax") || !strings.Contains(text, "5.50") {
		t.Fatalf("receipt lost the $5.50 tax line:\n%s", text)
	}
	if !strings.Contains(text, "Rounding") || !strings.Contains(text, "0.50") {
		t.Fatalf("receipt lost the 50 cent rounding line:\n%s", text)
	}
}

// Neither line appears when the total lands exactly on the cash grid at a 0%
// rate - the common case, which should print Subtotal/Discount and nothing else.
func TestReceiptOmitsBothLinesWhenExact(t *testing.T) {
	text := receiptLines(t, printReceiptData{
		Items:           []invoiceItem{{Name: "Shirt", Qty: 1, Price: 50.00, Discount: 0}},
		GrandTotalCents: 5000,
		TaxCents:        0,
		ReceiptWidth:    80,
	})
	if strings.Contains(text, "Tax") || strings.Contains(text, "Rounding") {
		t.Fatalf("receipt printed a stray tax/rounding line:\n%s", text)
	}
}
