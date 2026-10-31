package main

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

// shiftReceiptCentral is a stand-in for kashi that records POSTs to
// /branch/shift_receipt_images and lets a test dictate the status they get.
type shiftReceiptCentral struct {
	server *httptest.Server

	mu             sync.Mutex
	receiptPosts   int
	closePosts     int
	lastShiftRef   string
	lastImageBytes int
	status         int
	closeStatus    int
	closeToken     string
}

func newShiftReceiptCentral(t *testing.T) *shiftReceiptCentral {
	t.Helper()
	fc := &shiftReceiptCentral{status: http.StatusOK, closeStatus: http.StatusOK, closeToken: ""}
	fc.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/branch/shift_closes" {
			fc.mu.Lock()
			fc.closePosts++
			cs, tok := fc.closeStatus, fc.closeToken
			fc.mu.Unlock()
			w.WriteHeader(cs)
			w.Write([]byte(`{"upload_token":"` + tok + `"}`))
			return
		}
		if r.URL.Path == "/branch/shift_closes/upload_status" {
			fc.mu.Lock()
			up := fc.receiptPosts > 0
			fc.mu.Unlock()
			w.WriteHeader(http.StatusOK)
			if up {
				w.Write([]byte(`{"images_uploaded":true}`))
			} else {
				w.Write([]byte(`{"images_uploaded":false}`))
			}
			return
		}
		if r.URL.Path != "/branch/shift_receipt_images" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		fc.mu.Lock()
		status := fc.status
		fc.mu.Unlock()
		if status != http.StatusOK {
			w.WriteHeader(status)
			w.Write([]byte(`{"error":"boom"}`))
			return
		}

		mediaType, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if !strings.HasPrefix(mediaType, "multipart/") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		var shiftRef string
		var imageLen int
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			data, _ := io.ReadAll(part)
			switch part.FormName() {
			case "shift_ref":
				shiftRef = string(data)
			case "image":
				imageLen = len(data)
			}
		}

		fc.mu.Lock()
		fc.receiptPosts++
		fc.lastShiftRef = shiftRef
		fc.lastImageBytes = imageLen
		fc.mu.Unlock()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))
	t.Cleanup(fc.server.Close)
	return fc
}

func (fc *shiftReceiptCentral) setStatus(status int) {
	fc.mu.Lock()
	fc.status = status
	fc.mu.Unlock()
}

func (fc *shiftReceiptCentral) posts() int {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	return fc.receiptPosts
}

func (fc *shiftReceiptCentral) setClose(status int, token string) {
	fc.mu.Lock()
	fc.closeStatus, fc.closeToken = status, token
	fc.mu.Unlock()
}

