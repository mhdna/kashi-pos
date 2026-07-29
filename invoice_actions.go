package main

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/go-pdf/fpdf"

	db "kashi-pos/db/sqlc"
)

//go:embed assets/fonts/Belleza-Regular.ttf
var bellezaFont []byte

//go:embed assets/fonts/CascadiaMono-Regular.ttf
var cascadiaMonoRegular []byte

//go:embed assets/fonts/CascadiaMono-Bold.ttf
var cascadiaMonoBold []byte

//go:embed pdf-icons/facebook.png
var facebookIcon []byte

//go:embed pdf-icons/instagram.png
var instagramIcon []byte

//go:embed pdf-icons/threads.png
var threadsIcon []byte

//go:embed pdf-icons/tiktok.png
var tiktokIcon []byte

type invoiceItem struct {
	Name     string  `json:"name"`
	Code     string  `json:"code"`
	Barcode  string  `json:"barcode"`
	Category string  `json:"category"`
	Price    float64 `json:"price"`
	Discount float64 `json:"discount"`
	Qty      float64 `json:"qty"`
}

func (a *App) HoldInvoice(itemsJSON string) (int64, error) {
	if a.store == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	var items []struct {
		Code    string  `json:"code"`
		Barcode string  `json:"barcode"`
		Qty     float64 `json:"qty"`
	}
	if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
		return 0, err
	}
	inv, err := a.store.CreateHoldInvoice(a.ctx)
	if err != nil {
		return 0, err
	}
	for _, it := range items {
		var product db.Product
		if strings.TrimSpace(it.Barcode) != "" {
			product, err = a.store.GetProductByBarcode(a.ctx, strings.TrimSpace(it.Barcode))
		} else {
			product, err = a.store.GetProductByCode(a.ctx, it.Code)
		}
		if err != nil {
			continue
		}
		_ = a.store.AddHoldInvoiceItem(a.ctx, db.AddHoldInvoiceItemParams{
			HoldInvoiceID: inv.ID,
			ProductID:     product.ID,
			Qty:           int64(it.Qty),
		})
	}
	return inv.ID, nil
}

func (a *App) ListHoldInvoices() ([]map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	list, err := a.store.ListHoldInvoices(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(list))
	for i, inv := range list {
		items, _ := a.store.ListHoldInvoiceItems(a.ctx, inv.ID)
		itemCount := len(items)
		var itemList []map[string]any
		for _, it := range items {
			itemList = append(itemList, map[string]any{
				"name":     it.Name,
				"code":     it.Code,
				"barcode":  it.Barcode,
				"price":    float64(it.Price) / 100,
				"color":    it.Color,
				"size":     it.Size,
				"discount": it.Discount,
				"qty":      it.Qty,
				"category": it.Category,
			})
		}
		result[i] = map[string]any{
			"id":        inv.ID,
			"itemCount": itemCount,
			"items":     itemList,
			"createdAt": inv.CreatedAt,
		}
	}
	return result, nil
}

func (a *App) DeleteHoldInvoice(id int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.DeleteHoldInvoice(a.ctx, id)
}

func (a *App) DeleteInvoice(id int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if _, err := a.store.GetInvoiceByID(a.ctx, id); err != nil {
		return err
	}
	return a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		if err := q.DeleteInvoicePayments(a.ctx, id); err != nil {
			return err
		}
		if err := q.DeleteInvoicePaymentEntries(a.ctx, id); err != nil {
			return err
		}
		if err := q.DeleteInvoiceProducts(a.ctx, id); err != nil {
			return err
		}
		if err := q.DeleteSalesInvoice(a.ctx, id); err != nil {
			return err
		}
		return q.DeleteInvoice(a.ctx, id)
	})
}

func (a *App) VoidInvoice(itemsJSON string, reason string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := a.store.CreateVoidInvoice(a.ctx, db.CreateVoidInvoiceParams{
		ItemsJson: itemsJSON,
		Reason:    reason,
	})
	return err
}

