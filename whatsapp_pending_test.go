package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

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

	app.whatsAppThankYou("SA/CT/2026/00482", 15625, db.Client{Name: "Sara", Phone: "+96170123456"}, nil)
	pending, err := store.ListWhatsAppPending(ctx)
	if err != nil || len(pending) != 1 || pending[0].Attempts != 1 {
		t.Fatalf("after a failed send: %+v, %v", pending, err)
	}

	// Too soon: its first try may still be running.
	app.retryWhatsAppPending()
	if calls.Load() != 1 {
		t.Fatalf("retried too soon: %d calls", calls.Load())
	}

	if _, err := app.conn.ExecContext(ctx, `UPDATE whatsapp_pending SET created_at = datetime('now', '-2 minutes')`); err != nil {
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
