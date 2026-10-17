package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

// TestListInvoicesPageBatchesClientAndSettlementLookups proves the batched
// (non-N+1) rewrite of ListInvoicesPage produces exactly the same
// client/settlement-account data the old per-row lookup loop did, across a
// page containing invoices for different clients, a walk-in (no client),
// and payments through different cashbox accounts.
func TestListInvoicesPageBatchesClientAndSettlementLookups(t *testing.T) {
	app, cashboxA, walkInClient, database := setupQuoteTestApp(t, 0, "half_up", "USD")
	defer database.Close()

	cashboxB, err := app.store.CreateCashboxAccount(app.ctx, db.CreateCashboxAccountParams{Name: "Visa", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	clientA, err := app.store.CreateClient(app.ctx, db.CreateClientParams{Name: "First Client", Phone: "22222222"})
	if err != nil {
		t.Fatal(err)
	}
	clientB, err := app.store.CreateClient(app.ctx, db.CreateClientParams{Name: "Second Client", Phone: "11111111"})
	if err != nil {
		t.Fatal(err)
	}

	items := cartJSON(1)
	quote, err := app.QuoteInvoiceTotal(items)
	if err != nil {
		t.Fatal(err)
	}

	// Invoice for clientA, paid via cashboxA.
	paymentsA, _ := json.Marshal([]checkoutPayment{{AccountID: cashboxA.ID, AmountCents: quote.GrandTotalCents}})
	if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: items, PaymentsJSON: string(paymentsA), ClientID: clientA.ID}); err != nil {
		t.Fatal(err)
	}
	// Invoice for clientB, paid via cashboxB.
	paymentsB, _ := json.Marshal([]checkoutPayment{{AccountID: cashboxB.ID, AmountCents: quote.GrandTotalCents}})
	if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: items, PaymentsJSON: string(paymentsB), ClientID: clientB.ID}); err != nil {
		t.Fatal(err)
	}
	// Walk-in invoice (no explicit client), split across both cashboxes.
	half := quote.GrandTotalCents / 2
	paymentsSplit, _ := json.Marshal([]checkoutPayment{
		{AccountID: cashboxA.ID, AmountCents: half},
		{AccountID: cashboxB.ID, AmountCents: quote.GrandTotalCents - half},
	})
	if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: items, PaymentsJSON: string(paymentsSplit)}); err != nil {
		t.Fatal(err)
	}

	page, err := app.ListInvoicesPage(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || len(page.Items) != 3 {
		t.Fatalf("page = %#v, want 3 items", page)
	}

	byClientID := map[int64]map[string]any{}
	for _, item := range page.Items {
		byClientID[item["clientId"].(int64)] = item
	}

	rowA := byClientID[clientA.ID]
	if rowA == nil {
		t.Fatalf("no page row for clientA (id=%d): %#v", clientA.ID, page.Items)
	}
	if rowA["clientName"] != clientA.Name {
		t.Fatalf("clientA row clientName = %v, want %q", rowA["clientName"], clientA.Name)
	}
	settlementA, ok := rowA["settlementAccounts"].([]string)
	if !ok || len(settlementA) != 1 || settlementA[0] != "Cash" {
		t.Fatalf("clientA settlement accounts = %#v", rowA["settlementAccounts"])
	}

	rowB := byClientID[clientB.ID]
	if rowB == nil {
		t.Fatalf("no page row for clientB (id=%d): %#v", clientB.ID, page.Items)
	}
	if rowB["clientName"] != clientB.Name {
		t.Fatalf("clientB row clientName = %v, want %q", rowB["clientName"], clientB.Name)
	}
	settlementB, ok := rowB["settlementAccounts"].([]string)
	if !ok || len(settlementB) != 1 || settlementB[0] != "Visa" {
		t.Fatalf("clientB settlement accounts = %#v", rowB["settlementAccounts"])
	}

	walkIn := byClientID[walkInClient.ID]
	if walkIn == nil {
		t.Fatalf("no walk-in page row (client id=%d): %#v", walkInClient.ID, page.Items)
	}
	if walkIn["clientName"] != walkInClient.Name {
		t.Fatalf("walk-in row clientName = %v, want %q", walkIn["clientName"], walkInClient.Name)
	}
	settlementWalkIn, ok := walkIn["settlementAccounts"].([]string)
	if !ok || len(settlementWalkIn) != 2 {
		t.Fatalf("walk-in settlement accounts = %#v, want both cashboxes", walkIn["settlementAccounts"])
	}

	filterCases := []struct {
		name    string
		filters string
		want    int
	}{
		{name: "customer", filters: `{"customer":"Second"}`, want: 1},
		{name: "paid to", filters: `{"paidTo":"Visa"}`, want: 2},
		{name: "combined", filters: `{"customer":"Second","paidTo":"Visa"}`, want: 1},
		{name: "salesperson", filters: `{"salesperson":"Anonymous"}`, want: 3},
		{name: "invoice code", filters: fmt.Sprintf(`{"invoiceCode":%q}`, rowA["invoiceCode"]), want: 1},
		{name: "date", filters: fmt.Sprintf(`{"date":%q}`, localDay(t, rowA["createdAt"].(string))), want: 3},
		{name: "total", filters: fmt.Sprintf(`{"total":%q}`, fmt.Sprintf("%.2f", float64(quote.GrandTotalCents)/100)), want: 3},
		{name: "no match", filters: `{"customer":"does-not-exist"}`, want: 0},
	}
	for _, test := range filterCases {
		t.Run("filter "+test.name, func(t *testing.T) {
			filtered, err := app.ListInvoicesFilteredPage(0, 20, test.filters)
			if err != nil {
				t.Fatal(err)
			}
			if int(filtered.Total) != test.want || len(filtered.Items) != test.want {
				t.Fatalf("filtered page = %#v, want %d items", filtered, test.want)
			}
		})
	}
}

