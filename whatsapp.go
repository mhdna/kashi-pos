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
// message to the invoice's client. It talks to an open-wa (@open-wa/wa-automate)
// EASY API server:
//
//	POST {api_url}/{session}/sendText
//	header: api_key: {api_key}
//	body:   {"args": {"to": "<digits>@c.us", "content": "<text>"}}
//
// Everything is best-effort and off the checkout path: a bad config, an
// unreachable API, or a walk-in/anonymous client just means no message. The
// caller invokes it in its own goroutine; nothing here blocks the sale.
func (a *App) whatsAppThankYou(invoiceCode string, grandTotalCents int64, client db.Client) {
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
	apiURL := strings.TrimRight(strings.TrimSpace(settings.WhatsappApiUrl), "/")
	if apiURL == "" {
		log.Printf("whatsapp thank-you: enabled but whatsapp_api_url is blank")
		return
	}

	chatID := whatsAppChatID(client.Phone)
	if chatID == "" {
		log.Printf("whatsapp thank-you: client %d has an unusable phone %q", client.ID, client.Phone)
		return
	}

	message := renderWhatsAppMessage(settings, invoiceCode, grandTotalCents, client)
	if strings.TrimSpace(message) == "" {
		return
	}

	session := strings.TrimSpace(settings.WhatsappSession)
	if session == "" {
		session = "session"
	}
	endpoint := fmt.Sprintf("%s/%s/sendText", apiURL, session)

	payload, err := json.Marshal(map[string]any{
		"args": map[string]any{
			"to":      chatID,
			"content": message,
		},
	})
	if err != nil {
		log.Printf("whatsapp thank-you: encode payload: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		log.Printf("whatsapp thank-you: build request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if key := strings.TrimSpace(settings.WhatsappApiKey); key != "" {
		req.Header.Set("api_key", key)
	}

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
	log.Printf("whatsapp thank-you: sent for invoice %s to %s", invoiceCode, chatID)
}

// whatsAppChatID turns an E.164 phone ("+96170123456") into an open-wa chat id
// ("96170123456@c.us"). Returns "" when nothing dial-able is left.
func whatsAppChatID(phone string) string {
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
	return digits + "@c.us"
}

// renderWhatsAppMessage substitutes the template placeholders. A blank review
// URL drops "{link}" (and a trailing "space + {link}") so the sentence still
// reads cleanly.
func renderWhatsAppMessage(settings db.Setting, invoiceCode string, grandTotalCents int64, client db.Client) string {
	tmpl := strings.TrimSpace(settings.WhatsappMessage)
	if tmpl == "" {
		return ""
	}
	market := strings.TrimSpace(settings.MarketName)
	if market == "" {
		market = "us"
	}
	name := strings.TrimSpace(client.Name)
	if name == "" {
		name = "there"
	}
	link := strings.TrimSpace(settings.WhatsappReviewUrl)

	out := tmpl
	if link == "" {
		out = strings.ReplaceAll(out, " {link}", "")
	}
	replacer := strings.NewReplacer(
		"{name}", name,
		"{market}", market,
		"{branch}", strings.TrimSpace(settings.BranchName),
		"{invoice}", invoiceCode,
		"{total}", fmt.Sprintf("$%.2f", float64(grandTotalCents)/100),
		"{link}", link,
	)
	return strings.TrimSpace(replacer.Replace(out))
}
