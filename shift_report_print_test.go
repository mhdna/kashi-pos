package main

import (
	"context"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	db "kashi-pos/db/sqlc"
)

func TestShiftReportFormattingAndCashAccountDetection(t *testing.T) {
	if !isCashAccount("USD", "USD") || !isCashAccount("Cash LBP", "LBP") {
		t.Fatal("cash accounts were not detected")
	}
	if isCashAccount("Visa", "USD") || isCashAccount("Whish", "USD") {
		t.Fatal("electronic payment account was treated as cash")
	}
	if got := formatSignedShiftUSD(-200); got != "-$2.00" {
		t.Fatalf("USD difference = %q, want -$2.00", got)
	}
	if got := formatSignedShiftLBP(100000); got != "+100,000 LBP" {
		t.Fatalf("LBP difference = %q, want +100,000 LBP", got)
	}
}

func TestShiftReportPaidVsCountedIncludesOverpaymentOnce(t *testing.T) {
	const (
		netSales    = int64(10000)
		overpayment = int64(100)
		paid        = netSales + overpayment
	)
	countedTotal, err := reconcileShiftCountedTotal(
		paid,
		5000, 5000,
		4_500_000, 4_500_000,
		5000, 5000,
		100, 100,
		testLBPCurrency, roundingHalfUp,
	)
	if err != nil {
		t.Fatal(err)
	}
	if countedTotal != paid {
		t.Fatalf("counted total = %d, paid = %d; overpayment must be included exactly once", countedTotal, paid)
	}
}

func TestShiftReportPaidVsCountedUsesDisplayedLBPDifference(t *testing.T) {
	countedTotal, err := reconcileShiftCountedTotal(
		10000,
		5000, 5000,
		4_510_000, 4_510_000,
		2500, 2500,
		2500, 2500,
		testLBPCurrency, roundingHalfUp,
	)
	if err != nil {
		t.Fatal(err)
	}
	if countedTotal != 10000 {
		t.Fatalf("counted total = %d, want 10000 when every displayed count matches", countedTotal)
	}
}

func TestBuildShiftReportRaster(t *testing.T) {
	data := shiftReportPreviewData()
	payload, err := buildESCPOSRasterReceipt(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRasterPayload(payload, raster80mmWidth); err != nil {
		t.Fatalf("invalid shift report payload: %v", err)
	}
}

func TestWriteShiftReportPreview(t *testing.T) {
	outputPath := os.Getenv("KASHI_SHIFT_REPORT_PREVIEW_PNG")
	if outputPath == "" {
		t.Skip("preview output path not requested")
	}
	preview, err := renderReceiptRaster(shiftReportPreviewData())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, preview); err != nil {
		t.Fatal(err)
	}
}

func shiftReportPreviewData() printReceiptData {
	report := &shiftReportData{
		ShiftID: 24, Cashier: "John", Register: "POS-01",
		OpenedAt: "2026-07-30 08:00", ClosedAt: "2026-07-30 16:00",
		FirstInvoice: "SA/CN/2026/01251", LastInvoice: "SA/CN/2026/01308",
		Transactions: 58, ItemsSold: 143, Returns: 1,
		GrossSales: 142000, Discounts: 4500, ReturnAmount: 2000,
		Tax: 14850, NetSales: 150350, TotalPayments: 150350, CountedTotal: 149911,
		Payments: []receiptPayment{
			{AccountName: "USD", CurrencyCode: "USD", AmountCents: 65000},
			{AccountName: "LBP", CurrencyCode: "LBP", AmountCents: 50000},
			{AccountName: "Visa", CurrencyCode: "USD", AmountCents: 25350},
			{AccountName: "Whish", CurrencyCode: "USD", AmountCents: 10000},
		},
		ExpectedUSD: 72000, CountedUSD: 71800,
		USDBillCounts: []shiftDenominationCount{
			{Denomination: 100, Count: 6},
			{Denomination: 50, Count: 2},
			{Denomination: 10, Count: 1},
			{Denomination: 5, Count: 1},
			{Denomination: 1, Count: 3},
		},
		ExpectedLBP: 45000000, CountedLBP: 45100000,
		ExpectedVisa: 25350, CountedVisa: 25000,
		ExpectedWhish: 10000, CountedWhish: 10000,
		RoundingMode: roundingHalfUp,
	}
	return printReceiptData{
		StoreName: "Kashi Market", Branch: "Downtown",
		ReceiptWidth: 80, TitleSize: 21, BodySize: 11,
		Font: "Belleza", BodyFont: "Printer Font A",
		Cutoff: true, ShiftReport: report,
	}
}

