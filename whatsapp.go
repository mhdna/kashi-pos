package main

import (
	"context"
	"encoding/json"
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
	ClientName      string                 `json:"client_name"`
	ClientPhone     string                 `json:"client_phone"`
	InvoiceCode     string                 `json:"invoice_code"`
	GrandTotalCents int64                  `json:"grand_total_cents"`
	Items           []whatsAppThankYouItem `json:"items"`
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
func (a *App) whatsAppThankYou(invoiceCode string, grandTotalCents int64, client db.Client, items []invoiceItem) {
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

	payload, err := json.Marshal(whatsAppThankYouPayload{
		ClientName:      client.Name,
		ClientPhone:     client.Phone,
		InvoiceCode:     invoiceCode,
		GrandTotalCents: grandTotalCents,
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
	a.postWhatsAppPending(central, invoiceCode, payload)
}

// Pending thank-yous are tried again every minute, after the first try has
// had its time (whatsAppRetryAfter), for as long as a thank-you still makes
// sense (whatsAppPendingMaxAge). kashi sends at most one per invoice, so a
// retry of one that did go through just comes back "already sent".
const (
	whatsAppRetryInterval   = time.Minute
	whatsAppRetryAfter      = time.Minute
	whatsAppPendingMaxAge   = 6 * time.Hour
	whatsAppPendingMaxTries = 30
)

// postWhatsAppPending asks kashi to send one thank-you. Any answer from kashi
// settles it (sent, or not sent for a reason of its own, like a rule); only a
// request that didn't get an answer stays pending.
func (a *App) postWhatsAppPending(central *centralClient, invoiceCode string, payload []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := central.PostWhatsAppThankYou(ctx, payload)
	if err != nil {
		log.Printf("whatsapp thank-you: kashi request failed for invoice %s (will retry): %v", invoiceCode, err)
		if err := a.store.BumpWhatsAppPending(context.Background(), invoiceCode); err != nil {
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
		if age < whatsAppRetryAfter {
			continue // its first try may still be running
		}
		a.postWhatsAppPending(central, p.InvoiceCode, []byte(p.Payload))
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
