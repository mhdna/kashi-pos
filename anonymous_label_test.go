package main

import (
	"encoding/json"
	"testing"

	db "kashi-pos/db/sqlc"
)

// The till's screens call a sale with no client "Anonymous"; only the printed
// receipt says "Walk-in". The seeded walk-in pseudo-client is stored as
// "Walk-in", so every screen path has to translate it.

func TestClientDisplayNameAndReceiptCustomerName(t *testing.T) {
	cases := []struct {
		name        string
		client      db.Client
		found       bool
		wantScreen  string
		wantReceipt string
	}{
		{"seeded walk-in client", db.Client{Name: "Walk-in", Phone: walkInClientPhone}, true, "Anonymous", "Walk-in"},
		{"walk-in phone, renamed", db.Client{Name: "Someone", Phone: walkInClientPhone}, true, "Anonymous", "Walk-in"},
		{"blank name", db.Client{Name: "  ", Phone: "70123456"}, true, "Anonymous", "Walk-in"},
		{"real client", db.Client{Name: "Rana", Phone: "70123456"}, true, "Rana", "Rana"},
		{"client not found", db.Client{}, false, "Anonymous", "Walk-in"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.found {
				if got := clientDisplayName(tc.client); got != tc.wantScreen {
					t.Fatalf("screen name = %q, want %q", got, tc.wantScreen)
				}
			}
			if got := receiptCustomerName(tc.client, tc.found); got != tc.wantReceipt {
				t.Fatalf("receipt name = %q, want %q", got, tc.wantReceipt)
			}
		})
	}
}

func TestInvoiceDetailsSayAnonymousButReprintSaysWalkIn(t *testing.T) {
	app, cashbox, walkIn, database := setupQuoteTestApp(t, 0, "half_up", "USD")
	defer database.Close()
	named, err := app.store.CreateClient(app.ctx, db.CreateClientParams{Name: "Rana Haddad", Phone: "70123456"})
	if err != nil {
		t.Fatal(err)
	}

	items := cartJSON(1)
	quote, err := app.QuoteInvoiceTotal(items)
	if err != nil {
		t.Fatal(err)
	}
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: cashbox.ID, AmountCents: quote.GrandTotalCents}})
	checkout := func(clientID int64) int64 {
		t.Helper()
		res, err := app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: items, PaymentsJSON: string(payments), ClientID: clientID})
		if err != nil {
			t.Fatal(err)
		}
		return res.InvoiceID
	}

	cases := []struct {
		name        string
		clientID    int64
		wantScreen  string
		wantWalkIn  bool
		wantReceipt string
	}{
		{"no client picked", 0, "Anonymous", true, "Walk-in"},
		{"walk-in client picked", walkIn.ID, "Anonymous", true, "Walk-in"},
		{"named client", named.ID, "Rana Haddad", false, "Rana Haddad"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := checkout(tc.clientID)
			details, err := app.GetInvoiceDetails(id)
			if err != nil {
				t.Fatal(err)
			}
			if details["clientName"] != tc.wantScreen {
				t.Fatalf("details clientName = %v, want %q", details["clientName"], tc.wantScreen)
			}
			if details["clientIsWalkIn"] != tc.wantWalkIn {
				t.Fatalf("details clientIsWalkIn = %v, want %v", details["clientIsWalkIn"], tc.wantWalkIn)
			}
			if got := receiptCustomerFromDetails(details); got != tc.wantReceipt {
				t.Fatalf("reprint customer = %q, want %q", got, tc.wantReceipt)
			}
			// A repeat read gives the same answer.
			again, _ := app.GetInvoiceDetails(id)
			if again["clientName"] != details["clientName"] {
				t.Fatalf("second read clientName = %v, first %v", again["clientName"], details["clientName"])
			}
		})
	}

	page, err := app.ListInvoicesPage(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range page.Items {
		if row["clientName"] == "Walk-in" || row["clientName"] == "Walk-in customer" {
			t.Fatalf("invoice list row still says %v: %#v", row["clientName"], row)
		}
		wantWalkIn := row["clientId"].(int64) != named.ID
		if row["clientIsWalkIn"] != wantWalkIn {
			t.Fatalf("row clientIsWalkIn = %v, want %v: %#v", row["clientIsWalkIn"], wantWalkIn, row)
		}
	}
}

func TestReceiptCustomerFromDetailsEdges(t *testing.T) {
	if got := receiptCustomerFromDetails(map[string]any{}); got != "Walk-in" {
		t.Fatalf("empty details = %q, want Walk-in", got)
	}
	if got := receiptCustomerFromDetails(map[string]any{"clientName": "Anonymous", "clientIsWalkIn": true}); got != "Walk-in" {
		t.Fatalf("walk-in details = %q, want Walk-in", got)
	}
	if got := receiptCustomerFromDetails(map[string]any{"clientName": "Rana", "clientIsWalkIn": false}); got != "Rana" {
		t.Fatalf("named details = %q, want Rana", got)
	}
}