func (a *App) CheckoutInvoice(itemsJSON, paymentsJSON string, clientID int64, storeName, branch, phone, salesperson, receiptPrinter, notes string) (map[string]any, error) {
	result, err := a.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: itemsJSON, PaymentsJSON: paymentsJSON, ClientID: clientID, Notes: notes})
	if err != nil {
		return nil, fmt.Errorf("invoice creation failed: %w", err)
	}

	response := map[string]any{
		"invoiceCode":    result.InvoiceCode,
		"receiptPrinted": false,
	}
	// salesInvoiceTx creates the invoice before printing and leaves the
	// salesperson unset. The app-bar selection must be persisted regardless of
	// whether receipt printing is enabled.
	if strings.TrimSpace(salesperson) != "" {
		if people, peopleErr := a.store.ListSalespersons(a.ctx); peopleErr == nil {
			for _, person := range people {
				if person.FullName == salesperson {
					_ = a.store.UpdateInvoiceSalesperson(a.ctx, db.UpdateInvoiceSalespersonParams{SalespersonID: person.ID, ID: result.InvoiceID})
					break
				}
			}
		}
	}
	settings, settingsErr := a.store.GetSettings(a.ctx)
	if settingsErr == nil && settings.ReceiptEnabled != 0 {
		marketName := settings.MarketName
		if marketName == "" {
			marketName = "Market"
		}
		customer := "Walk-in"
		if client, err := a.store.GetClientByID(a.ctx, clientID); err == nil && client.Name != "" && !strings.Contains(strings.ToLower(client.Name), "walk-in") {
			customer = client.Name
		}
		if err := a.printReceipt(result.Items, marketName, settings.BranchName, settings.MarketPhone, settings.BranchPhone, settings.MarketDescription, settings.CounterNumber, salesperson, customer, result.Notes, result.InvoiceCode, settings.ReceiptPrinter, result.GrandTotal, result.Payments, settings.ExchangeRate, settings.RoundingMode, settings.ReceiptWidth, settings.ReceiptHeight, settings.ReturnPolicy, settings.Website, settings.Instagram, decodeSocialPlatforms(settings.SocialPlatforms), decodeSocialHandles(settings.SocialHandles), settings.ReceiptFont, settings.ReceiptBodyFont, settings.ReceiptTitleSize, settings.ReceiptBodySize, settings.ReceiptCutoff != 0); err != nil {
			response["receiptError"] = err.Error()
		} else {
			response["receiptPrinted"] = true
		}
	}
	return response, nil
}

func (a *App) GetInvoiceDetails(id int64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	inv, err := a.store.GetInvoiceByID(a.ctx, id)
	if err != nil {
		return nil, err
	}
	products, err := a.store.ListInvoiceProducts(a.ctx, id)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(products))
	for _, line := range products {
		product, productErr := a.store.GetProductByID(a.ctx, line.ProductID)
		if productErr != nil {
			continue
		}
		items = append(items, map[string]any{
			"name": product.Name, "code": product.Code, "barcode": product.Barcode, "category": product.Description,
			"price": float64(line.UnitPrice) / 100, "lineTotal": float64(line.LineTotal) / 100,
			"discount": line.Discount, "qty": line.Quantity,
		})
	}
	payments, err := a.store.ListInvoicePayments(a.ctx, id)
	if err != nil {
		return nil, err
	}
	paymentList := make([]map[string]any, 0, len(payments))
	for _, payment := range payments {
		account, _ := a.store.GetCashboxAccountByID(a.ctx, payment.CashboxID)
		paymentList = append(paymentList, map[string]any{"accountId": payment.CashboxID, "accountName": account.Name, "amount": payment.Amount})
	}
	clientName := "Walk-in"
	clientPhone := ""
	if client, clientErr := a.store.GetClientByID(a.ctx, inv.ClientID); clientErr == nil {
		clientName, clientPhone = client.Name, client.Phone
	}
	salespersonName := "Anonymous"
	if inv.SalespersonID > 0 {
		if salesperson, salespersonErr := a.store.GetSalespersonByID(a.ctx, inv.SalespersonID); salespersonErr == nil {
			salespersonName = salesperson.FullName
		}
	}
	return map[string]any{
		"id": inv.ID, "invoiceCode": inv.InvoiceCode, "clientId": inv.ClientID, "clientName": clientName, "clientPhone": clientPhone,
		"salespersonId": inv.SalespersonID, "salespersonName": salespersonName, "cashboxId": inv.CashboxID,
		"subtotal": inv.Subtotal, "discountedTotal": inv.DiscountedTotal, "grandTotal": inv.GrandTotal,
		"createdAt": inv.CreatedAt, "notes": inv.Notes, "items": items, "payments": paymentList,
	}, nil
}

