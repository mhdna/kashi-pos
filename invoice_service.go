package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

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
