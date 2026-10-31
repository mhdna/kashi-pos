package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	db "kashi-pos/db/sqlc"
)

// newShiftUploadToken mints the secret embedded in the QR URL: 12 random
// bytes as 16 URL-safe characters, short enough to type, and unguessable for
// a single-use link that expires (kashi mints its tokens the same way).
func newShiftUploadToken() (string, error) {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// The close-shift receipt photo lands on the POS itself first (see the
// shift_receipt_uploads table), so the cashier's phone never needs an
// internet connection -- only the same LAN as the till. shift_upload_server
// is the tiny always-on HTTP listener that phone talks to; sync_shift_receipts.go
// then forwards whatever it received on to kashi, retrying forever.
const (
	shiftUploadTokenTTL       = 2 * time.Hour
	maxShiftReceiptImageBytes = 10 << 20 // 10MB
	defaultShiftUploadListen  = "0.0.0.0:8788"
)

// shiftReceiptImageExt mirrors allowedExpenseImageContentTypes in the kashi
// repo's api/expense_upload.go -- the same four types kashi will accept on
// the far end, checked here too so a bad file is rejected before it's
// buffered rather than looping forever in the uploader.
var shiftReceiptImageExt = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/heic": ".heic",
}

//go:embed shift_upload_assets/upload.html
var shiftUploadPageHTML []byte

type shiftUploadServer struct {
	app    *App
	server *http.Server
}

// shiftUploadListenAddr is where the local upload server binds. It listens
// on all interfaces by default so a phone on the shop wifi can reach it;
// override with KASHI_SHIFT_UPLOAD_LISTEN (e.g. to change the port, or pin
// it to one interface).
func (a *App) shiftUploadListenAddr() string {
	if v := strings.TrimSpace(os.Getenv("KASHI_SHIFT_UPLOAD_LISTEN")); v != "" {
		return v
	}
	return defaultShiftUploadListen
}

func (a *App) startShiftUploadServer() {
	if a.store == nil {
		return
	}
	addr := a.shiftUploadListenAddr()

	s := &shiftUploadServer{app: a}
	mux := http.NewServeMux()
	mux.HandleFunc("/shift_uploads/", s.route)
	// The short path the QR code carries; the long one is for links
	// shown before.
	mux.HandleFunc("/s/", s.route)
	s.server = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
	}
	a.shiftUpload = s

	go func() {
		log.Printf("shift receipt upload server listening on %s", addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("shift receipt upload server stopped: %v", err)
		}
	}()
}

func (a *App) stopShiftUploadServer(ctx context.Context) {
	if a.shiftUpload != nil && a.shiftUpload.server != nil {
		if err := a.shiftUpload.server.Shutdown(ctx); err != nil {
			log.Printf("stop shift receipt upload server: %v", err)
		}
	}
}

// shiftUploadURL builds the link the close-shift screen turns into a QR
// code: the POS's own LAN address plus the one-time token. Falls back to
// the loopback address if no routable interface is found -- useless for a
// phone, but it keeps the flow from crashing and the URL is still shown as
// text for manual entry.
func (a *App) shiftUploadURL(token string) string {
	_, port, err := net.SplitHostPort(a.shiftUploadListenAddr())
	if err != nil || port == "" {
		port = "8788"
	}
	host := outboundLANIP()
	if host == "" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%s/s/%s", host, port, token)
}

// outboundLANIP returns this machine's primary non-loopback IPv4 address by
// asking the OS which local address it would use to reach an external host
// (no packet is actually sent -- a UDP socket's Dial just resolves the
// route). Empty string if there's no usable route.
func outboundLANIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return addr.IP.String()
	}
	return ""
}

func (s *shiftUploadServer) route(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/shift_uploads/")
	if rest == r.URL.Path {
		rest = strings.TrimPrefix(r.URL.Path, "/s/")
	}
	segments := strings.Split(rest, "/")
	token := segments[0]
	if token == "" {
		http.NotFound(w, r)
		return
	}

	switch {
	case len(segments) == 1 && r.Method == http.MethodGet:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(shiftUploadPageHTML)
	case len(segments) == 2 && segments[1] == "status" && r.Method == http.MethodGet:
		s.status(w, token)
	case len(segments) == 2 && segments[1] == "images" && r.Method == http.MethodPost:
		s.upload(w, r, token)
	default:
		http.NotFound(w, r)
	}
}