func (a *App) ChangeInvoiceClient(invoiceID, clientID int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if _, err := a.store.GetClientByID(a.ctx, clientID); err != nil {
		return err
	}
	return a.store.UpdateInvoiceClient(a.ctx, db.UpdateInvoiceClientParams{ClientID: clientID, ID: invoiceID})
}

func (a *App) ChangeInvoiceSalesperson(invoiceID, salespersonID int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if salespersonID > 0 {
		if _, err := a.store.GetSalespersonByID(a.ctx, salespersonID); err != nil {
			return err
		}
	}
	return a.store.UpdateInvoiceSalesperson(a.ctx, db.UpdateInvoiceSalespersonParams{SalespersonID: salespersonID, ID: invoiceID})
}

func (a *App) ChangeInvoiceSettlement(invoiceID, cashboxID int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	account, err := a.store.GetCashboxAccountByID(a.ctx, cashboxID)
	if err != nil || account.ID == 0 {
		return fmt.Errorf("settlement account not found")
	}
	inv, err := a.store.GetInvoiceByID(a.ctx, invoiceID)
	if err != nil {
		return err
	}
	oldPayments, err := a.store.ListInvoicePayments(a.ctx, invoiceID)
	if err != nil {
		return err
	}
	return a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		for _, payment := range oldPayments {
			if _, err := q.AddCashboxAccountBalance(a.ctx, db.AddCashboxAccountBalanceParams{AccountID: payment.CashboxID, ShiftID: inv.ShiftID, Balance: -payment.Amount}); err != nil {
				return err
			}
		}
		if err := q.DeleteInvoicePayments(a.ctx, invoiceID); err != nil {
			return err
		}
		if err := q.DeleteInvoicePaymentEntries(a.ctx, invoiceID); err != nil {
			return err
		}
		if err := q.CreateInvoicePayment(a.ctx, db.CreateInvoicePaymentParams{InvoiceID: invoiceID, CashboxID: cashboxID, Amount: inv.GrandTotal}); err != nil {
			return err
		}
		if err := q.CreateInvoicePaymentEntry(a.ctx, db.CreateInvoicePaymentEntryParams{InvoiceID: invoiceID, CashboxID: cashboxID, Amount: inv.GrandTotal, EntryType: "payment"}); err != nil {
			return err
		}
		if _, err := q.AddCashboxAccountBalance(a.ctx, db.AddCashboxAccountBalanceParams{AccountID: cashboxID, ShiftID: inv.ShiftID, Balance: inv.GrandTotal}); err != nil {
			return err
		}
		return q.UpdateInvoiceCashbox(a.ctx, db.UpdateInvoiceCashboxParams{CashboxID: cashboxID, ID: invoiceID})
	})
}

func (a *App) ReprintInvoice(id int64) error {
	return a.printInvoiceReceipt(id, nil)
}

func (a *App) printInvoiceReceipt(id int64, payments []checkoutPayment) error {
	details, err := a.GetInvoiceDetails(id)
	if err != nil {
		return err
	}
	items := make([]invoiceItem, 0)
	for _, raw := range details["items"].([]map[string]any) {
		items = append(items, invoiceItem{Name: raw["name"].(string), Code: raw["code"].(string), Barcode: raw["barcode"].(string), Category: raw["category"].(string), Price: raw["price"].(float64), Discount: float64(raw["discount"].(int64)), Qty: float64(raw["qty"].(int64))})
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return err
	}
	marketName := settings.MarketName
	if marketName == "" {
		marketName = "Market"
	}
	return a.printReceipt(items, marketName, settings.BranchName, settings.MarketPhone, settings.BranchPhone, settings.MarketDescription, settings.CounterNumber, details["salespersonName"].(string), details["clientName"].(string), details["notes"].(string), details["invoiceCode"].(string), settings.ReceiptPrinter, details["grandTotal"].(int64), payments, settings.ExchangeRate, settings.RoundingMode, settings.ReceiptWidth, settings.ReceiptHeight, settings.ReturnPolicy, settings.Website, settings.Instagram, decodeSocialPlatforms(settings.SocialPlatforms), decodeSocialHandles(settings.SocialHandles), settings.ReceiptFont, settings.ReceiptBodyFont, settings.ReceiptTitleSize, settings.ReceiptBodySize, settings.ReceiptCutoff != 0)
}