// captureReceipt inserts a shift_receipt_uploads row for shiftID and marks
// it captured against a real temp file -- the state the local upload server
// leaves it in once a phone has finished.
func captureReceipt(t *testing.T, app *App, store *db.Store, shiftID int64) db.ShiftReceiptUpload {
	t.Helper()
	row, err := store.CreateShiftReceiptUpload(app.ctx, db.CreateShiftReceiptUploadParams{
		ShiftID:   shiftID,
		Token:     "tok-" + shiftRef(shiftID),
		ExpiresAt: time.Now().UTC().Add(time.Hour).Format("2006-01-02 15:04:05"),
		Mode:      "local",
		UploadUrl: "http://127.0.0.1:8788/shift_uploads/tok-" + shiftRef(shiftID),
	})
	if err != nil {
		t.Fatalf("create shift receipt upload: %v", err)
	}

	path := filepath.Join(t.TempDir(), "receipt.jpg")
	if err := os.WriteFile(path, []byte("JPEGDATA-1234567890"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkShiftReceiptCaptured(app.ctx, db.MarkShiftReceiptCapturedParams{
		FilePath:    path,
		ContentType: "image/jpeg",
		SizeBytes:   19,
		ID:          row.ID,
	}); err != nil {
		t.Fatalf("mark captured: %v", err)
	}
	row, err = store.GetShiftReceiptUploadByToken(app.ctx, row.Token)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func closeShiftAndSync(t *testing.T, app *App, store *db.Store, fc *shiftReceiptCentral) int64 {
	t.Helper()
	shiftID, closer := openShiftAndCloser(t, app, store)
	if _, err := app.CloseShiftByID(shiftID, closer.ID, 0, 0, 0, "{}"); err != nil {
		t.Fatal(err)
	}
	if err := app.runSyncOutboxOnce(app.ctx, newCentralClient(fc.server.URL, "TEST01", "test-key")); err != nil {
		t.Fatal(err)
	}
	return shiftID
}

func TestShiftReceiptUploadWaitsForShiftCloseToSync(t *testing.T) {
	fc := newShiftReceiptCentral(t)
	app, store, _, _ := setUpTestInvoiceApp(t, fc.server.URL)

	// A photo captured for a shift whose close was never queued/synced.
	row := captureReceipt(t, app, store, 999)

	app.runShiftReceiptUploadsOnce(app.ctx, newCentralClient(fc.server.URL, "TEST01", "test-key"))

	if fc.posts() != 0 {
		t.Fatalf("forwarded a photo before its shift close synced (%d posts)", fc.posts())
	}
	got, _ := store.GetShiftReceiptUploadByToken(app.ctx, row.Token)
	if got.SyncedAt.Valid {
		t.Fatal("row marked synced despite no forward")
	}
}

func TestShiftReceiptUploadForwardsOnceCloseIsSynced(t *testing.T) {
	fc := newShiftReceiptCentral(t)
	app, store, _, _ := setUpTestInvoiceApp(t, fc.server.URL)
	shiftID := closeShiftAndSync(t, app, store, fc)
	row := captureReceipt(t, app, store, shiftID)

	client := newCentralClient(fc.server.URL, "TEST01", "test-key")
	app.runShiftReceiptUploadsOnce(app.ctx, client)

	if fc.posts() != 1 {
		t.Fatalf("posts to /branch/shift_receipt_images = %d, want 1", fc.posts())
	}
	if fc.lastShiftRef != shiftRef(shiftID) {
		t.Fatalf("forwarded shift_ref = %q, want %q", fc.lastShiftRef, shiftRef(shiftID))
	}
	if fc.lastImageBytes == 0 {
		t.Fatal("forwarded request carried no image bytes")
	}

	got, _ := store.GetShiftReceiptUploadByToken(app.ctx, row.Token)
	if !got.SyncedAt.Valid {
		t.Fatal("row not marked synced after a successful forward")
	}

	// Idempotency: a second pass must not re-POST.
	app.runShiftReceiptUploadsOnce(app.ctx, client)
	if fc.posts() != 1 {
		t.Fatalf("re-forwarded an already-synced photo (%d posts)", fc.posts())
	}
}

func TestShiftReceiptUploadRetriesAndResumesAfterFailure(t *testing.T) {
	fc := newShiftReceiptCentral(t)
	app, store, _, _ := setUpTestInvoiceApp(t, fc.server.URL)
	shiftID := closeShiftAndSync(t, app, store, fc)
	row := captureReceipt(t, app, store, shiftID)
	client := newCentralClient(fc.server.URL, "TEST01", "test-key")

	fc.setStatus(http.StatusInternalServerError)
	app.runShiftReceiptUploadsOnce(app.ctx, client)

	got, _ := store.GetShiftReceiptUploadByToken(app.ctx, row.Token)
	if got.SyncedAt.Valid {
		t.Fatal("row marked synced despite a 500 from kashi")
	}
	if got.AttemptCount != 1 {
		t.Fatalf("attempt_count = %d, want 1 after one failed forward", got.AttemptCount)
	}

	// Simulate the backoff elapsing / an app restart: the row is still in
	// the table, so re-arming next_attempt_at is all it takes to retry.
	if err := store.MarkShiftReceiptUploadAttemptFailed(app.ctx, db.MarkShiftReceiptUploadAttemptFailedParams{
		NextAttemptAt: "2000-01-01 00:00:00",
		ID:            row.ID,
	}); err != nil {
		t.Fatal(err)
	}
	fc.setStatus(http.StatusOK)
	app.runShiftReceiptUploadsOnce(app.ctx, client)

	got, _ = store.GetShiftReceiptUploadByToken(app.ctx, row.Token)
	if !got.SyncedAt.Valid {
		t.Fatal("row not marked synced after kashi recovered")
	}
	if fc.posts() != 1 {
		t.Fatalf("successful forwards = %d, want 1", fc.posts())
	}
}

func setUploadMode(t *testing.T, store *db.Store, mode int64) {
	t.Helper()
	if err := store.SetShiftReceiptUploadMode(context.Background(), mode); err != nil {
		t.Fatal(err)
	}
}

func TestResolveShiftReceiptUpload_CentralModeUsesKashiURL(t *testing.T) {
	fc := newShiftReceiptCentral(t)
	fc.setClose(http.StatusOK, "central-tok-123")
	app, store, _, _ := setUpTestInvoiceApp(t, fc.server.URL)
	setUploadMode(t, store, shiftUploadModeCentral)

	shiftID, closer := openShiftAndCloser(t, app, store)
	if _, err := app.CloseShiftByID(shiftID, closer.ID, 0, 0, 0, "{}"); err != nil {
		t.Fatal(err)
	}

	ref := shiftRef(shiftID)
	upload := app.resolveShiftReceiptUpload(shiftID, ref, branchShiftClosePayload{
		ShiftRef: ref, BranchShiftID: shiftID, OccurredAt: time.Now().UTC(),
	}, fc.server.URL)
	if upload == nil {
		t.Fatal("resolveShiftReceiptUpload returned nil in central mode")
	}
	if upload["mode"] != "central" {
		t.Fatalf("mode = %v, want central", upload["mode"])
	}
	if got := upload["url"].(string); got != fc.server.URL+"/s/central-tok-123" {
		t.Fatalf("url = %q", got)
	}

	row, err := store.GetShiftReceiptUploadByToken(app.ctx, "central-tok-123")
	if err != nil {
		t.Fatalf("central-mode row not written: %v", err)
	}
	if row.Mode != "central" {
		t.Fatalf("row mode = %q, want central", row.Mode)
	}

	// The queued outbox row for this close is now redundant.
	synced, _ := store.ShiftCloseSyncedForRef(app.ctx, ref)
	if synced != 1 {
		t.Fatal("outbox shift_close row was not marked synced after the direct push")
	}
}

func TestResolveShiftReceiptUpload_HybridFallsBackToLocal(t *testing.T) {
	fc := newShiftReceiptCentral(t)
	fc.setClose(http.StatusInternalServerError, "") // central push fails
	app, store, _, _ := setUpTestInvoiceApp(t, fc.server.URL)
	setUploadMode(t, store, shiftUploadModeHybrid)

	shiftID, closer := openShiftAndCloser(t, app, store)
	if _, err := app.CloseShiftByID(shiftID, closer.ID, 0, 0, 0, "{}"); err != nil {
		t.Fatal(err)
	}

	ref := shiftRef(shiftID)
	upload := app.resolveShiftReceiptUpload(shiftID, ref, branchShiftClosePayload{
		ShiftRef: ref, BranchShiftID: shiftID, OccurredAt: time.Now().UTC(),
	}, fc.server.URL)
	if upload == nil {
		t.Fatal("hybrid mode returned nil instead of falling back to local")
	}
	if upload["mode"] != "local" {
		t.Fatalf("mode = %v, want local (fallback)", upload["mode"])
	}
}

func TestResolveShiftReceiptUpload_OffMode(t *testing.T) {
	fc := newShiftReceiptCentral(t)
	app, store, _, _ := setUpTestInvoiceApp(t, fc.server.URL)
	setUploadMode(t, store, shiftUploadModeOff)

	if upload := app.resolveShiftReceiptUpload(1, "shift-1", branchShiftClosePayload{}, fc.server.URL); upload != nil {
		t.Fatalf("off mode returned %v, want nil", upload)
	}
}
