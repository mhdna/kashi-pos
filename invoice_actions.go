package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/go-pdf/fpdf"

	db "kashi-pos/db/sqlc"
)

type invoiceItem struct {
	Name     string  `json:"name"`
	Code     string  `json:"code"`
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
		Code string  `json:"code"`
		Qty  float64 `json:"qty"`
	}
	if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
		return 0, err
	}
	inv, err := a.store.CreateHoldInvoice(a.ctx)
	if err != nil {
		return 0, err
	}
	for _, it := range items {
		product, err := a.store.GetProductByCode(a.ctx, it.Code)
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
				"price":    float64(it.Price) / 100,
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
		if err := q.DeleteInvoicePayments(a.ctx, id); err != nil { return err }
		if err := q.DeleteInvoicePaymentEntries(a.ctx, id); err != nil { return err }
		if err := q.DeleteInvoiceProducts(a.ctx, id); err != nil { return err }
		if err := q.DeleteSalesInvoice(a.ctx, id); err != nil { return err }
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

func (a *App) CheckoutInvoice(itemsJSON, paymentsJSON string, clientID int64, storeName, branch, phone, salesperson, receiptPrinter string) (map[string]any, error) {
	result, err := a.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: itemsJSON, PaymentsJSON: paymentsJSON, ClientID: clientID})
	if err != nil {
		return nil, fmt.Errorf("invoice creation failed: %w", err)
	}

	response := map[string]any{
		"invoiceCode":    result.InvoiceCode,
		"receiptPrinted": false,
	}
	settings, settingsErr := a.store.GetSettings(a.ctx)
	if settingsErr == nil && settings.ReceiptEnabled != 0 {
		if err := a.printReceipt(result.Items, settings.BranchName, settings.BranchName, phone, salesperson, result.InvoiceCode, settings.ReceiptPrinter, result.GrandTotal, result.Payments, settings.ExchangeRate, settings.RoundingMode, settings.ReceiptWidth, settings.ReceiptHeight); err != nil {
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
			"name": product.Name, "code": product.Code, "category": product.Description,
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
	clientName := "Walk-in customer"
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
		"createdAt": inv.CreatedAt, "items": items, "payments": paymentList,
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
	details, err := a.GetInvoiceDetails(id)
	if err != nil {
		return err
	}
	items := make([]invoiceItem, 0)
	for _, raw := range details["items"].([]map[string]any) {
		items = append(items, invoiceItem{Name: raw["name"].(string), Code: raw["code"].(string), Category: raw["category"].(string), Price: raw["price"].(float64), Discount: float64(raw["discount"].(int64)), Qty: float64(raw["qty"].(int64))})
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return err
	}
	return a.printReceipt(items, settings.BranchName, settings.BranchName, details["clientPhone"].(string), details["salespersonName"].(string), details["invoiceCode"].(string), settings.ReceiptPrinter, details["grandTotal"].(int64), nil, settings.ExchangeRate, settings.RoundingMode, settings.ReceiptWidth, settings.ReceiptHeight)
}

func (a *App) printReceipt(items []invoiceItem, storeName, branch, phone, salesperson, receiptNo, printerName string, grandTotalCents int64, payments []checkoutPayment, exchangeRate int64, roundingMode string, receiptWidth, receiptHeight int64) error {
	if printerName == "" {
		return fmt.Errorf("no receipt printer configured")
	}

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
		Salesperson:     salesperson,
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

func buildReceiptPDF(items []invoiceItem, storeName, branch, phone, salesperson, receiptNo string, grandTotalCents int64, payments []receiptPayment, exchangeRate int64, roundingMode string, receiptWidth, receiptHeight int64) *fpdf.Fpdf {
	const (
		defaultPageW = 80.0
		defaultPageH = 297.0
		margin       = 4.0
	)
	pageW := float64(receiptWidth)
	if pageW <= 0 {
		pageW = defaultPageW
	}
	pageH := float64(receiptHeight)
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

	const fontFamily = "Courier"

	boldText := func(s string, size float64, align string) {
		pdf.SetFont(fontFamily, "B", size)
		w := contentW
		if align == "C" {
			pdf.SetXY(margin, pdf.GetY())
			w = contentW
		}
		pdf.CellFormat(w, 4, s, "", 0, align, false, 0, "")
	}

	twoCol := func(left, right string, size float64) {
		pdf.SetFont(fontFamily, "B", size)
		y := pdf.GetY()
		pdf.SetXY(margin, y)
		pdf.CellFormat(contentW/2, 4, left, "", 0, "L", false, 0, "")
		pdf.SetXY(margin+contentW/2, y)
		pdf.CellFormat(contentW/2, 4, right, "", 0, "R", false, 0, "")
	}

	center := func(s string, size float64) { boldText(s, size, "C") }
	down := func(mm float64) { pdf.SetY(pdf.GetY() + mm) }

	dashedRule := func() {
		pdf.SetDashPattern([]float64{1.1, 1.0}, 0)
		pdf.SetLineWidth(0.35)
		y := pdf.GetY()
		pdf.Line(margin, y, pageW-margin, y)
		pdf.SetDashPattern([]float64{}, 0)
	}

	pdf.SetY(8)
	center(storeName, 21)
	down(8)
	center(branch, 12)
	down(4)
	center(phone, 12)
	down(5)
	dashedRule()
	down(4)

	twoCol("Receipt: "+receiptNo, "", 11)
	down(4)
	now := time.Now()
	twoCol("Date: "+now.Format("02/01/2006"), "Time: "+now.Format("15:04"), 11)
	down(4)
	twoCol(salesperson, "", 11)
	down(5)
	dashedRule()
	down(4)

	twoCol("ITEM", "TOTAL", 11)
	down(3)
	dashedRule()
	down(4)

	var subtotal, totalDiscount float64
	for _, it := range items {
		gross := it.Qty * it.Price
		discAmt := gross * (it.Discount / 100.0)
		lineTotal := gross - discAmt
		subtotal += gross
		totalDiscount += discAmt

		twoCol(it.Name, fmt.Sprintf("$%.2f", lineTotal), 11)
		down(4)

		pdf.SetFont(fontFamily, "", 10)
		qtyLine := fmt.Sprintf("%.0f x $%.2f", it.Qty, it.Price)
		if it.Discount > 0 {
			qtyLine += fmt.Sprintf("  (-%.0f%% = -$%.2f)", it.Discount, discAmt)
		}
		pdf.SetXY(margin+2, pdf.GetY())
		pdf.CellFormat(contentW-2, 4, qtyLine, "", 0, "L", false, 0, "")
		down(5)
	}

	dashedRule()
	down(4)
	taxable := subtotal - totalDiscount
	grandTotal := float64(grandTotalCents) / 100
	tax := grandTotal - taxable

	twoCol("Subtotal", fmt.Sprintf("$%.2f", subtotal), 11)
	down(4)
	twoCol("Discount", fmt.Sprintf("-$%.2f", totalDiscount), 11)
	down(5)
	twoCol("Tax", fmt.Sprintf("$%.2f", tax), 11)
	down(5)
	dashedRule()
	down(5)
	twoCol("TOTAL", fmt.Sprintf("$%.2f", grandTotal), 13)
	down(5)
	rate := exchangeRate
	if rate <= 0 {
		rate = 90000
	}
	roundedTotalLBP, roundErr := roundLBPFromCents(grandTotalCents, rate, roundingMode)
	if roundErr != nil {
		roundedTotalLBP = int64(math.Round(grandTotal*float64(rate)/float64(lbpRoundingUnit))) * lbpRoundingUnit
	}
	twoCol("LBP TOTAL", fmt.Sprintf("%s L.L.", formatInt(roundedTotalLBP)), 11)
	down(6)
	if len(payments) > 0 {
		dashedRule()
		down(4)
		center("PAYMENT DETAILS", 11)
		down(4)
		for _, payment := range payments {
			label := payment.AccountName
			if label == "" {
				label = "Cash"
			}
			if payment.EntryType == "change" {
				label = "Change returned - " + label
			} else {
				label = "Cash received - " + label
			}
			amount := formatReceiptPayment(payment, rate, roundingMode)
			twoCol(label, amount, 10)
			down(4)
		}
	}
	dashedRule()
	down(4)
	center("RETURN & EXCHANGE POLICY", 10)
	down(4)
	for _, line := range []string{
		"Exchanges accepted within 3 days.",
		"Items must be unworn, unused,",
		"unwashed, undamaged, with tags.",
		"Present this receipt.",
		"Subject to availability.",
	} {
		center(line, 8.5)
		down(3.5)
	}
	dashedRule()
	down(4)
	center("Thank you for shopping with us!", 10)
	down(4)
	center(receiptNo, 11)
	down(8)

	seed := int64(0)
	for _, c := range receiptNo {
		seed += int64(c)
	}
	rnd := rand.New(rand.NewSource(seed))
	widths := []float64{0.35, 0.55, 0.8, 1.05}
	cx := margin
	top := pdf.GetY()
	const barH = 9.0
	for cx < margin+contentW {
		w := widths[rnd.Intn(len(widths))]
		if rnd.Float64() > 0.35 {
			pdf.SetLineWidth(w)
			pdf.Line(cx, top, cx, top+barH)
		}
		cx += w + 0.3
	}
	down(barH + 3)
	center(receiptNo, 11)
	down(6)

	return pdf
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