// GetReceiptPreview returns a PDF rendered with the current receipt settings.
// It deliberately uses sample products so the settings screen can preview the
// complete layout without creating an invoice or touching inventory.
func (a *App) GetReceiptPreview() (string, error) {
	if a.store == nil {
		return "", fmt.Errorf("database not initialized")
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return "", err
	}
	marketName := settings.MarketName
	if marketName == "" {
		marketName = "Market"
	}
	data := printReceiptData{
		Items: []invoiceItem{
			{Name: "Kids Boy Pants - White", Code: "DEMO-001", Barcode: "100000000001", Price: 12, Qty: 1},
			{Name: "Lady Shorts - Khaki", Code: "DEMO-002", Barcode: "100000000002", Price: 15, Qty: 1, Discount: 15},
			{Name: "Men Jeans", Code: "DEMO-003", Barcode: "100000000003", Price: 25, Qty: 2, Discount: 10},
		},
		StoreName: marketName, Branch: settings.BranchName, Phone: settings.MarketPhone, BranchPhone: settings.BranchPhone,
		Description: settings.MarketDescription, CounterNumber: settings.CounterNumber, Salesperson: "Sarah Demo", Customer: "Walk-in",
		ReceiptNo: "INV-DEMO-0001", GrandTotalCents: 6500, ExchangeRate: settings.ExchangeRate,
		RoundingMode: settings.RoundingMode, ReceiptWidth: settings.ReceiptWidth, ReceiptHeight: settings.ReceiptHeight,
		ReturnPolicy: settings.ReturnPolicy, Website: settings.Website, Instagram: settings.Instagram, SocialPlatforms: decodeSocialPlatforms(settings.SocialPlatforms), SocialHandles: decodeSocialHandles(settings.SocialHandles),
		Font: settings.ReceiptFont, BodyFont: settings.ReceiptBodyFont, TitleSize: settings.ReceiptTitleSize, BodySize: settings.ReceiptBodySize,
	}
	pdf := buildReceiptPDF(data)
	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(output.Bytes()), nil
}

func (a *App) printReceipt(items []invoiceItem, storeName, branch, phone, branchPhone, description, counter, salesperson, customer, notes, receiptNo, printerName string, grandTotalCents int64, payments []checkoutPayment, exchangeRate int64, roundingMode string, receiptWidth, receiptHeight int64, returnPolicy, website, instagram string, socialPlatforms []string, socialHandles map[string]string, font, bodyFont string, titleSize, bodySize float64, cutoff bool) error {
	if printerName == "" {
		return fmt.Errorf("no receipt printer configured")
	}
	notes = strings.TrimSpace(notes)

	receiptPayments := make([]receiptPayment, 0, len(payments))
	for _, payment := range payments {
		account, err := a.store.GetCashboxAccountByID(a.ctx, payment.AccountID)
		if err != nil {
			return fmt.Errorf("receipt payment account: %w", err)
		}
		receiptPayments = append(receiptPayments, receiptPayment{AccountName: account.Name, CurrencyCode: account.CurrencyCode, AmountCents: payment.AmountCents, EntryType: payment.EntryType})
	}
	return printReceiptToPrinter(printReceiptData{
		Items:           items,
		StoreName:       storeName,
		Branch:          branch,
		Phone:           phone,
		BranchPhone:     branchPhone,
		Description:     description,
		CounterNumber:   counter,
		Salesperson:     salesperson,
		Customer:        customer,
		Notes:           notes,
		ReturnPolicy:    returnPolicy,
		Website:         website,
		Instagram:       instagram,
		SocialPlatforms: socialPlatforms,
		SocialHandles:   socialHandles,
		Font:            font,
		BodyFont:        bodyFont,
		TitleSize:       titleSize,
		BodySize:        bodySize,
		Cutoff:          cutoff,
		ReceiptNo:       receiptNo,
		PrinterName:     printerName,
		GrandTotalCents: grandTotalCents,
		Payments:        receiptPayments,
		ExchangeRate:    exchangeRate,
		RoundingMode:    roundingMode,
		ReceiptWidth:    receiptWidth,
		ReceiptHeight:   receiptHeight,
	})
}

