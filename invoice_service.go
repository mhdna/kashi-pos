package main

import (
	"database/sql"
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
	invoice, err := a.store.GetInvoiceByID(a.ctx, id)
	if err != nil {
		return err
	}
	if invoice.CancelledAt.Valid {
		return fmt.Errorf("a cancelled invoice can't be deleted")
	}
	// Only an invoice kashi never accepted can go: kashi has no way to hear
	// about a delete, so removing an uploaded one would leave its sale counted
	// there for good. The fix for a mistaken sale is CancelInvoice.
	return a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		if returns, err := q.CountReturnsForSale(a.ctx, id); err != nil {
			return err
		} else if returns > 0 {
			return fmt.Errorf("this sale has a return or exchange against it -- cancel it instead")
		}
		if coupons, err := q.CountCouponsRedeemedOnInvoice(a.ctx, sql.NullInt64{Int64: id, Valid: true}); err != nil {
			return err
		} else if coupons > 0 {
			return fmt.Errorf("a coupon was redeemed on this invoice -- cancel it instead")
		}
		ref, err := q.GetInvoiceClientRef(a.ctx, id)
		switch {
		case err == nil:
			// A ref with no upload row, or a synced one, is already on kashi
			// (pushed, or pulled down from it as history).
			status, statusErr := q.GetSyncOutboxStatusByClientRef(a.ctx, ref)
			if statusErr == sql.ErrNoRows || (statusErr == nil && status == "synced") {
				return fmt.Errorf("this invoice is already uploaded to kashi -- cancel it instead")
			}
			if statusErr != nil {
				return statusErr
			}
			if err := q.DeleteUnsyncedSyncOutboxByClientRef(a.ctx, ref); err != nil {
				return err
			}
			if err := q.DeleteInvoiceClientRef(a.ctx, id); err != nil {
				return err
			}
		case err != sql.ErrNoRows:
			return err
		}
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
	result, err := a.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: itemsJSON, PaymentsJSON: paymentsJSON, ClientID: clientID, Notes: notes, Salesperson: strings.TrimSpace(salesperson)})
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
	invoiceClient, invoiceClientErr := a.store.GetClientByID(a.ctx, clientID)

	// Post-sale WhatsApp "thank you / please rate us" message. Best-effort and
	// off the checkout path -- see whatsAppThankYou.
	if invoiceClientErr == nil {
		go a.whatsAppThankYou(result.InvoiceCode, result.GrandTotal, invoiceClient, result.Items)
	}

	settings, settingsErr := a.store.GetSettings(a.ctx)
	if settingsErr == nil && settings.ReceiptEnabled != 0 {
		marketName := settings.MarketName
		if marketName == "" {
			marketName = "Market"
		}
		customer := "Walk-in"
		if invoiceClientErr == nil && invoiceClient.Name != "" && !isWalkInClient(invoiceClient) {
			customer = invoiceClient.Name
		}
		roundingCurrency, currErr := resolveCurrency(a.ctx, a.store.Queries, settings.RoundingCurrency)
		if currErr != nil {
			response["receiptError"] = currErr.Error()
			return response, nil
		}
		if err := a.printReceipt(result.Items, marketName, settings.BranchName, settings.MarketPhone, settings.BranchPhone, settings.MarketDescription, settings.CounterNumber, salesperson, customer, result.Notes, result.InvoiceCode, settings.ReceiptPrinter, result.GrandTotal, result.TaxCents, result.Payments, roundingCurrency, settings.RoundingMode, settings.ReceiptWidth, settings.ReceiptHeight, settings.ReturnPolicy, settings.Website, settings.Instagram, decodeSocialPlatforms(settings.SocialPlatforms), decodeSocialHandles(settings.SocialHandles), settings.ReceiptFont, settings.ReceiptBodyFont, settings.ReceiptTitleSize, settings.ReceiptBodySize, settings.ReceiptCutoff != 0); err != nil {
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
	payments, err := a.store.ListInvoicePayments(a.ctx, id)
	if err != nil {
		return nil, err
	}

	// Batch every per-line and per-payment lookup into one query each,
	// instead of up to four product/salesperson/price-list/discount-list
	// round trips per line plus a cashbox lookup per payment.
	productIDSet := make(map[int64]struct{}, len(products))
	for _, line := range products {
		productIDSet[line.ProductID] = struct{}{}
	}
	productIDs := make([]int64, 0, len(productIDSet))
	for productID := range productIDSet {
		productIDs = append(productIDs, productID)
	}
	productByID := make(map[int64]db.Product, len(productIDs))
	if len(productIDs) > 0 {
		productRows, productErr := a.store.ListProductsByIDs(a.ctx, productIDs)
		if productErr != nil {
			return nil, productErr
		}
		for _, product := range productRows {
			productByID[product.ID] = product
		}
	}

	salespersonNameByID := make(map[int64]string)
	if salespeople, listErr := a.store.ListSalespersons(a.ctx); listErr == nil {
		for _, person := range salespeople {
			salespersonNameByID[person.ID] = person.FullName
		}
	}
	priceListNameByID := make(map[int64]string)
	if lists, listErr := a.store.ListPriceLists(a.ctx); listErr == nil {
		for _, list := range lists {
			priceListNameByID[list.ID] = list.Name
		}
	}
	discountListNameByID := make(map[int64]string)
	if lists, listErr := a.store.ListDiscountLists(a.ctx); listErr == nil {
		for _, list := range lists {
			discountListNameByID[list.ID] = list.Name
		}
	}
	accountByID := make(map[int64]db.CashboxAccount)
	if accounts, listErr := a.store.ListCashboxAccounts(a.ctx); listErr == nil {
		for _, account := range accounts {
			accountByID[account.ID] = account
		}
	}

	items := make([]map[string]any, 0, len(products))
	for _, line := range products {
		product, ok := productByID[line.ProductID]
		if !ok {
			continue
		}
		lineSalespersonName := ""
		if line.SalespersonID > 0 {
			lineSalespersonName = salespersonNameByID[line.SalespersonID]
		}
		priceListName := ""
		if line.PriceListID.Valid {
			priceListName = priceListNameByID[line.PriceListID.Int64]
		}
		discountListName := ""
		if line.DiscountListID.Valid {
			discountListName = discountListNameByID[line.DiscountListID.Int64]
		}
		items = append(items, map[string]any{
			"name": product.Name, "code": product.Code, "barcode": product.Barcode, "category": product.Description,
			"color": product.Color, "size": product.Size,
			"price": float64(line.UnitPrice) / 100, "lineTotal": float64(line.LineTotal) / 100,
			"discount": line.Discount, "qty": line.Quantity,
			"salespersonId": line.SalespersonID, "salespersonName": lineSalespersonName,
			"note":                 line.Note,
			"isPriceOverridden":    line.PriceListID.Valid,
			"priceListName":        priceListName,
			"isDiscountOverridden": line.DiscountListID.Valid,
			"discountListName":     discountListName,
		})
	}
	paymentList := make([]map[string]any, 0, len(payments))
	for _, payment := range payments {
		account := accountByID[payment.CashboxID]
		paymentList = append(paymentList, map[string]any{
			"accountId": payment.CashboxID, "accountName": account.Name,
			"currencyCode": account.CurrencyCode, "amount": payment.Amount,
		})
	}
	clientName := "Walk-in"
	clientPhone := ""
	invoiceIsWalkIn := true
	if client, clientErr := a.store.GetClientByID(a.ctx, inv.ClientID); clientErr == nil {
		clientName, clientPhone = client.Name, client.Phone
		invoiceIsWalkIn = isWalkInClient(client)
	}
	loyaltyPointsEarned := inv.GrandTotal / 100
	if invoiceIsWalkIn {
		loyaltyPointsEarned = 0
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
		"loyaltyPointsEarned": loyaltyPointsEarned,
		"createdAt":           inv.CreatedAt, "notes": inv.Notes, "items": items, "payments": paymentList,
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
		isWalkInClient(client) {
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
	centralURL, err := a.syncCentralAPIURL()
	if err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	err = a.store.ExecTx(a.ctx, func(q *db.Queries) error {
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
		if err := q.UpdateInvoiceCashbox(a.ctx, db.UpdateInvoiceCashboxParams{CashboxID: cashboxID, ID: invoiceID}); err != nil {
			return err
		}
		return queueSettlementChange(a.ctx, q, centralURL, invoiceID, inv.GrandTotal)
	})
	if err != nil {
		return err
	}
	if centralURL != "" {
		a.kickSyncWorker()
	}
	return nil
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

	centralURL, err := a.syncCentralAPIURL()
	if err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	err = a.store.ExecTx(a.ctx, func(q *db.Queries) error {
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
			if err := q.UpdateInvoiceCashbox(a.ctx, db.UpdateInvoiceCashboxParams{
				CashboxID: toCashboxID, ID: invoiceID,
			}); err != nil {
				return err
			}
		}
		return queueSettlementChange(a.ctx, q, centralURL, invoiceID, inv.GrandTotal)
	})
	if err != nil {
		return err
	}
	if centralURL != "" {
		a.kickSyncWorker()
	}
	return nil
}

// GetInvoiceForSettlementChange loads a sales invoice for "change settlement"
// mode: the full frozen cart (items/discounts/client/salesperson, exactly as
// GetInvoiceDetails returns it) plus its existing payments reshaped into
// Calculator entries, so the frontend can seed the calculator via
// restoreHoldState and let the cashier edit only the settlement.
func (a *App) GetInvoiceForSettlementChange(invoiceID int64) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	// GetSalesInvoiceByID only matches invoices flagged as an original sale
	// (joins sales_invoices) - this is what keeps settlement change from ever
	// being opened against a hold, void, or exchange/return invoice.
	if _, err := a.store.GetSalesInvoiceByID(a.ctx, invoiceID); err != nil {
		return nil, fmt.Errorf("sales invoice not found")
	}
	details, err := a.GetInvoiceDetails(invoiceID)
	if err != nil {
		return nil, err
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}
	roundingMode, err := normalizeRoundingMode(settings.RoundingMode)
	if err != nil {
		return nil, err
	}
	payments, err := a.store.ListInvoicePayments(a.ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	entries := make([]map[string]any, 0, len(payments))
	for _, payment := range payments {
		if payment.Amount == 0 {
			continue
		}
		account, accountErr := a.store.GetCashboxAccountByID(a.ctx, payment.CashboxID)
		if accountErr != nil {
			continue
		}
		currency, currErr := resolveCurrency(a.ctx, a.store.Queries, account.CurrencyCode)
		if currErr != nil {
			continue
		}
		negative := payment.Amount < 0
		nativeAmount := float64(absInt64(payment.Amount)) / 100
		if currency.CashRoundingUnit > 1 {
			if rounded, roundErr := roundCurrencyUnitsFromCents(absInt64(payment.Amount), currency.UnitsPerUsdMicros, currency.CashRoundingUnit, roundingMode); roundErr == nil {
				nativeAmount = float64(rounded)
			}
		}
		if negative {
			nativeAmount = -nativeAmount
		}
		entryType := "payment"
		if negative {
			entryType = "change"
		}
		entries = append(entries, map[string]any{
			"type": account.Name, "amount": nativeAmount, "amountCents": payment.Amount,
			"accountId": account.ID, "entryType": entryType,
		})
	}
	details["entries"] = entries
	return details, nil
}

// SettleInvoicePayments replaces an existing sales invoice's payments in
// place, without touching its items, discounts, client, or salesperson -
// there is no items input to this function at all, which is what
// structurally guarantees a "change settlement" checkout can never edit
// anything besides the settlement. The new payments are validated against
// the invoice's frozen GrandTotal (see QuoteSettlementPaymentStatus), the
// old cashbox balance deltas are fully reversed, then the new ones applied,
// mirroring ChangeInvoiceSettlementAmount's delete-and-recreate pattern but
// generalized to an arbitrary new payment set (including On Account/On
// Coupon, via recordSpecialPaymentMetadata).
func (a *App) SettleInvoicePayments(invoiceID int64, paymentsJSON string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	invoice, err := a.store.GetSalesInvoiceByID(a.ctx, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("sales invoice not found")
	}
	payments, err := parsePayments(paymentsJSON)
	if err != nil {
		return nil, err
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}
	roundingMode, err := normalizeRoundingMode(settings.RoundingMode)
	if err != nil {
		return nil, err
	}
	roundingCurrency, err := resolveCurrency(a.ctx, a.store.Queries, settings.RoundingCurrency)
	if err != nil {
		return nil, fmt.Errorf("rounding currency: %w", err)
	}

	err = a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		if _, err := normalizeCashRoundedPayments(a.ctx, q, payments, roundingMode); err != nil {
			return err
		}
		if err := validatePayments(a.ctx, q, payments, invoice.GrandTotal, settings.MaxOverpaymentCents, roundingCurrency, roundingMode); err != nil {
			return err
		}
		oldPayments, err := q.ListInvoicePayments(a.ctx, invoiceID)
		if err != nil {
			return err
		}
		for _, old := range oldPayments {
			if old.Amount == 0 {
				continue
			}
			if _, err := q.AddCashboxAccountBalance(a.ctx, db.AddCashboxAccountBalanceParams{
				AccountID: old.CashboxID, ShiftID: invoice.ShiftID, Balance: -old.Amount,
			}); err != nil {
				return fmt.Errorf("reverse prior settlement: %w", err)
			}
		}
		if err := q.DeleteInvoicePayments(a.ctx, invoiceID); err != nil {
			return err
		}
		if err := q.DeleteInvoicePaymentEntries(a.ctx, invoiceID); err != nil {
			return err
		}
		for _, payment := range payments {
			if err := q.CreateInvoicePaymentEntry(a.ctx, db.CreateInvoicePaymentEntryParams{
				InvoiceID: invoiceID, CashboxID: payment.AccountID, Amount: payment.AmountCents, EntryType: payment.EntryType,
			}); err != nil {
				return fmt.Errorf("record settlement entry: %w", err)
			}
		}
		if err := recordSpecialPaymentMetadata(a, q, invoiceID, invoice.ClientID, payments); err != nil {
			return err
		}
		netByAccount := netPaymentsByAccount(payments)
		var primaryAccountID, primaryAmount int64
		for accountID, amount := range netByAccount {
			if amount == 0 {
				continue
			}
			if err := q.CreateInvoicePayment(a.ctx, db.CreateInvoicePaymentParams{
				InvoiceID: invoiceID, CashboxID: accountID, Amount: amount,
			}); err != nil {
				return fmt.Errorf("record settlement: %w", err)
			}
			if _, err := q.AddCashboxAccountBalance(a.ctx, db.AddCashboxAccountBalanceParams{
				AccountID: accountID, ShiftID: invoice.ShiftID, Balance: amount,
			}); err != nil {
				return fmt.Errorf("add cashbox balance: %w", err)
			}
			if amount > primaryAmount {
				primaryAmount, primaryAccountID = amount, accountID
			}
		}
		if primaryAccountID != 0 && primaryAccountID != invoice.CashboxID {
			if err := q.UpdateInvoiceCashbox(a.ctx, db.UpdateInvoiceCashboxParams{
				CashboxID: primaryAccountID, ID: invoiceID,
			}); err != nil {
				return err
			}
		}
		return queueSettlementChange(a.ctx, q, settings.CentralApiUrl, invoiceID, invoice.GrandTotal)
	})
	if err != nil {
		return nil, err
	}
	if settings.CentralApiUrl != "" {
		a.kickSyncWorker()
	}
	return map[string]any{"invoiceCode": invoice.InvoiceCode}, nil
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
	roundingCurrency, err := resolveCurrency(a.ctx, a.store.Queries, settings.RoundingCurrency)
	if err != nil {
		return fmt.Errorf("rounding currency: %w", err)
	}
	// Use the already-persisted, already-rounded grand total as-is - never
	// re-round it against *current* settings, or a reprint after a
	// rounding-mode/rate/currency change would show a different total than
	// what was actually charged at checkout.
	grandTotal := details["grandTotal"].(int64)
	// Invoices don't persist the tax they were charged, so a reprint has to
	// re-derive it from the *current* rate. Unlike the grand total above
	// that's unavoidable; it only misstates the tax line on a reprint taken
	// after the rate changed, and the total the customer paid is still the
	// persisted one.
	taxCents, err := roundFloatToCents(float64(details["discountedTotal"].(int64))/100*settings.TaxRate, settings.RoundingMode)
	if err != nil {
		return fmt.Errorf("receipt tax: %w", err)
	}
	marketName := settings.MarketName
	if marketName == "" {
		marketName = "Market"
	}
	return a.printReceipt(items, marketName, settings.BranchName, settings.MarketPhone, settings.BranchPhone, settings.MarketDescription, settings.CounterNumber, details["salespersonName"].(string), details["clientName"].(string), details["notes"].(string), details["invoiceCode"].(string), settings.ReceiptPrinter, grandTotal, taxCents, payments, roundingCurrency, settings.RoundingMode, settings.ReceiptWidth, settings.ReceiptHeight, settings.ReturnPolicy, settings.Website, settings.Instagram, decodeSocialPlatforms(settings.SocialPlatforms), decodeSocialHandles(settings.SocialHandles), settings.ReceiptFont, settings.ReceiptBodyFont, settings.ReceiptTitleSize, settings.ReceiptBodySize, settings.ReceiptCutoff != 0)
}

