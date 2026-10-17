package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	db "kashi-pos/db/sqlc"
)

// branchRecordCategoryPayload is the body POSTed to kashi's
// /branch/expenses/category and /branch/loans/category (see
// api/branch_record_category.go in the kashi repo).
type branchRecordCategoryPayload struct {
	ClientRef string `json:"client_ref"`
	Category  string `json:"category"`
}

// UpdateExpenseCategory changes an expense's category after it was made
// (the expense detail dialog).
func (a *App) UpdateExpenseCategory(id int64, category string) (map[string]any, error) {
	var expense db.Expense
	err := a.updateRecordCategory("expense", "$.category", func(q *db.Queries) (string, error) {
		var err error
		expense, err = q.SetExpenseCategory(a.ctx, db.SetExpenseCategoryParams{Category: category, ID: id})
		return expense.ClientRef, err
	}, category)
	if err != nil {
		return nil, err
	}
	return expenseMap(expense), nil
}

// UpdateLoanCategory is UpdateExpenseCategory for a loan.
func (a *App) UpdateLoanCategory(id int64, category string) (map[string]any, error) {
	if strings.TrimSpace(category) == "" {
		return nil, fmt.Errorf("a loan needs a category")
	}
	var loan db.Loan
	err := a.updateRecordCategory("loan", "$.category_name", func(q *db.Queries) (string, error) {
		var err error
		loan, err = q.SetLoanCategory(a.ctx, db.SetLoanCategoryParams{Category: category, ID: id})
		return loan.ClientRef, err
	}, category)
	if err != nil {
		return nil, err
	}
	return loanMap(loan), nil
}

// updateRecordCategory saves the new category locally and gets it to kashi.
// A record kashi hasn't accepted yet has the category rewritten in its own
// pending outbox row, so it goes up with the record. One kashi already has
// gets a "<kind>_category" row, keyed per record so a later edit replaces
// an unsent earlier one. It can't be sent ahead of the record: kashi's 404
// for a record it doesn't have yet would park the row as failed.
func (a *App) updateRecordCategory(kind, payloadField string, save func(q *db.Queries) (string, error), category string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	var queued bool
	err := a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		clientRef, err := save(q)
		if err != nil {
			return fmt.Errorf("update %s category: %w", kind, err)
		}
		settings, err := q.GetSettings(a.ctx)
		if err != nil {
			return fmt.Errorf("load settings: %w", err)
		}
		if settings.CentralApiUrl == "" || clientRef == "" {
			return nil
		}
		rewritten, err := q.SetUnsyncedSyncOutboxField(a.ctx, db.SetUnsyncedSyncOutboxFieldParams{
			FieldPath: payloadField, Value: category, ClientRef: clientRef,
		})
		if err != nil {
			return fmt.Errorf("update pending %s: %w", kind, err)
		}
		if rewritten > 0 {
			return nil
		}
		body, err := json.Marshal(branchRecordCategoryPayload{ClientRef: clientRef, Category: category})
		if err != nil {
			return fmt.Errorf("marshal sync outbox payload: %w", err)
		}
		queued = true
		return q.UpsertSyncOutboxEntry(a.ctx, db.UpsertSyncOutboxEntryParams{
			EntityType:  kind + "_category",
			ClientRef:   kind + "-category-" + clientRef,
			PayloadJson: string(body),
		})
	})
	if err != nil {
		return err
	}
	if queued {
		a.kickSyncWorker()
	}
	return nil
}

// AddExpensePhotosFromComputer lets the cashier pick photos on this
// computer instead of scanning the QR code: it gets a fresh upload link
// from kashi, like the QR, and posts the files to it. limit is how many
// more photos the expense can take. It returns how many were sent (0 when
// the picker was cancelled).
func (a *App) AddExpensePhotosFromComputer(clientRef string, limit int) (int, error) {
	client, token, err := a.requestUploadToken("/branch/expenses/upload_link", clientRef)
	if err != nil {
		return 0, err
	}
	return a.uploadPickedPhotos(client.UploadURL(token)+"/images", limit)
}

// AddLoanPhotosFromComputer is AddExpensePhotosFromComputer for a loan.
func (a *App) AddLoanPhotosFromComputer(clientRef string, limit int) (int, error) {
	client, token, err := a.requestUploadToken("/branch/loans/upload_link", clientRef)
	if err != nil {
		return 0, err
	}
	return a.uploadPickedPhotos(client.LoanUploadURL(token)+"/images", limit)
}

// The image types kashi's upload endpoints accept, by file extension.
var uploadPhotoTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
	".heic": "image/heic",
}

func (a *App) uploadPickedPhotos(uploadURL string, limit int) (int, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Choose photos",
		Filters: []runtime.FileFilter{{DisplayName: "Images", Pattern: "*.jpg;*.jpeg;*.png;*.webp;*.heic"}},
	})
	if err != nil {
		return 0, err
	}
	if len(paths) == 0 {
		return 0, nil
	}
	if limit > 0 && len(paths) > limit {
		return 0, fmt.Errorf("pick at most %d photo(s)", limit)
	}

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for _, path := range paths {
		contentType, ok := uploadPhotoTypes[strings.ToLower(filepath.Ext(path))]
		if !ok {
			return 0, fmt.Errorf("%s isn't a JPEG, PNG, WebP or HEIC photo", filepath.Base(path))
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return 0, fmt.Errorf("read %s: %w", filepath.Base(path), err)
		}
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "images", "filename": filepath.Base(path)}))
		header.Set("Content-Type", contentType)
		part, err := form.CreatePart(header)
		if err != nil {
			return 0, err
		}
		if _, err := part.Write(data); err != nil {
			return 0, err
		}
	}
	if err := form.Close(); err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(a.ctx, http.MethodPost, uploadURL, &body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	resp, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		return 0, fmt.Errorf("couldn't reach kashi: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e centralErrorBody
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&e)
		if e.Error == "" {
			e.Error = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return 0, fmt.Errorf("%s", e.Error)
	}
	return len(paths), nil
}