// buildReceiptPDF renders a compact, boutique-style receipt: tight vertical
// rhythm, a slim header, condensed item rows (name + qty/price on one line
// where it fits), and a single-row "Follow us on:" social strip with one
// handle beneath it.
func buildReceiptPDF(data printReceiptData) *fpdf.Fpdf {
	const (
		defaultPageW = 80.0
		defaultPageH = 297.0
		margin       = 3.5
	)
	pageW := float64(data.ReceiptWidth)
	if pageW <= 0 {
		pageW = defaultPageW
	}
	pageH := float64(data.ReceiptHeight)
	if pageH <= 0 {
		pageH = defaultPageH
	}
	contentW := pageW - 2*margin

	pdf := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "mm",
		SizeStr:        "",
		Size:           fpdf.SizeType{Wd: pageW, Ht: pageH},
	})
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()

	titleFont := data.Font
	if titleFont == "" {
		titleFont = "Belleza"
	} else if titleFont != "Belleza" && titleFont != "Helvetica" && titleFont != "Times" && titleFont != "Courier" {
		titleFont = "Belleza"
	}
	bodyFont := data.BodyFont
	if bodyFont == "" || bodyFont == "Printer Font A" || bodyFont == "Courier" {
		// Printer Font A is hardware-defined for ESC/POS; CascadiaMono is the
		// PDF equivalent — a natural monospace instead of Courier's typewriter look.
		bodyFont = "CascadiaMono"
	}
	if bodyFont != "Helvetica" && bodyFont != "Times" && bodyFont != "CascadiaMono" && bodyFont != "Belleza" {
		bodyFont = "CascadiaMono"
	}
	if titleFont == "Belleza" || bodyFont == "Belleza" {
		pdf.AddUTF8FontFromBytes("Belleza", "", bellezaFont)
		pdf.AddUTF8FontFromBytes("Belleza", "B", bellezaFont)
	}
	if bodyFont == "CascadiaMono" {
		pdf.AddUTF8FontFromBytes("CascadiaMono", "", cascadiaMonoRegular)
		pdf.AddUTF8FontFromBytes("CascadiaMono", "B", cascadiaMonoBold)
	}
	titleSize, bodySize := data.TitleSize, data.BodySize
	if titleSize <= 0 {
		titleSize = 19
	}
	if bodySize <= 0 {
		bodySize = 9.5
	}
	const lh = 3.6 // base line height, tighter than the old 4mm rhythm

	boldText := func(s string, size float64, align string) {
		pdf.SetFont(bodyFont, "B", size)
		pdf.SetXY(margin, pdf.GetY())
		pdf.CellFormat(contentW, lh, s, "", 0, align, false, 0, "")
	}
	twoCol := func(left, right string, size float64) {
		pdf.SetFont(bodyFont, "B", size)
		y := pdf.GetY()
		pdf.SetXY(margin, y)
		pdf.CellFormat(contentW/2, lh, left, "", 0, "L", false, 0, "")
		pdf.SetXY(margin+contentW/2, y)
		pdf.CellFormat(contentW/2, lh, right, "", 0, "R", false, 0, "")
	}
	down := func(mm float64) { pdf.SetY(pdf.GetY() + mm) }
	center := func(s string, size float64) { boldText(s, size, "C") }
	normalText := func(s string, size float64, align string) {
		pdf.SetFont(bodyFont, "", size)
		pdf.SetXY(margin, pdf.GetY())
		pdf.CellFormat(contentW, lh, s, "", 0, align, false, 0, "")
	}
	centerNormal := func(s string, size float64) { normalText(s, size, "C") }
	centerWithFont := func(s, font string, size float64) {
		pdf.SetFont(font, "B", size)
		pdf.SetXY(margin, pdf.GetY())
		pdf.CellFormat(contentW, lh, s, "", 0, "C", false, 0, "")
		// Slight overprint makes the preview title match the heavier printed title.
		pdf.SetXY(margin+0.18, pdf.GetY())
		pdf.CellFormat(contentW, lh, s, "", 0, "C", false, 0, "")
	}
	centerWithFontNormal := func(s, font string, size float64) {
		pdf.SetFont(font, "", size)
		pdf.SetXY(margin, pdf.GetY())
		pdf.CellFormat(contentW, lh, s, "", 0, "C", false, 0, "")
	}
	wrapLines := func(s string, size float64) []string {
		pdf.SetFont(bodyFont, "", size)
		lines := make([]string, 0)
		for _, paragraph := range strings.Split(s, "\n") {
			words := strings.Fields(paragraph)
			if len(words) == 0 {
				lines = append(lines, "")
				continue
			}
			line := words[0]
			for _, word := range words[1:] {
				candidate := line + " " + word
				if pdf.GetStringWidth(candidate) > contentW && line != "" {
					lines = append(lines, line)
					line = word
				} else {
					line = candidate
				}
			}
			lines = append(lines, line)
		}
		return lines
	}
	centerWrapped := func(s string, size float64, spacing float64) {
		for _, line := range wrapLines(s, size) {
			center(line, size)
			down(spacing)
		}
	}
	dashedRule := func() {
		pdf.SetDashPattern([]float64{1.0, 0.9}, 0)
		pdf.SetLineWidth(0.3)
		y := pdf.GetY()
		pdf.Line(margin, y, pageW-margin, y)
		pdf.SetDashPattern([]float64{}, 0)
	}

	// ---- Header --------------------------------------------------------
	pdf.SetY(2)
	centerWithFont(data.StoreName, titleFont, titleSize)
	down(titleSize/3 - 0.5)
	branchPhone := data.BranchPhone
	if branchPhone == "" {
		branchPhone = data.Phone
	}
	if data.Branch != "" {
		centerWithFontNormal(data.Branch, titleFont, titleSize-8)
		down(lh + 0.5)
	}
	if branchPhone != "" {
		centerWithFontNormal(branchPhone, titleFont, titleSize-8)
		down(lh + 2)
	}
	// Receipt description temporarily disabled.
	// if data.Description != "" {
	// 	centerNormalWrapped(data.Description, bodySize-1, lh)
	// }
	down(4)

	center(receiptInvoiceLabel(data.ReceiptNo)+": "+data.ReceiptNo, bodySize-1)
	down(lh)

	if data.Website != "" {
		contact := strings.TrimSpace(data.Website)
		if contact != "" {
			center(contact, bodySize-1.5)
			down(lh)
		}
	}

	down(1)
	dashedRule()
	down(2.5)

	// ---- Sale / customer -------------------------------------------------
	normalText("Customer: "+data.Customer, bodySize-1, "L")
	down(lh)
	normalText("Salesperson: "+formatReceiptSalesperson(data.Salesperson), bodySize-1, "L")
	down(lh)
	normalText("Date: "+currentReceiptDate(), bodySize-1, "L")
	down(lh - 0.5)
	if strings.TrimSpace(data.Notes) != "" {
		down(1.5)
		normalText("Note: "+strings.TrimSpace(data.Notes), bodySize-1.5, "L")
		down(lh)
	}
	down(1)
	dashedRule()
	down(2.5)

	// ---- Items ------------------------------------------------------------
	twoCol("ITEM", "TOTAL", bodySize)
	down(lh - 0.6)
	dashedRule()
	down(2.5)

	var subtotal, totalDiscount float64
	for i, it := range data.Items {
		gross := it.Qty * it.Price
		discAmt := gross * (it.Discount / 100.0)
		lineTotal := gross - discAmt
		subtotal += gross
		totalDiscount += discAmt

		price := fmt.Sprintf("$%.2f", lineTotal)
		pdf.SetFont(bodyFont, "B", bodySize)
		priceW := pdf.GetStringWidth(price)
		nameMaxW := contentW - priceW - 2.5
		nameLines := wrapToWidth(pdf, it.Name, nameMaxW, contentW)

		for idx, line := range nameLines {
			pdf.SetFont(bodyFont, "B", bodySize)
			pdf.SetXY(margin, pdf.GetY())
			if idx == 0 {
				pdf.CellFormat(nameMaxW, lh, line, "", 0, "L", false, 0, "")
				pdf.SetXY(margin+contentW-priceW, pdf.GetY())
				pdf.CellFormat(priceW, lh, price, "", 0, "R", false, 0, "")
			} else {
				pdf.CellFormat(contentW, lh, line, "", 0, "L", false, 0, "")
			}
			down(lh - 0.3)
		}

		// Code + qty/price on a single condensed detail line.
		pdf.SetFont(bodyFont, "", bodySize-1.5)
		detail := fmt.Sprintf("%.0f x $%.2f", it.Qty, it.Price)
		if it.Discount > 0 {
			detail += fmt.Sprintf("  (-%.0f%%)", it.Discount)
		}
		if it.Code != "" {
			detail = "Code: " + it.Code + "   " + detail
		}
		pdf.SetXY(margin+2, pdf.GetY())
		pdf.CellFormat(contentW-2, lh-0.6, detail, "", 0, "L", false, 0, "")
		down(lh - 0.3)

		if i < len(data.Items)-1 {
			down(1.2)
		}
	}
	down(1.5)

	dashedRule()
	down(2.5)
	taxable := subtotal - totalDiscount
	grandTotal := float64(data.GrandTotalCents) / 100
	tax := grandTotal - taxable

	twoCol("Subtotal", fmt.Sprintf("$%.2f", subtotal), bodySize-1)
	down(lh - 0.4)
	if totalDiscount > 0 {
		twoCol("Discount", fmt.Sprintf("-$%.2f", totalDiscount), bodySize-1)
		down(lh - 0.4)
	}
	if math.Abs(tax) > 0.005 {
		twoCol("Tax", fmt.Sprintf("$%.2f", tax), bodySize-1)
		down(lh - 0.4)
	}
	down(1)
	dashedRule()
	down(3)
	drawDottedPDFBox(pdf, margin, pdf.GetY(), contentW, 10.5)
	down(1)
	twoCol("TOTAL", fmt.Sprintf("$%.2f", grandTotal), bodySize+2)
	down(lh + 1)

	rate := data.ExchangeRate
	if rate <= 0 {
		rate = 90000
	}
	roundedTotalLBP, roundErr := roundLBPFromCents(data.GrandTotalCents, rate, data.RoundingMode)
	if roundErr != nil {
		roundedTotalLBP = int64(math.Round(grandTotal*float64(rate)/float64(lbpRoundingUnit))) * lbpRoundingUnit
	}
	twoCol("LBP TOTAL", fmt.Sprintf("%s L.L.", formatInt(roundedTotalLBP)), bodySize-1)
	down(lh + 1)

	// ---- Payments -----------------------------------------------------
	if len(data.Payments) > 0 {
		center("PAYMENT DETAILS", bodySize-1)
		down(lh)
		for _, payment := range data.Payments {
			label := payment.AccountName
			if label == "" {
				label = "Cash"
			}
			if payment.EntryType == "change" {
				label = "Change - " + label
			} else {
				label = "Received - " + label
			}
			amount := formatReceiptPayment(payment, rate, data.RoundingMode)
			twoCol(label, amount, bodySize-1.5)
			down(lh - 0.4)
		}
		down(1)
	}

	// ---- Policy ---------------------------------------------------------
	if len(data.Payments) > 0 {
		dashedRule()
		down(2.5)
	}
	center("RETURN & EXCHANGE POLICY", bodySize-1.5)
	down(lh)
	policy := data.ReturnPolicy
	if policy == "" {
		policy = "Exchanges accepted within 3 days. Items must be unworn, unused, unwashed, undamaged, with tags. Present this receipt."
	}
	centerWrapped(policy, bodySize-2.5, lh-0.5)
	down(1)
	dashedRule()
	down(3)

	// ---- Social footer: single row of icons, one handle below ----------
	socialItems := socialFooterItems(data.SocialPlatforms, data.SocialHandles)
	if len(socialItems) > 0 {
		const iconSize = 5.0
		const gap = 2.5
		label := "Follow us on:"
		pdf.SetFont(bodyFont, "B", bodySize-1)
		labelW := pdf.GetStringWidth(label) + 2
		rowW := labelW + float64(len(socialItems))*iconSize + float64(len(socialItems)-1)*gap
		startX := margin + (contentW-rowW)/2
		if startX < margin {
			startX = margin
		}

		pdf.SetXY(startX, pdf.GetY())
		pdf.CellFormat(labelW, 5, label, "", 0, "L", false, 0, "")
		drawSocialFooterRow(pdf, socialItems, startX+labelW, pdf.GetY(), iconSize, gap)
		down(iconSize + 2)

		handle := socialItems[0].handle
		centerNormal(handle, bodySize-1.5)
		down(lh)
		down(1)
	}

	down(9)
	center("Thank you for shopping with us!", bodySize-1)
	down(9)
	// PDF barcode temporarily disabled.
	// drawPDFBarcode(pdf, margin, pdf.GetY(), contentW, 7, data.ReceiptNo)
	// down(9)

	down(lh + 12)

	return pdf
}

