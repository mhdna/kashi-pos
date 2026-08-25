package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCentralClientDisabledWhenNotConfigured(t *testing.T) {
	var c *centralClient
	if err := c.Health(context.Background()); err != errCentralSyncDisabled {
		t.Fatalf("expected errCentralSyncDisabled for nil client, got %v", err)
	}

	c = newCentralClient("", "", "")
	if err := c.Health(context.Background()); err != errCentralSyncDisabled {
		t.Fatalf("expected errCentralSyncDisabled for empty config, got %v", err)
	}
}

func TestCentralClientHealthSendsBranchHeaders(t *testing.T) {
	var gotCode, gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCode = r.Header.Get("X-Branch-Code")
		gotKey = r.Header.Get("X-Branch-Key")
		if r.URL.Path != "/branch/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"branch_id":1}`))
	}))
	defer server.Close()

	c := newCentralClient(server.URL, "beirut-01", "test-key")
	if err := c.Health(context.Background()); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if gotCode != "beirut-01" || gotKey != "test-key" {
		t.Fatalf("expected headers beirut-01/test-key, got %s/%s", gotCode, gotKey)
	}
}

func TestCentralClientHealthReportsServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid branch key"}`))
	}))
	defer server.Close()

	c := newCentralClient(server.URL, "beirut-01", "wrong-key")
	err := c.Health(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
}
