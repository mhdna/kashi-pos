package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

// whatsAppThankYou fires a post-sale "thank you / please rate us" WhatsApp
// message to the invoice's client via Meta's WhatsApp Business Platform
// (Cloud API):
//
//	POST https://graph.facebook.com/v22.0/{phone_number_id}/messages
//	Authorization: Bearer {access_token}
//	body: {"messaging_product": "whatsapp", "to": "<digits>", "type": "template",
//	       "template": {"name": "...", "language": {"code": "..."},
//	                    "components": [{"type": "body", "parameters": [...]}]}}
//
// Cloud API only allows business-initiated messages through a pre-approved
// template, so the outgoing message is built from whatsapp_template_name /
// whatsapp_template_lang plus an ordered whatsapp_template_params list
// (comma-separated placeholder names: name, market, branch, invoice, total)
// that fill the template's {{1}}, {{2}}, ... body parameters in order.
//
// Everything is best-effort and off the checkout path: a bad config, an
// unreachable API, or a walk-in/anonymous client just means no message. The
// caller invokes it in its own goroutine; nothing here blocks the sale.
func (a *App) whatsAppThankYou(invoiceCode string, grandTotalCents int64, client db.Client, items []invoiceItem) {
	if a == nil || a.store == nil {
		return
	}
	settings, err := a.store.GetSettings(context.Background())
	if err != nil {
		log.Printf("whatsapp thank-you: load settings: %v", err)
		return
	}
	if settings.WhatsappEnabled == 0 {
		return
	}
	if isWalkInClient(client) || strings.TrimSpace(client.Phone) == "" {
		return
	}

	phoneNumberID := strings.TrimSpace(settings.WhatsappPhoneNumberID)
	token := strings.TrimSpace(settings.WhatsappAccessToken)
	templateName := strings.TrimSpace(settings.WhatsappTemplateName)
	if phoneNumberID == "" || token == "" || templateName == "" {
		log.Printf("whatsapp thank-you: enabled but phone_number_id/access_token/template_name are incomplete")
		return
	}

	to := whatsAppDigits(client.Phone)
	if to == "" {
		log.Printf("whatsapp thank-you: client %d has an unusable phone %q", client.ID, client.Phone)
		return
	}

	templateLang := strings.TrimSpace(settings.WhatsappTemplateLang)
	if templateLang == "" {
		templateLang = "en_US"
	}

	params := whatsAppTemplateParams(settings, invoiceCode, grandTotalCents, client)

	endpoint := fmt.Sprintf("https://graph.facebook.com/v22.0/%s/messages", phoneNumberID)
	body := map[string]any{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "template",
		"template": map[string]any{
			"name":     templateName,
			"language": map[string]any{"code": templateLang},
			"components": []map[string]any{
				{"type": "body", "parameters": params},
			},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		log.Printf("whatsapp thank-you: encode payload: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		log.Printf("whatsapp thank-you: build request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("whatsapp thank-you: POST %s: %v", endpoint, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("whatsapp thank-you: %s -> HTTP %d", endpoint, resp.StatusCode)
		return
	}
	log.Printf("whatsapp thank-you: sent for invoice %s to %s", invoiceCode, to)
}

// whatsAppTemplateParams builds the template's body parameters in the order
// given by settings.WhatsappTemplateParams (comma-separated placeholder
// names). Unknown placeholder names are skipped.
func whatsAppTemplateParams(settings db.Setting, invoiceCode string, grandTotalCents int64, client db.Client) []map[string]any {
	market := strings.TrimSpace(settings.MarketName)
	if market == "" {
		market = "us"
	}
	name := strings.TrimSpace(client.Name)
	if name == "" {
		name = "there"
	}
	values := map[string]string{
		"name":    name,
		"market":  market,
		"branch":  strings.TrimSpace(settings.BranchName),
		"invoice": invoiceCode,
		"total":   fmt.Sprintf("$%.2f", float64(grandTotalCents)/100),
	}

	var params []map[string]any
	for _, key := range strings.Split(settings.WhatsappTemplateParams, ",") {
		key = strings.TrimSpace(key)
		v, ok := values[key]
		if !ok {
			continue
		}
		params = append(params, map[string]any{"type": "text", "text": v})
	}
	return params
}

// whatsAppDigits turns an E.164 phone ("+96170123456") into the digits-only
// form the Cloud API expects ("96170123456"). Returns "" when nothing
// dial-able is left.
func whatsAppDigits(phone string) string {
	var b strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if len(digits) < 8 {
		return ""
	}
	return digits
}