// wrapToWidth wraps s into lines, keeping the first line within firstLineW
// (to leave room for a trailing price) and subsequent lines within fullW.
func wrapToWidth(pdf *fpdf.Fpdf, s string, firstLineW, fullW float64) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	lines := make([]string, 0, 2)
	for _, word := range words {
		limit := fullW
		if len(lines) == 0 {
			limit = firstLineW
		}
		if len(lines) > 0 {
			candidate := lines[len(lines)-1] + " " + word
			if pdf.GetStringWidth(candidate) <= limit {
				lines[len(lines)-1] = candidate
				continue
			}
		}
		if pdf.GetStringWidth(word) <= limit {
			lines = append(lines, word)
			continue
		}
		for _, chunk := range splitPDFWord(pdf, word, limit) {
			lines = append(lines, chunk)
			limit = fullW
		}
	}
	return lines
}

func splitPDFWord(pdf *fpdf.Fpdf, word string, width float64) []string {
	chunks := make([]string, 0, 2)
	chunk := ""
	for _, r := range word {
		candidate := chunk + string(r)
		if chunk != "" && pdf.GetStringWidth(candidate) > width {
			chunks = append(chunks, chunk)
			chunk = string(r)
		} else {
			chunk = candidate
		}
	}
	if chunk != "" {
		chunks = append(chunks, chunk)
	}
	return chunks
}

