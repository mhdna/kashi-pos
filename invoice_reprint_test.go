package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestReprintInvoiceUsesStoredTotalNotCurrentSettings guards against a real
// bug: printInvoiceReceipt used to re-round the already-persisted grand
// total against *current* settings on every reprint, so a reprint after a
// rounding-mode/currency change could show a different total than what was
// actually charged - or, as here, fail outright if the current settings
// happen to be in a bad state that has nothing to do with the original
// sale. The fix reads the stored total as-is. We prove it by corrupting
// rounding_mode to an invalid value *after* checkout (simulating stale/
// legacy settings) and confirming a reprint attempt fails only because no
// printer is configured in this test - never because of rounding.
func TestReprintInvoiceUsesStoredTotalNotCurrentSettings(t *testing.T) {
	app, cashbox, client, database := setupQuoteTestApp(t, 0.11, "half_up", "LBP")
	items := cartJSON(3)

	quote, err := app.QuoteInvoiceTotal(items)
	if err != nil {
		t.Fatal(err)
	}
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: quote.GrandTotalCents}})
	result, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: items, PaymentsJSON: string(payments), ClientID: client.ID})
	if err != nil {
		t.Fatal(err)
	}

	// Simulate a settings row that has drifted into an invalid rounding
	// mode - something the normal UpdateSettings validation would never
	// allow, but which the reprint path must not depend on regardless.
	if _, err := database.Exec(`UPDATE settings SET rounding_mode = 'bogus'`); err != nil {
		t.Fatal(err)
	}

	err = app.ReprintInvoice(result.InvoiceID)
	if err == nil {
		t.Fatal("expected an error since no receipt printer is configured in this test")
	}
	if !strings.Contains(err.Error(), "printer") {
		t.Fatalf("reprint failed for a reason other than the missing printer (rounding leaked in): %v", err)
	}

	details, err := app.GetInvoiceDetails(result.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	if details["grandTotal"].(int64) != result.GrandTotal {
		t.Fatalf("stored grand total = %d, want unchanged %d", details["grandTotal"].(int64), result.GrandTotal)
	}
}
