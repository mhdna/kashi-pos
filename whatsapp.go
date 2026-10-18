package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	db "kashi-pos/db/sqlc"
)

// whatsAppThankYouPayload is the body POSTed to kashi's
// /branch/whatsapp/thank_you. All WhatsApp/Meta config (Cloud API
// credentials, template, etc.) now lives centrally on kashi -- see
// api/whatsapp.go in that repo -- kashi-pos just reports the sale. Items let
// kashi's WhatsApp rules match on what was sold (a code or a category).
type whatsAppThankYouPayload struct {
	ClientName      string `json:"client_name"`
	ClientPhone     string `json:"client_phone"`
	InvoiceCode     string `json:"invoice_code"`
	GrandTotalCents int64  `json:"grand_total_cents"`
	// PaidCents is what the client actually paid now: the total minus any
	// part put on account. kashi shows it as "Total paid", and leaves the
	// line out when it's 0. Omitted by older tills; kashi then uses the total.
	PaidCents *int64                 `json:"paid_cents,omitempty"`
	Items     []whatsAppThankYouItem `json:"items"`
}

type whatsAppThankYouItem struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Qty      float64 `json:"qty"`
}

// whatsAppThankYou asks kashi to fire a post-sale "thank you / please rate
// us" WhatsApp message to the invoice's client. It's off the checkout path:
// no central sync configured, a walk-in/anonymous client or a client with no
// phone just mean no message, and kashi being unreachable means it's sent
// later (retryWhatsAppPending). The caller invokes it in its own goroutine; nothing
// here blocks the sale.
func (a *App) whatsAppThankYou(invoiceCode string, grandTotalCents int64, client db.Client, items []invoiceItem, payments []checkoutPayment) {
	if a == nil || a.store == nil {
		return
	}
	if isWalkInClient(client) || strings.TrimSpace(client.Phone) == "" {
		return
	}

	settings, err := a.store.GetSettings(context.Background())
	if err != nil {
		log.Printf("whatsapp thank-you: load settings: %v", err)
		return
	}
	central := newCentralClient(settings.CentralApiUrl, settings.BranchCode, settings.ApiKey)
	if !central.enabled() {
		return
	}

	paid := grandTotalCents - a.onAccountCents(payments)
	if paid < 0 {
		paid = 0
	}
	payload, err := json.Marshal(whatsAppThankYouPayload{
		ClientName:      client.Name,
		ClientPhone:     client.Phone,
		InvoiceCode:     invoiceCode,
		GrandTotalCents: grandTotalCents,
		PaidCents:       &paid,
		Items:           whatsAppItems(items),
	})
	if err != nil {
		log.Printf("whatsapp thank-you: encode payload: %v", err)
		return
	}

	// Saved first, so a request cut off (the app closing for an update, no
	// internet) is sent again by retryWhatsAppPending.
	if err := a.store.SaveWhatsAppPending(context.Background(), db.SaveWhatsAppPendingParams{InvoiceCode: invoiceCode, Payload: string(payload)}); err != nil {
		log.Printf("whatsapp thank-you: save pending %s: %v", invoiceCode, err)
	}
	a.postWhatsAppPending(central, invoiceCode, payload, 0)
}

// Pending thank-yous are checked every minute. A failed try waits 1, 2, 4...
// minutes (at most whatsAppRetryMaxDelay) before the next, for as long as a
// thank-you still makes sense (whatsAppPendingMaxAge). kashi sends at most
// one per invoice, so a retry of one that did go through just comes back
// "already sent".
const (
	whatsAppRetryInterval   = time.Minute
	whatsAppRetryAfter      = time.Minute
	whatsAppRetryMaxDelay   = 30 * time.Minute
	whatsAppPendingMaxAge   = 6 * time.Hour
	whatsAppPendingMaxTries = 30
)

// whatsAppRetryDelay is how long to wait after the attempts-th failed try.
func whatsAppRetryDelay(attempts int64) time.Duration {
	delay := whatsAppRetryAfter
	for i := int64(0); i < attempts && delay < whatsAppRetryMaxDelay; i++ {
		delay *= 2
	}
	return min(delay, whatsAppRetryMaxDelay)
}