func filterEmpty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

func drawDottedPDFBox(pdf *fpdf.Fpdf, x, y, width, height float64) {
	pdf.SetDashPattern([]float64{1.0, 0.9}, 0)
	pdf.SetLineWidth(0.3)
	pdf.Rect(x, y, width, height, "D")
	pdf.SetDashPattern([]float64{}, 0)
}

func drawPDFBarcode(pdf *fpdf.Fpdf, x, y, width, height float64, value string) {
	seed := int64(0)
	for _, char := range value {
		seed += int64(char)
	}
	rnd := rand.New(rand.NewSource(seed))
	barWidths := []float64{0.35, 0.55, 0.8, 1.05}
	for cx := x; cx < x+width; {
		barWidth := barWidths[rnd.Intn(len(barWidths))]
		if rnd.Float64() > 0.35 {
			pdf.SetLineWidth(barWidth)
			pdf.Line(cx, y, cx, y+height)
		}
		cx += barWidth + 0.3
	}
}

func formatInt(value int64) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	digits := fmt.Sprintf("%d", value)
	for i := len(digits) - 3; i > 0; i -= 3 {
		digits = digits[:i] + "," + digits[i:]
	}
	return sign + digits
}

func formatReceiptPayment(payment receiptPayment, exchangeRate int64, roundingMode string) string {
	amount := payment.AmountCents
	if amount < 0 {
		amount = -amount
	}
	if payment.CurrencyCode == "LBP" {
		if rounded, err := roundLBPFromCents(amount, exchangeRate, roundingMode); err == nil {
			return fmt.Sprintf("%s LBP", formatInt(rounded))
		}
		return "0 LBP"
	}
	return fmt.Sprintf("$%.2f", float64(amount)/100)
}

