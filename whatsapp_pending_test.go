package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

// A thank-you kashi didn't answer stays saved and is sent again later; once
// kashi answers it's gone.
func TestWhatsAppThankYouRetriedUntilKashiAnswers(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/branch/whatsapp/thank_you" {
			http.NotFound(w, r)
			return
		}
		if calls.Add(1) == 1 {
			http.Error(w, "down for an update", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"sent":true}`))
	}))
	defer server.Close()
	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	ctx := context.Background()

	app.whatsAppThankYou("SA/CT/2026/00482", 15625, db.Client{Name: "Sara", Phone: "+96170123456"}, nil, nil)
	pending, err := store.ListWhatsAppPending(ctx)
	if err != nil || len(pending) != 1 || pending[0].Attempts != 1 {
		t.Fatalf("after a failed send: %+v, %v", pending, err)
	}

	// Too soon: the next try waits a minute.
	app.retryWhatsAppPending()
	if calls.Load() != 1 {
		t.Fatalf("retried too soon: %d calls", calls.Load())
	}

	if _, err := app.conn.ExecContext(ctx, `UPDATE whatsapp_pending SET next_attempt_at = datetime('now', '-1 minute')`); err != nil {
		t.Fatal(err)
	}
	app.retryWhatsAppPending()
	if calls.Load() != 2 {
		t.Fatalf("not retried: %d calls", calls.Load())
	}
	if pending, _ := store.ListWhatsAppPending(ctx); len(pending) != 0 {
		t.Fatalf("still pending after kashi answered: %+v", pending)
	}
}

// One too old to still make sense is dropped without sending.
func TestWhatsAppPendingTooOldIsDropped(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	ctx := context.Background()
	if err := store.SaveWhatsAppPending(ctx, db.SaveWhatsAppPendingParams{InvoiceCode: "SA/CT/2026/00001", Payload: "{}"}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.conn.ExecContext(ctx, `UPDATE whatsapp_pending SET created_at = datetime('now', '-7 hours')`); err != nil {
		t.Fatal(err)
	}
	app.retryWhatsAppPending()
	if calls.Load() != 0 {
		t.Fatalf("sent a stale thank-you")
	}
	if pending, _ := store.ListWhatsAppPending(ctx); len(pending) != 0 {
		t.Fatalf("stale one kept: %+v", pending)
	}
}

// kashi answering "not sent, retry" (its bridge was down) keeps the
// thank-you pending, with the next try further off each time.
func TestWhatsAppThankYouKeptWhenKashiSaysRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"sent":false,"reason":"WhatsApp bridge unreachable","retry":true}`))
	}))
	defer server.Close()
	app, store, _, _ := setUpTestInvoiceApp(t, server.URL)
	ctx := context.Background()

	app.whatsAppThankYou("SA/CT/2026/00483", 5000, db.Client{Name: "Sara", Phone: "+96170123456"}, nil, nil)
	pending, err := store.ListWhatsAppPending(ctx)
	if err != nil || len(pending) != 1 || pending[0].Attempts != 1 || !pending[0].NextAttemptAt.Valid {
		t.Fatalf("after a retry answer: %+v, %v", pending, err)
	}
	if got := whatsAppRetryDelay(0); got != time.Minute {
		t.Fatalf("first delay = %v, want 1m", got)
	}
	if got := whatsAppRetryDelay(3); got != 8*time.Minute {
		t.Fatalf("delay after 3 tries = %v, want 8m", got)
	}
	if got := whatsAppRetryDelay(20); got != whatsAppRetryMaxDelay {
		t.Fatalf("delay after 20 tries = %v, want the cap", got)
	}
}

// A sale put on account tells kashi the client paid nothing now.
func TestWhatsAppThankYouSendsPaidAmountWithoutOnAccount(t *testing.T) {
	var got whatsAppThankYouPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"sent":true}`))
	}))
	defer server.Close()
	app, store, cashbox, _ := setUpTestInvoiceApp(t, server.URL)
	ctx := context.Background()
	onAccount, err := store.CreateCashboxAccount(ctx, db.CreateCashboxAccountParams{Name: "On Account", CurrencyCode: "USD"})
	if err != nil {
		t.Fatal(err)
	}

	app.whatsAppThankYou("SA/CT/2026/00484", 8000, db.Client{Name: "Sara", Phone: "+96170123456"}, nil, []checkoutPayment{
		{AccountID: cashbox.ID, AmountCents: 5000},
		{AccountID: onAccount.ID, AmountCents: 3000},
	})
	if got.PaidCents == nil || *got.PaidCents != 5000 || got.GrandTotalCents != 8000 {
		t.Fatalf("payload = total %d, paid %v; want 8000 and 5000", got.GrandTotalCents, got.PaidCents)
	}
}