// onAccountCents is the part of a sale put on the client's account.
func (a *App) onAccountCents(payments []checkoutPayment) int64 {
	var total int64
	for _, p := range payments {
		account, err := a.store.GetCashboxAccountByID(context.Background(), p.AccountID)
		if err == nil && isNonCollectedIncomeAccount(account.Name) {
			total += p.AmountCents
		}
	}
	return total
}

// postWhatsAppPending asks kashi to send one thank-you. kashi's answer
// settles it (sent, or not sent for a reason of its own, like a rule), unless
// kashi says to retry (its bridge or Meta was unreachable); a request that
// got no answer stays pending too.
func (a *App) postWhatsAppPending(central *centralClient, invoiceCode string, payload []byte, attempts int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := central.PostWhatsAppThankYou(ctx, payload)
	if err == nil && !resp.Sent && resp.Retry {
		err = fmt.Errorf("kashi couldn't send for now: %s", resp.Reason)
	}
	if err != nil {
		log.Printf("whatsapp thank-you: not sent for invoice %s (will retry): %v", invoiceCode, err)
		delay := fmt.Sprintf("+%d minutes", int(whatsAppRetryDelay(attempts).Minutes()))
		if err := a.store.BumpWhatsAppPending(context.Background(), db.BumpWhatsAppPendingParams{Delay: delay, InvoiceCode: invoiceCode}); err != nil {
			log.Printf("whatsapp thank-you: bump pending %s: %v", invoiceCode, err)
		}
		return
	}
	if err := a.store.DeleteWhatsAppPending(context.Background(), invoiceCode); err != nil {
		log.Printf("whatsapp thank-you: clear pending %s: %v", invoiceCode, err)
	}
	if !resp.Sent {
		log.Printf("whatsapp thank-you: kashi did not send for invoice %s: %s", invoiceCode, resp.Reason)
		return
	}
	log.Printf("whatsapp thank-you: sent for invoice %s", invoiceCode)
	var sale whatsAppThankYouPayload
	_ = json.Unmarshal(payload, &sale)
	select {
	case <-a.ready:
		runtime.EventsEmit(a.ctx, "whatsapp_sent", map[string]string{"client_name": sale.ClientName, "invoice_code": invoiceCode})
	default:
	}
}

// retryWhatsAppPending sends the thank-yous still waiting for kashi, oldest
// first, dropping ones too old or tried too often.
func (a *App) retryWhatsAppPending() {
	ctx := context.Background()
	pending, err := a.store.ListWhatsAppPending(ctx)
	if err != nil || len(pending) == 0 {
		return
	}
	settings, err := a.store.GetSettings(ctx)
	if err != nil {
		return
	}
	central := newCentralClient(settings.CentralApiUrl, settings.BranchCode, settings.ApiKey)
	for _, p := range pending {
		age := time.Since(p.CreatedAt)
		if age > whatsAppPendingMaxAge || p.Attempts >= whatsAppPendingMaxTries || !central.enabled() {
			log.Printf("whatsapp thank-you: giving up on invoice %s after %d tries", p.InvoiceCode, p.Attempts)
			_ = a.store.DeleteWhatsAppPending(ctx, p.InvoiceCode)
			continue
		}
		if p.NextAttemptAt.Valid {
			if time.Now().Before(p.NextAttemptAt.Time) {
				continue
			}
		} else if age < whatsAppRetryAfter {
			continue // its first try may still be running
		}
		a.postWhatsAppPending(central, p.InvoiceCode, []byte(p.Payload), p.Attempts)
	}
}

// startWhatsAppRetry runs retryWhatsAppPending now (thank-yous cut off when
// the app last closed) and every minute until shutdown.
func (a *App) startWhatsAppRetry() {
	if a.store == nil || a.syncWorkerStop == nil {
		return
	}
	a.backgroundWG.Add(1)
	go func() {
		defer a.backgroundWG.Done()
		a.retryWhatsAppPending()
		ticker := time.NewTicker(whatsAppRetryInterval)
		defer ticker.Stop()
		for {
			select {
			case <-a.syncWorkerStop:
				return
			case <-ticker.C:
				a.retryWhatsAppPending()
			}
		}
	}()
}

func whatsAppItems(items []invoiceItem) []whatsAppThankYouItem {
	out := make([]whatsAppThankYouItem, len(items))
	for i, it := range items {
		out[i] = whatsAppThankYouItem{Code: it.Code, Name: it.Name, Category: it.Category, Qty: it.Qty}
	}
	return out
}