type socialFooterItem struct {
	platform string
	handle   string
}

func socialFooterItems(platforms []string, handles map[string]string) []socialFooterItem {
	items := make([]socialFooterItem, 0, len(platforms))
	for _, platform := range platforms {
		handle := strings.TrimSpace(handles[platform])
		if handle == "" {
			continue
		}
		items = append(items, socialFooterItem{platform: platform, handle: handle})
	}
	return items
}

// drawSocialFooterRow draws all platform icons in a single horizontal row
// starting at startX, vertically aligned at top. Pair with a centered
// handle line drawn separately beneath it.
func drawSocialFooterRow(pdf *fpdf.Fpdf, items []socialFooterItem, startX, top, iconSize, gap float64) {
	x := startX
	for _, item := range items {
		iconBytes := socialPlatformIconBytes(item.platform)
		if len(iconBytes) == 0 {
			x += iconSize + gap
			continue
		}
		iconName := "social-" + item.platform
		opts := fpdf.ImageOptions{ImageType: "png", ReadDpi: true}
		pdf.RegisterImageOptionsReader(iconName, opts, bytes.NewReader(iconBytes))
		pdf.ImageOptions(iconName, x, top, iconSize, iconSize, false, opts, 0, "")
		x += iconSize + gap
	}
}

func socialPlatformIconBytes(platform string) []byte {
	switch platform {
	case "facebook":
		return facebookIcon
	case "instagram":
		return instagramIcon
	case "threads":
		return threadsIcon
	case "tiktok":
		return tiktokIcon
	default:
		return nil
	}
}
