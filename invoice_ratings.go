package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// How clients rated this branch's sales: kashi's WhatsApp thank-you message
// is followed by a poll (😍 Loved it, 🙂 It was okay, 😕 Not great), and the
// invoices list shows each answer beside its sale. The answers live only in
// kashi, so the list asks for the codes it has on screen (like the expense
// photos); offline, unsynced, or a kashi too old to know shows nothing.

// Shows the ratings column on the invoices list. Defaults to on when unset:
// a till that never touched it shows the ratings. kashi can push it as
// invoice_ratings_enabled.
const invoiceRatingsFlag = "invoice_ratings"

const (
	invoiceRatingsTimeout  = 8 * time.Second
	invoiceRatingsMaxCodes = 200
)

// InvoiceRating is one client's answer: 1-5 stars (the poll's options spread
// over the scale: 5 loved it, 3 okay, 1 not great by default) and the option
// as they saw it.
type InvoiceRating struct {
	Rating int    `json:"rating"`
	Answer string `json:"answer"`
}

// SetInvoiceRatingsEnabled shows or hides the ratings on the invoices list.
func (a *App) SetInvoiceRatingsEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := a.setFeatureFlagBool(a.ctx, invoiceRatingsFlag, enabled); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

// GetInvoiceRatings is kashi's poll answers for these invoice codes, keyed by
// code; sales nobody rated are left out. Never an error: anything that goes
// wrong is just no ratings.
func (a *App) GetInvoiceRatings(codes []string) map[string]InvoiceRating {
	out := map[string]InvoiceRating{}
	if a.store == nil || !a.flagEnabledDefaultOn(a.ctx, invoiceRatingsFlag) {
		return out
	}
	clean := make([]string, 0, len(codes))
	for _, code := range codes {
		if code = strings.TrimSpace(code); code != "" && len(clean) < invoiceRatingsMaxCodes {
			clean = append(clean, code)
		}
	}
	client := a.photoClient()
	if client == nil || len(clean) == 0 {
		return out
	}
	ctx, cancel := context.WithTimeout(context.Background(), invoiceRatingsTimeout)
	defer cancel()
	ratings, err := client.fetchInvoiceRatings(ctx, clean)
	if err != nil {
		return out
	}
	return ratings
}

func (c *centralClient) fetchInvoiceRatings(ctx context.Context, codes []string) (map[string]InvoiceRating, error) {
	body, err := json.Marshal(map[string]any{"codes": codes})
	if err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/branch/invoice_ratings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, responseError(http.MethodPost, "/branch/invoice_ratings", resp)
	}
	var out struct {
		Ratings map[string]InvoiceRating `json:"ratings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Ratings == nil {
		out.Ratings = map[string]InvoiceRating{}
	}
	return out.Ratings, nil
}