func TestCloseShiftStoresClosingPersonAndCounts(t *testing.T) {
	ctx := context.Background()
	database := openTestDatabase(t)
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO settings (
		branch_name, api_key, printer_id, printer_size, receipt_printer,
		screen_port, receipt_width, receipt_height, theme, tax_rate,
		rounding_mode, receipt_enabled, sales_inventory_id
	) VALUES ('Test Branch', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	shift, err := store.CreateShift(ctx, db.CreateShiftParams{})
	if err != nil {
		t.Fatal(err)
	}
	person, err := store.CreateSalesperson(ctx, "John")
	if err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: ctx, store: store}
	result, err := app.CloseShiftByID(
		shift.ID,
		person.ID,
		45100000,
		12000,
		8000,
		`{"100":6,"50":2,"20":0,"10":1,"5":1,"1":3}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if printed, _ := result["receiptPrinted"].(bool); printed {
		t.Fatal("receipt printed while receipt printing was disabled")
	}
	closed, err := store.GetShiftByID(ctx, shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	if closed.IsClosed != 1 || closed.ClosedBySalespersonID != person.ID ||
		closed.CountedUsd != 71800 || closed.CountedLbp != 45100000 ||
		closed.CountedVisa != 12000 || closed.CountedWhish != 8000 ||
		closed.CountedUsdBreakdown == "" {
		t.Fatalf("stored close data = %#v", closed)
	}
	// No invoices were rung up on this shift, so nothing was expected.
	if closed.ExpectedUsd != 0 || closed.ExpectedLbp != 0 || closed.ExpectedVisa != 0 || closed.ExpectedWhish != 0 {
		t.Fatalf("expected counts on an income-free shift = %#v", closed)
	}
}

// TestCloseShiftPersistsAndExposesExpectedVsCountedVariance is the
// regression guard for "we don't save the close-shift counted amount, and
// can't see it later": the expected_* columns computed at close time
// (mirroring what the close-shift receipt itself showed) must be
// persisted, and GetShiftTotals - which feeds the monthly table - must
// expose both counted_* and expected_* so a variance can be displayed.
func TestCloseShiftPersistsAndExposesExpectedVsCountedVariance(t *testing.T) {
	ctx := context.Background()
	database := openTestDatabase(t)
	if err := applyMigrations(ctx, database, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO settings (branch_name, api_key, printer_id, printer_size, receipt_printer, screen_port, receipt_width, receipt_height, theme, tax_rate, rounding_mode, receipt_enabled, sales_inventory_id) VALUES ('Test', '', '', '80mm', '', '', 80, 297, 'system', 0, 'half_up', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	settings, err := store.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.RoundingCurrency = "USD"
	if err := store.UpdateSettings(ctx, settingsToUpdateParams(settings)); err != nil {
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
	settings.SalesInventoryID = inventory.ID
	if err := store.UpdateSettings(ctx, settingsToUpdateParams(settings)); err != nil {
		t.Fatal(err)
	}
	// A whole-dollar price so the expected total is exactly representable
	// by physical USD bills at close time.
	product, err := store.CreateProduct(ctx, db.CreateProductParams{
		Barcode: "1", Code: "SKU-1", Name: "Test", IsActive: 1, Price: 2000, Discount: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertInventoryItem(ctx, db.UpsertInventoryItemParams{InventoryID: inventory.ID, ProductID: product.ID, Quantity: 5}); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, store: store}
	items := cartJSON(1)
	quote, err := app.QuoteInvoiceTotal(items)
	if err != nil {
		t.Fatal(err)
	}
	if quote.GrandTotalCents != 2000 {
		t.Fatalf("quote grand total = %d, want 2000", quote.GrandTotalCents)
	}
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: quote.GrandTotalCents}})
	if _, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: items, PaymentsJSON: string(payments), ClientID: client.ID}); err != nil {
		t.Fatal(err)
	}

	person, err := store.CreateSalesperson(ctx, "Jane")
	if err != nil {
		t.Fatal(err)
	}

	// Counted $1 short of the $20 expected, to prove the variance survives
	// the round trip through storage, not just the in-memory close-shift
	// report.
	if _, err := app.CloseShiftByID(shift.ID, person.ID, 0, 0, 0, `{"100":0,"50":0,"20":0,"10":1,"5":1,"1":4}`); err != nil {
		t.Fatal(err)
	}

	closed, err := store.GetShiftByID(ctx, shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	if closed.ExpectedUsd != 2000 {
		t.Fatalf("stored expected USD = %d, want 2000", closed.ExpectedUsd)
	}
	if closed.CountedUsd != 1900 {
		t.Fatalf("stored counted USD = %d, want 1900", closed.CountedUsd)
	}

	totals, err := app.GetShiftTotals("")
	if err != nil {
		t.Fatal(err)
	}
	if len(totals) != 1 {
		t.Fatalf("shift totals = %#v", totals)
	}
	row := totals[0]
	if databaseInt64(row["expectedUsd"]) != 2000 {
		t.Fatalf("GetShiftTotals expectedUsd = %v, want 2000", row["expectedUsd"])
	}
	if databaseInt64(row["countedUsd"]) != 1900 {
		t.Fatalf("GetShiftTotals countedUsd = %v, want 1900", row["countedUsd"])
	}
	if row["closedBySalesperson"] != "Jane" {
		t.Fatalf("GetShiftTotals closedBySalesperson = %v, want Jane", row["closedBySalesperson"])
	}
}

func TestParseUSDBillCounts(t *testing.T) {
	breakdown, total, canonical, err := parseUSDBillCounts(
		`{"100":6,"50":2,"20":0,"10":1,"5":1,"1":3}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if total != 71800 || len(breakdown) != 5 || canonical == "" {
		t.Fatalf("breakdown=%#v total=%d canonical=%q", breakdown, total, canonical)
	}
	if _, _, _, err := parseUSDBillCounts(`{"200":1}`); err == nil {
		t.Fatal("unsupported denomination was accepted")
	}
}