// TestListInvoicesPageKeysetPaginationWalksEveryRowOnce proves the keyset
// ("seek") pagination contract: passing the previous page's last id as the
// cursor walks the whole history in strict id-descending order with no
// gaps or repeats, and COUNT(*) is only computed for the first page
// (Total = -1 thereafter).
func TestListInvoicesPageKeysetPaginationWalksEveryRowOnce(t *testing.T) {
	app, cashboxA, _, database := setupQuoteTestApp(t, 0, "half_up", "USD")
	defer database.Close()

	items := cartJSON(1)
	quote, err := app.QuoteInvoiceTotal(items)
	if err != nil {
		t.Fatal(err)
	}
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashboxA.ID, AmountCents: quote.GrandTotalCents}})
	const invoiceCount = 7
	for i := 0; i < invoiceCount; i++ {
		if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: items, PaymentsJSON: string(payments)}); err != nil {
			t.Fatal(err)
		}
	}

	const pageSize = 2
	var seen []int64
	cursor := int64(0)
	for page := 0; ; page++ {
		result, err := app.ListInvoicesPage(cursor, pageSize)
		if err != nil {
			t.Fatal(err)
		}
		if page == 0 && result.Total != invoiceCount {
			t.Fatalf("first page Total = %d, want %d", result.Total, invoiceCount)
		}
		if page > 0 && result.Total != -1 {
			t.Fatalf("page %d Total = %d, want -1 (count only runs on the first page)", page, result.Total)
		}
		if len(result.Items) == 0 {
			break
		}
		for _, item := range result.Items {
			id := item["id"].(int64)
			if len(seen) > 0 && id >= seen[len(seen)-1] {
				t.Fatalf("ids not strictly descending: %d after %d", id, seen[len(seen)-1])
			}
			seen = append(seen, id)
		}
		cursor = result.Items[len(result.Items)-1]["id"].(int64)
		if len(result.Items) < pageSize {
			break
		}
		if page > invoiceCount {
			t.Fatal("pagination did not terminate")
		}
	}
	if len(seen) != invoiceCount {
		t.Fatalf("walked %d invoices across all pages, want %d", len(seen), invoiceCount)
	}
}

// TestGetShiftTotalsIsBoundedToTheCurrentMonth proves the monthly shift
// table's backing query only loads the current month's shifts, not the
// shop's entire history.
func TestGetShiftTotalsIsBoundedToTheCurrentMonth(t *testing.T) {
	app, database := newProductTestApp(t)
	defer database.Close()

	shift, err := app.store.CreateShift(app.ctx, db.CreateShiftParams{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE shifts SET created_at = datetime('now', 'localtime', '-2 months') WHERE id != ?`, shift.ID); err != nil {
		t.Fatal(err)
	}
	oldShift, err := app.store.CreateShift(app.ctx, db.CreateShiftParams{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE shifts SET created_at = datetime('now', 'localtime', '-2 months') WHERE id = ?`, oldShift.ID); err != nil {
		t.Fatal(err)
	}

	totals, err := app.GetShiftTotals("")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int64]bool{}
	for _, row := range totals {
		seen[row["id"].(int64)] = true
	}
	if !seen[shift.ID] {
		t.Fatalf("current-month shift missing from GetShiftTotals: %#v", totals)
	}
	if seen[oldShift.ID] {
		t.Fatalf("a shift from 2 months ago was included in GetShiftTotals: %#v", totals)
	}
}

// localDay is the local calendar day of a stored UTC created_at -- what the
// date filter matches (datetime(created_at, 'localtime')), and not the UTC
// day the string starts with, which differs from local midnight to 02:00/03:00.
func localDay(t *testing.T, createdAt string) string {
	t.Helper()
	at, err := time.ParseInLocation("2006-01-02 15:04:05", createdAt, time.UTC)
	if err != nil {
		t.Fatalf("parse createdAt %q: %v", createdAt, err)
	}
	return at.Local().Format("2006-01-02")
}
