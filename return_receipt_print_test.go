package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// countReceiptPrints swaps the printer for a counter for the test.
func countReceiptPrints(t *testing.T) *int {
	t.Helper()
	prints := 0
	saved := printReturnReceipt
	printReturnReceipt = func(*App, int64, []checkoutPayment) error {
		prints++
		return nil
	}
	t.Cleanup(func() { printReturnReceipt = saved })
	return &prints
}

func setReceipts(t *testing.T, shop dealShop, receiptsOn bool, returnReceipts *bool) {
	t.Helper()
	if _, err := shop.app.conn.Exec(`UPDATE settings SET receipt_enabled = ?`, boolInt(receiptsOn)); err != nil {
		t.Fatal(err)
	}
	if returnReceipts != nil {
		if err := shop.app.ReportDisplayPrefs(fmt.Sprintf(`{"print_return_receipts": %t}`, *returnReceipts)); err != nil {
			t.Fatal(err)
		}
	}
}

func cashierExchange(t *testing.T, shop dealShop, saleID int64, lines []exchangeLine, printReceipt bool) map[string]any {
	t.Helper()
	itemsJSON, _ := json.Marshal(lines)
	quote, err := shop.app.QuoteExchangeTotal(string(itemsJSON), saleID)
	if err != nil {
		t.Fatal(err)
	}
	payments := "[]"
	if quote.GrandTotalCents > 0 {
		raw, _ := json.Marshal([]checkoutPayment{{AccountID: shop.cashbox.ID, AmountCents: quote.GrandTotalCents}})
		payments = string(raw)
	}
	result, err := shop.app.CheckoutExchangeInvoice(string(itemsJSON), payments, saleID, 0, shop.salesperson.ID, "", printReceipt)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func saleClientRef(t *testing.T, shop dealShop, saleID int64) string {
	t.Helper()
	var ref string
	if err := shop.app.conn.QueryRow(`SELECT client_ref FROM invoice_client_refs WHERE invoice_id = ?`, saleID).Scan(&ref); err != nil {
		t.Fatal(err)
	}
	return ref
}

// choiceName names a yes/no/unset choice in a subtest name.
func choiceName(v *bool) string {
	if v == nil {
		return "unset"
	}
	return fmt.Sprint(*v)
}

var (
	yes = func() *bool { v := true; return &v }()
	no  = func() *bool { v := false; return &v }()
)

// The cashier's "Print receipt" tick decides, over the setting (which only
// sets where the tick starts), as long as receipts are on.
func TestCashierExchangeReceiptFollowsTheTick(t *testing.T) {
	exchanges := map[string][]exchangeLine{
		"paid item for a jacket": {line("A", -1), line("JACKET", 1)},
		"free item for socks":    {line("C", -1), line("SOCKS", 1)},
		"one item for two":       {line("A", -1), line("JACKET", 1), line("SOCKS", 1)},
	}
	for name, lines := range exchanges {
		for _, c := range []struct {
			receiptsOn     bool
			returnReceipts *bool
			tick           bool
			wantPrint      bool
		}{
			{true, nil, true, true},
			{true, nil, false, false},
			{true, yes, false, false},
			{true, no, true, true},
			{true, no, false, false},
			{false, yes, true, false},
			{false, nil, false, false},
		} {
			t.Run(fmt.Sprintf("%s/receipts=%t/setting=%s/tick=%t", name, c.receiptsOn, choiceName(c.returnReceipts), c.tick), func(t *testing.T) {
				shop := setUpDealShop(t)
				prints := countReceiptPrints(t)
				setReceipts(t, shop, c.receiptsOn, c.returnReceipts)
				sale := shop.sell(t, "A", "B", "C")
				result := cashierExchange(t, shop, sale.InvoiceID, lines, c.tick)
				if got := *prints == 1; got != c.wantPrint || *prints > 1 {
					t.Fatalf("printed %d times, want print=%t", *prints, c.wantPrint)
				}
				if result["receiptPrinted"] != c.wantPrint {
					t.Fatalf("receiptPrinted = %v, want %t", result["receiptPrinted"], c.wantPrint)
				}
				var queued int
				if err := shop.app.conn.QueryRow(`SELECT count(*) FROM sync_outbox WHERE entity_type = 'return_invoice'`).Scan(&queued); err != nil || queued != 1 {
					t.Fatalf("exchange queued for central %d times (%v), want 1: a skipped receipt changes nothing else", queued, err)
				}
			})
		}
	}
}

// A return central sends (RS/CT/2026/00068 was one): unticked in central it
// never prints; ticked or from an older central (no print_receipt) it follows
// the till's "Print return receipts". Same for a remote exchange and cancel.
func TestRemoteReturnReceipt(t *testing.T) {
	kinds := map[string]func(ref string) map[string]any{
		"return": func(ref string) map[string]any { return map[string]any{"client_ref": ref} },
		"one item": func(ref string) map[string]any {
			return map[string]any{"client_ref": ref, "items": []map[string]any{{"branch_product_id": 0, "quantity": 1}}}
		},
		"cancel": func(ref string) map[string]any { return map[string]any{"client_ref": ref, "cancel": true} },
	}
	for name, build := range kinds {
		for _, c := range []struct {
			receiptsOn     bool
			returnReceipts *bool
			printReceipt   *bool
			wantPrint      bool
		}{
			{true, nil, nil, true},
			{true, nil, yes, true},
			{true, nil, no, false},
			{true, yes, no, false},
			{true, no, nil, false},
			{true, no, yes, false},
			{false, yes, yes, false},
		} {
			t.Run(fmt.Sprintf("%s/receipts=%t/setting=%s/kashi=%s", name, c.receiptsOn, choiceName(c.returnReceipts), choiceName(c.printReceipt)), func(t *testing.T) {
				shop := setUpDealShop(t)
				prints := countReceiptPrints(t)
				setReceipts(t, shop, c.receiptsOn, c.returnReceipts)
				sale := shop.sell(t, "A", "B", "C")
				payload := build(saleClientRef(t, shop, sale.InvoiceID))
				if name == "one item" {
					var productID int64
					if err := shop.app.conn.QueryRow(`SELECT id FROM products WHERE code = 'A'`).Scan(&productID); err != nil {
						t.Fatal(err)
					}
					payload["items"] = []map[string]any{{"branch_product_id": productID, "quantity": 1}}
				}
				if c.printReceipt != nil {
					payload["print_receipt"] = *c.printReceipt
				}
				raw, _ := json.Marshal(payload)
				if err := shop.app.executeRemoteReturnInvoiceCommand(shop.app.ctx, raw); err != nil {
					t.Fatal(err)
				}
				if got := *prints == 1; got != c.wantPrint || *prints > 1 {
					t.Fatalf("printed %d times, want print=%t", *prints, c.wantPrint)
				}
				var returns int
				if err := shop.app.conn.QueryRow(`SELECT count(*) FROM return_invoices`).Scan(&returns); err != nil || returns != 1 {
					t.Fatalf("%d returns recorded (%v), want 1", returns, err)
				}
			})
		}
	}
}

// The setting reads as on until it's turned off, and a push from central
// (update_settings.display_prefs) turns it off and on like the screen does.
func TestPrintReturnReceiptsSetting(t *testing.T) {
	shop := setUpDealShop(t)
	if !shop.app.returnReceiptsOn() {
		t.Fatal("return receipts must start on")
	}
	if err := shop.app.ReportDisplayPrefs(`{"print_return_receipts": false, "animations_enabled": true}`); err != nil {
		t.Fatal(err)
	}
	if shop.app.returnReceiptsOn() {
		t.Fatal("turned off on the screen, still on")
	}
	if err := shop.app.applyRemoteDisplayPrefs(json.RawMessage(`{"print_return_receipts": true}`)); err != nil {
		t.Fatal(err)
	}
	if !shop.app.returnReceiptsOn() {
		t.Fatal("turned on from central, still off")
	}
	if err := shop.app.applyRemoteDisplayPrefs(json.RawMessage(`{"print_return_receipts": "no"}`)); err == nil {
		t.Fatal("a non-boolean print_return_receipts was accepted")
	}
}

// A return that didn't print is still there to print from Invoices.
func TestUnprintedReturnCanBeReprinted(t *testing.T) {
	shop := setUpDealShop(t)
	countReceiptPrints(t)
	setReceipts(t, shop, true, nil)
	sale := shop.sell(t, "A", "B", "C")
	result := cashierExchange(t, shop, sale.InvoiceID, []exchangeLine{line("A", -1), line("JACKET", 1)}, false)
	var id int64
	if err := shop.app.conn.QueryRow(`SELECT id FROM invoices WHERE invoice_code = ?`, result["invoiceCode"]).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := shop.app.conn.Exec(`UPDATE settings SET receipt_printer = ''`); err != nil {
		t.Fatal(err)
	}
	if err := shop.app.ReprintInvoice(id); err == nil || err.Error() != "no receipt printer configured" {
		t.Fatalf("reprint = %v, want it to build the receipt and reach the printer step", err)
	}
}

// The screen's "Print receipt" tick follows the same rule as the backend
// (frontend/src/lib/returnReceipt.ts checkoutPrints): this writes the
// backend's answer for every case to a fixture returnReceipt.test.ts reads
// (KASHI_UPDATE_GOLDEN=1 go test -run TestReturnReceiptRuleGolden .).
func TestReturnReceiptRuleGolden(t *testing.T) {
	shop := setUpDealShop(t)
	type row struct {
		ReceiptsOn bool `json:"receiptsOn"`
		Tick       bool `json:"tick"`
		Prints     bool `json:"prints"`
	}
	var rows []row
	for _, receiptsOn := range []bool{false, true} {
		for _, tick := range []bool{false, true} {
			// The tick wins over the setting, so both settings must agree.
			answers := map[bool]bool{}
			for _, setting := range []bool{false, true} {
				if err := shop.app.ReportDisplayPrefs(fmt.Sprintf(`{"print_return_receipts": %t}`, setting)); err != nil {
					t.Fatal(err)
				}
				answers[setting] = shop.app.printsReturnReceipt(receiptsOn, receiptChoiceOf(&tick))
			}
			if answers[false] != answers[true] {
				t.Fatalf("the screen's tick lost to the setting: receipts=%t tick=%t", receiptsOn, tick)
			}
			rows = append(rows, row{receiptsOn, tick, answers[true]})
		}
	}
	out, _ := json.MarshalIndent(rows, "", "  ")
	out = append(out, '\n')
	const path = "frontend/src/lib/testdata/return_receipt_rule.json"
	if os.Getenv("KASHI_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, out, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (write it with KASHI_UPDATE_GOLDEN=1)", err)
	}
	if string(stored) != string(out) {
		t.Fatalf("%s is stale: the screen would disagree with the till (rewrite with KASHI_UPDATE_GOLDEN=1)\nwant:\n%s", path, out)
	}
}
