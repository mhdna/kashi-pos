package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"

	qrcode "github.com/skip2/go-qrcode"
)

// whatsAppThankYou fires a post-sale "thank you / please rate us" WhatsApp
// message to the invoice's client. It talks to an open-wa
// (@open-wa/wa-automate) EASY API server.
//
// When a QR of the invoice number can be rendered, the message goes out as an
// image (open-wa /sendImage) with the item list + copy as the caption:
//
//	POST {api_url}/{session}/sendImage
//	body: {"args": {"to": "<digits>@c.us",
//	                "base64": "data:image/png;base64,<png>",
//	                "filename": "<invoice>.png",
//	                "caption": "<text>"}}
//
// Otherwise it falls back to a plain /sendText with the same text.
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

	message := renderWhatsAppMessage(settings, invoiceCode, grandTotalCents, client, items)
	if strings.TrimSpace(message) == "" {
		return
	}

	session := strings.TrimSpace(settings.WhatsappSession)
	if session == "" {
		session = "session"
	}

	var (
		endpoint string
		args     map[string]any
	)
	if png, qrErr := qrcode.Encode(invoiceCode, qrcode.Medium, 512); qrErr == nil {
		endpoint = fmt.Sprintf("%s/%s/sendImage", apiURL, session)
		args = map[string]any{
			"to":       chatID,
			"base64":   "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
			"filename": sanitizeFilename(invoiceCode) + ".png",
			"caption":  message,
		}
	} else {
		log.Printf("whatsapp thank-you: QR encode for %q failed (%v), sending text only", invoiceCode, qrErr)
		endpoint = fmt.Sprintf("%s/%s/sendText", apiURL, session)
		args = map[string]any{"to": chatID, "content": message}
	}

	payload, err := json.Marshal(map[string]any{"args": args})
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

// sanitizeFilename keeps the invoice code usable as a file name.
func sanitizeFilename(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9', r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, s)
	if s == "" {
		return "invoice"
	}
	return s
}

// whatsAppItemLines renders the purchased products as bullet lines, e.g.
//
//   - 2 x Blue Shirt - $30.00
func whatsAppItemLines(items []invoiceItem) string {
	var b strings.Builder
	for _, it := range items {
		name := strings.TrimSpace(it.Name)
		if name == "" {
			name = strings.TrimSpace(it.Code)
		}
		if name == "" {
			continue
		}
		lineTotal := it.Qty * it.Price * (1 - it.Discount/100)
		fmt.Fprintf(&b, "• %s x %s - $%.2f\n", trimQty(it.Qty), name, lineTotal)
	}
	return strings.TrimRight(b.String(), "\n")
}

// trimQty prints 2 rather than 2.00 for whole quantities.
func trimQty(q float64) string {
	if q == float64(int64(q)) {
		return fmt.Sprintf("%d", int64(q))
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", q), "0"), ".")
}

// renderWhatsAppMessage substitutes the template placeholders:
//
//	{name} {market} {branch} {invoice} {total} {items} {link}
//
// A blank review URL drops "{link}" (and a preceding space/newline) so the
// text still reads cleanly; likewise "{items}" collapses when there are none.
func renderWhatsAppMessage(settings db.Setting, invoiceCode string, grandTotalCents int64, client db.Client, items []invoiceItem) string {
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
	itemLines := whatsAppItemLines(items)

	out := tmpl
	if link == "" {
		out = collapsePlaceholder(out, "{link}")
	}
	if itemLines == "" {
		out = collapsePlaceholder(out, "{items}")
	}
	replacer := strings.NewReplacer(
		"{name}", name,
		"{market}", market,
		"{branch}", strings.TrimSpace(settings.BranchName),
		"{invoice}", invoiceCode,
		"{total}", fmt.Sprintf("$%.2f", float64(grandTotalCents)/100),
		"{items}", itemLines,
		"{link}", link,
	)
	return strings.TrimSpace(replacer.Replace(out))
}

// collapsePlaceholder removes an empty placeholder along with one leading
// space or newline, so "total {total}\n{link}" doesn't leave a dangling line.
func collapsePlaceholder(s, ph string) string {
	s = strings.ReplaceAll(s, "\n"+ph, "")
	s = strings.ReplaceAll(s, " "+ph, "")
	return strings.ReplaceAll(s, ph, "")
}