func (a *App) printReceipt(items []invoiceItem, storeName, branch, phone, branchPhone, description, registerName, salesperson, customer, notes, receiptNo, printerName string, grandTotalCents, taxCents int64, payments []checkoutPayment, roundingCurrency db.Currency, roundingMode string, receiptWidth, receiptHeight int64, returnPolicy, website, instagram string, socialPlatforms []string, socialHandles map[string]string, font, bodyFont string, titleSize, bodySize float64, cutoff bool) error {
	if printerName == "" {
		return fmt.Errorf("no receipt printer configured")
	}
	notes = strings.TrimSpace(notes)

	currencyCache := make(map[string]receiptCurrency)
	receiptPayments := make([]receiptPayment, 0, len(payments))
	for _, payment := range payments {
		account, err := a.store.GetCashboxAccountByID(a.ctx, payment.AccountID)
		if err != nil {
			return fmt.Errorf("receipt payment account: %w", err)
		}
		paymentCurrency, cached := currencyCache[account.CurrencyCode]
		if !cached {
			if resolved, currErr := resolveCurrency(a.ctx, a.store.Queries, account.CurrencyCode); currErr == nil {
				paymentCurrency = toReceiptCurrency(resolved)
			}
			currencyCache[account.CurrencyCode] = paymentCurrency
		}
		receiptPayments = append(receiptPayments, receiptPayment{
			AccountID:    account.ID,
			AccountName:  account.Name,
			CurrencyCode: account.CurrencyCode,
			Currency:     paymentCurrency,
			AmountCents:  payment.AmountCents,
			EntryType:    payment.EntryType,
		})
	}
	receiptPayments = aggregateReceiptPayments(receiptPayments)
	return printReceiptToPrinter(printReceiptData{
		Items:            items,
		StoreName:        storeName,
		Branch:           branch,
		Phone:            phone,
		BranchPhone:      branchPhone,
		Description:      description,
		RegisterName:     strings.TrimSpace(registerName),
		Salesperson:      salesperson,
		Customer:         customer,
		Notes:            notes,
		ReturnPolicy:     returnPolicy,
		Website:          website,
		Instagram:        instagram,
		SocialPlatforms:  socialPlatforms,
		SocialHandles:    socialHandles,
		Font:             font,
		BodyFont:         bodyFont,
		TitleSize:        titleSize,
		BodySize:         bodySize,
		Cutoff:           cutoff,
		ReceiptNo:        receiptNo,
		PrinterName:      printerName,
		GrandTotalCents:  grandTotalCents,
		TaxCents:         taxCents,
		Payments:         receiptPayments,
		RoundingCurrency: toReceiptCurrency(roundingCurrency),
		RoundingMode:     roundingMode,
		ReceiptWidth:     receiptWidth,
		ReceiptHeight:    receiptHeight,
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

func formatReceiptPayment(payment receiptPayment, roundingMode string) string {
	amount := payment.AmountCents
	if amount < 0 {
		amount = -amount
	}
	// A whole-major-unit rounding unit greater than 1 identifies a physical
	// cash currency that should be displayed as a rounded integer amount
	// (e.g. LBP); anything else (the default accounting currency, cards)
	// displays as a decimal dollar amount.
	if payment.Currency.CashRoundingUnit > 1 {
		if rounded, err := roundCurrencyUnitsFromCents(amount, payment.Currency.UnitsPerUsdMicros, payment.Currency.CashRoundingUnit, roundingMode); err == nil {
			return fmt.Sprintf("%s %s", formatInt(rounded), payment.Currency.Symbol)
		}
		return "0 " + payment.Currency.Symbol
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