func writeShiftUploadError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// lookupValidToken resolves a token to its row, treating "not found" and
// "expired" identically -- a phone scanning a stale QR doesn't need the two
// told apart.
func (s *shiftUploadServer) lookupValidToken(token string) (db.ShiftReceiptUpload, error) {
	row, err := s.app.store.GetShiftReceiptUploadByToken(s.app.ctx, token)
	if err != nil {
		return db.ShiftReceiptUpload{}, err
	}
	if exp := parseShiftTimestamp(row.ExpiresAt); !exp.IsZero() && time.Now().UTC().After(exp) {
		return db.ShiftReceiptUpload{}, sql.ErrNoRows
	}
	return row, nil
}

func (s *shiftUploadServer) status(w http.ResponseWriter, token string) {
	row, err := s.lookupValidToken(token)
	if err != nil {
		if err == sql.ErrNoRows {
			writeShiftUploadError(w, http.StatusNotFound, "this link is no longer valid")
			return
		}
		writeShiftUploadError(w, http.StatusInternalServerError, err.Error())
		return
	}

	subtitle := fmt.Sprintf("Shift #%d", row.ShiftID)
	if shift, err := s.app.store.GetShiftByID(s.app.ctx, row.ShiftID); err == nil && shift.ClosedAt != "" {
		subtitle += " · closed " + formatShiftReportTime(shift.ClosedAt)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"subtitle": subtitle,
		"used":     row.CapturedAt.Valid,
	})
}

// upload is the only write path the phone can reach. It re-validates the
// token from scratch (unexpired, not already captured) rather than trusting
// an earlier /status call, which is read-only and racy against a second
// device on the same link.
func (s *shiftUploadServer) upload(w http.ResponseWriter, r *http.Request, token string) {
	row, err := s.lookupValidToken(token)
	if err != nil {
		if err == sql.ErrNoRows {
			writeShiftUploadError(w, http.StatusNotFound, "this link is no longer valid")
			return
		}
		writeShiftUploadError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if row.CapturedAt.Valid {
		writeShiftUploadError(w, http.StatusGone, "a photo has already been uploaded for this shift")
		return
	}

	if err := r.ParseMultipartForm(maxShiftReceiptImageBytes + (1 << 20)); err != nil {
		writeShiftUploadError(w, http.StatusBadRequest, "could not read the upload")
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeShiftUploadError(w, http.StatusBadRequest, "no image provided")
		return
	}
	defer file.Close()

	if header.Size > maxShiftReceiptImageBytes {
		writeShiftUploadError(w, http.StatusBadRequest, fmt.Sprintf("the image must be under %dMB", maxShiftReceiptImageBytes>>20))
		return
	}
	contentType := header.Header.Get("Content-Type")
	ext, ok := shiftReceiptImageExt[contentType]
	if !ok {
		writeShiftUploadError(w, http.StatusBadRequest, fmt.Sprintf("unsupported image type %q", contentType))
		return
	}

	destDir := filepath.Join(shiftReceiptStorageDir(), fmt.Sprintf("%d", row.ShiftID))
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		writeShiftUploadError(w, http.StatusInternalServerError, err.Error())
		return
	}
	destPath := filepath.Join(destDir, uuid.NewString()+ext)

	dst, err := os.Create(destPath)
	if err != nil {
		writeShiftUploadError(w, http.StatusInternalServerError, err.Error())
		return
	}
	written, err := io.Copy(dst, file)
	if err != nil {
		dst.Close()
		os.Remove(destPath)
		writeShiftUploadError(w, http.StatusInternalServerError, err.Error())
		return
	}
	dst.Close()

	if err := s.app.store.MarkShiftReceiptCaptured(s.app.ctx, db.MarkShiftReceiptCapturedParams{
		FilePath:    destPath,
		ContentType: contentType,
		SizeBytes:   written,
		ID:          row.ID,
	}); err != nil {
		os.Remove(destPath)
		writeShiftUploadError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Wake the close-shift screen and the uploader immediately.
	if a := s.app; a != nil {
		select {
		case <-a.ready:
			runtime.EventsEmit(a.ctx, "shift_receipt_captured:"+token)
		default:
		}
		a.kickSyncWorker()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// shiftReceiptStorageDir is where the buffered photo lives on the POS until
// kashi confirms it. Kept beside the database under the per-user config dir.
func shiftReceiptStorageDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "kashi-pos", "shift-receipts")
	}
	return filepath.Join(os.TempDir(), "kashi-pos-shift-receipts")
}
