package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
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
	Color    string  `json:"color"`
	Size     string  `json:"size"`
	Price    float64 `json:"price"`
	Discount float64 `json:"discount"`
	Qty      float64 `json:"qty"`
}

func (a *App) HoldInvoice(snapshotJSON string) (int64, error) {
	if a.store == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	var snapshot struct {
		Items []struct {
			Code    string  `json:"code"`
			Barcode string  `json:"barcode"`
			Qty     float64 `json:"qty"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil {
		return 0, err
	}
	if len(snapshot.Items) == 0 {
		return 0, fmt.Errorf("cannot hold an empty invoice")
	}
	inv, err := a.store.CreateHoldInvoice(a.ctx, snapshotJSON)
	if err != nil {
		return 0, err
	}
	for _, it := range snapshot.Items {
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
		if strings.TrimSpace(inv.SnapshotJson) != "" {
			var snapshot map[string]any
			if json.Unmarshal([]byte(inv.SnapshotJson), &snapshot) == nil {
				snapshot["id"] = inv.ID
				snapshot["createdAt"] = inv.CreatedAt
				if items, ok := snapshot["items"].([]any); ok {
					snapshot["itemCount"] = len(items)
				}
				result[i] = snapshot
				continue
			}
		}
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
			"color": product.Color, "size": product.Size,
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
		paymentList = append(paymentList, map[string]any{
			"accountId": payment.CashboxID, "accountName": account.Name,
			"currencyCode": account.CurrencyCode, "amount": payment.Amount,
		})
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
	client, err := a.store.GetClientByID(a.ctx, clientID)
	if err != nil {
		return err
	}
	if _, accountErr := a.store.GetClientAccountEntryByInvoiceID(a.ctx, invoiceID); accountErr == nil &&
		strings.Contains(strings.ToLower(client.Name), "walk-in") {
		return fmt.Errorf("a named client is required for an on-account invoice")
	}
	return a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		if err := q.UpdateInvoiceClient(a.ctx, db.UpdateInvoiceClientParams{ClientID: clientID, ID: invoiceID}); err != nil {
			return err
		}
		return q.UpdateClientAccountOwnerByInvoice(a.ctx, db.UpdateClientAccountOwnerByInvoiceParams{
			ClientID: clientID, InvoiceID: invoiceID,
		})
	})
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
	if strings.EqualFold(strings.TrimSpace(account.Name), "On Account") {
		return fmt.Errorf("use checkout to create on-account debt")
	}
	inv, err := a.store.GetInvoiceByID(a.ctx, invoiceID)
	if err != nil {
		return err
	}
	oldPayments, err := a.store.ListInvoicePayments(a.ctx, invoiceID)
	if err != nil {
		return err
	}
	for _, payment := range oldPayments {
		oldAccount, lookupErr := a.store.GetCashboxAccountByID(a.ctx, payment.CashboxID)
		if lookupErr == nil && strings.EqualFold(strings.TrimSpace(oldAccount.Name), "On Account") {
			return fmt.Errorf("use the client account screen to pay an on-account invoice")
		}
	}
	settlementAmount := inv.GrandTotal
	if len(oldPayments) > 0 {
		settlementAmount = 0
		for _, payment := range oldPayments {
			if (payment.Amount > 0 && settlementAmount > math.MaxInt64-payment.Amount) ||
				(payment.Amount < 0 && settlementAmount < math.MinInt64-payment.Amount) {
				return fmt.Errorf("settlement amount is too large")
			}
			settlementAmount += payment.Amount
		}
	}
	if settlementAmount <= 0 {
		return fmt.Errorf("invoice has no positive settlement")
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
		if err := q.CreateInvoicePayment(a.ctx, db.CreateInvoicePaymentParams{InvoiceID: invoiceID, CashboxID: cashboxID, Amount: settlementAmount}); err != nil {
			return err
		}
		if err := q.CreateInvoicePaymentEntry(a.ctx, db.CreateInvoicePaymentEntryParams{InvoiceID: invoiceID, CashboxID: cashboxID, Amount: settlementAmount, EntryType: "payment"}); err != nil {
			return err
		}
		if _, err := q.AddCashboxAccountBalance(a.ctx, db.AddCashboxAccountBalanceParams{AccountID: cashboxID, ShiftID: inv.ShiftID, Balance: settlementAmount}); err != nil {
			return err
		}
		return q.UpdateInvoiceCashbox(a.ctx, db.UpdateInvoiceCashboxParams{CashboxID: cashboxID, ID: invoiceID})
	})
}

// ChangeInvoiceSettlementAmount moves part of an invoice's settled accounting
// value from one payment account to another. Amounts are always expressed in
// the application's accounting currency (USD cents); account currencies only
// affect how the frontend accepts and displays the amount.
func (a *App) ChangeInvoiceSettlementAmount(invoiceID, fromCashboxID, toCashboxID, amountCents int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if fromCashboxID <= 0 || toCashboxID <= 0 || fromCashboxID == toCashboxID {
		return fmt.Errorf("choose different settlement accounts")
	}
	if amountCents <= 0 {
		return fmt.Errorf("enter a positive settlement amount")
	}
	if _, err := a.store.GetCashboxAccountByID(a.ctx, fromCashboxID); err != nil {
		return fmt.Errorf("source settlement account not found")
	}
	fromAccount, _ := a.store.GetCashboxAccountByID(a.ctx, fromCashboxID)
	toAccount, err := a.store.GetCashboxAccountByID(a.ctx, toCashboxID)
	if err != nil {
		return fmt.Errorf("destination settlement account not found")
	}
	if strings.EqualFold(strings.TrimSpace(fromAccount.Name), "On Account") ||
		strings.EqualFold(strings.TrimSpace(toAccount.Name), "On Account") {
		return fmt.Errorf("use the client account screen for on-account invoices")
	}
	inv, err := a.store.GetInvoiceByID(a.ctx, invoiceID)
	if err != nil {
		return err
	}
	oldPayments, err := a.store.ListInvoicePayments(a.ctx, invoiceID)
	if err != nil {
		return err
	}

	updatedPayments := make(map[int64]int64, len(oldPayments)+1)
	for _, payment := range oldPayments {
		updatedPayments[payment.CashboxID] = payment.Amount
	}
	sourceAmount := updatedPayments[fromCashboxID]
	if sourceAmount <= 0 {
		return fmt.Errorf("invoice has no positive settlement in the source account")
	}
	if amountCents > sourceAmount {
		return fmt.Errorf("settlement amount exceeds the source payment")
	}
	destinationAmount := updatedPayments[toCashboxID]
	if destinationAmount > math.MaxInt64-amountCents {
		return fmt.Errorf("settlement amount is too large")
	}
	updatedPayments[fromCashboxID] = sourceAmount - amountCents
	updatedPayments[toCashboxID] = destinationAmount + amountCents

	return a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		if _, err := q.AddCashboxAccountBalance(a.ctx, db.AddCashboxAccountBalanceParams{
			AccountID: fromCashboxID, ShiftID: inv.ShiftID, Balance: -amountCents,
		}); err != nil {
			return fmt.Errorf("reduce source settlement balance: %w", err)
		}
		if _, err := q.AddCashboxAccountBalance(a.ctx, db.AddCashboxAccountBalanceParams{
			AccountID: toCashboxID, ShiftID: inv.ShiftID, Balance: amountCents,
		}); err != nil {
			return fmt.Errorf("increase destination settlement balance: %w", err)
		}
		if err := q.DeleteInvoicePayments(a.ctx, invoiceID); err != nil {
			return err
		}
		if err := q.DeleteInvoicePaymentEntries(a.ctx, invoiceID); err != nil {
			return err
		}
		for accountID, amount := range updatedPayments {
			if amount == 0 {
				continue
			}
			if err := q.CreateInvoicePayment(a.ctx, db.CreateInvoicePaymentParams{
				InvoiceID: invoiceID, CashboxID: accountID, Amount: amount,
			}); err != nil {
				return fmt.Errorf("record settlement: %w", err)
			}
			entryType := "payment"
			if amount < 0 {
				entryType = "change"
			}
			if err := q.CreateInvoicePaymentEntry(a.ctx, db.CreateInvoicePaymentEntryParams{
				InvoiceID: invoiceID, CashboxID: accountID, Amount: amount, EntryType: entryType,
			}); err != nil {
				return fmt.Errorf("record settlement entry: %w", err)
			}
		}
		if inv.CashboxID == fromCashboxID && updatedPayments[fromCashboxID] == 0 {
			return q.UpdateInvoiceCashbox(a.ctx, db.UpdateInvoiceCashboxParams{
				CashboxID: toCashboxID, ID: invoiceID,
			})
		}
		return nil
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
		items = append(items, invoiceItem{Name: raw["name"].(string), Code: raw["code"].(string), Barcode: raw["barcode"].(string), Category: raw["category"].(string), Color: databaseString(raw["color"]), Size: databaseString(raw["size"]), Price: raw["price"].(float64), Discount: float64(raw["discount"].(int64)), Qty: float64(raw["qty"].(int64))})
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return err
	}
	grandTotal, roundErr := roundCentsByCurrency(details["grandTotal"].(int64), settings.ExchangeRate, settings.RoundingCurrency, settings.RoundingMode)
	if roundErr != nil {
		return fmt.Errorf("round receipt total: %w", roundErr)
	}
	marketName := settings.MarketName
	if marketName == "" {
		marketName = "Market"
	}
	return a.printReceipt(items, marketName, settings.BranchName, settings.MarketPhone, settings.BranchPhone, settings.MarketDescription, settings.CounterNumber, details["salespersonName"].(string), details["clientName"].(string), details["notes"].(string), details["invoiceCode"].(string), settings.ReceiptPrinter, grandTotal, payments, settings.ExchangeRate, settings.RoundingMode, settings.ReceiptWidth, settings.ReceiptHeight, settings.ReturnPolicy, settings.Website, settings.Instagram, decodeSocialPlatforms(settings.SocialPlatforms), decodeSocialHandles(settings.SocialHandles), settings.ReceiptFont, settings.ReceiptBodyFont, settings.ReceiptTitleSize, settings.ReceiptBodySize, settings.ReceiptCutoff != 0)
}

func (a *App) printReceipt(items []invoiceItem, storeName, branch, phone, branchPhone, description, registerName, salesperson, customer, notes, receiptNo, printerName string, grandTotalCents int64, payments []checkoutPayment, exchangeRate int64, roundingMode string, receiptWidth, receiptHeight int64, returnPolicy, website, instagram string, socialPlatforms []string, socialHandles map[string]string, font, bodyFont string, titleSize, bodySize float64, cutoff bool) error {
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
		receiptPayments = append(receiptPayments, receiptPayment{
			AccountID:    account.ID,
			AccountName:  account.Name,
			CurrencyCode: account.CurrencyCode,
			AmountCents:  payment.AmountCents,
			EntryType:    payment.EntryType,
		})
	}
	receiptPayments = aggregateReceiptPayments(receiptPayments)
	return printReceiptToPrinter(printReceiptData{
		Items:           items,
		StoreName:       storeName,
		Branch:          branch,
		Phone:           phone,
		BranchPhone:     branchPhone,
		Description:     description,
		RegisterName:    strings.TrimSpace(registerName),
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
