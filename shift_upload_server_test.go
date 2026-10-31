package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The QR link is short: a 16-character token on the /s/ path. The page
// answers on it and on the long path links shown before still carry.
func TestShiftUploadShortLink(t *testing.T) {
	token, err := newShiftUploadToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 16 || strings.ContainsAny(token, "/+=") {
		t.Fatalf("token %q: want 16 URL-safe characters", token)
	}
	app := &App{}
	if url := app.shiftUploadURL(token); !strings.HasSuffix(url, "/s/"+token) {
		t.Fatalf("upload URL %q doesn't end in /s/<token>", url)
	}
	server := &shiftUploadServer{app: app}
	for _, path := range []string{"/s/" + token, "/shift_uploads/" + token} {
		rec := httptest.NewRecorder()
		server.route(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("GET %s = %d %q, want the upload page", path, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
}
