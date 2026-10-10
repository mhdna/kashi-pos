package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

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
// us" WhatsApp message to the invoice's client. Everything is best-effort
// and off the checkout path: no central sync configured, a walk-in/
// anonymous client, a client with no phone, or kashi being unreachable all
// just mean no message. The caller invokes it in its own goroutine; nothing
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := central.PostWhatsAppThankYou(ctx, payload)
	if err != nil {
		log.Printf("whatsapp thank-you: kashi request failed for invoice %s: %v", invoiceCode, err)
		return
	}
	if !resp.Sent {
		log.Printf("whatsapp thank-you: kashi did not send for invoice %s: %s", invoiceCode, resp.Reason)
		return
	}
	log.Printf("whatsapp thank-you: sent for invoice %s", invoiceCode)
}

func whatsAppItems(items []invoiceItem) []whatsAppThankYouItem {
	out := make([]whatsAppThankYouItem, len(items))
	for i, it := range items {
		out[i] = whatsAppThankYouItem{Code: it.Code, Name: it.Name, Category: it.Category, Qty: it.Qty}
	}
	return out
}
