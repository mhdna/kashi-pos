package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Photos of this till's expenses, loans and transfers. The phone uploads them
// straight to kashi (QR links), so the till has no copy: the Expenses and
// Transfers pages fetch them from kashi's /branch/expense_photos and
// /branch/transfer_photo, which only ever serve this branch's own. A width
// asks kashi for a thumbnail. Offline, unsynced or no photo all come back
// empty, which the pages show as an icon instead of a picture.

const photoFetchTimeout = 15 * time.Second

func (a *App) photoClient() *centralClient {
	if a.store == nil {
		return nil
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil || settings.CentralApiUrl == "" {
		return nil
	}
	client := newCentralClient(settings.CentralApiUrl, settings.BranchCode, settings.ApiKey)
	if !client.enabled() {
		return nil
	}
	return client
}

// fetchImageDataURL GETs an image from kashi as a data: URL the screen can
// put straight into an <img>. A 404 (no photo) is "", not an error.
func (c *centralClient) fetchImageDataURL(ctx context.Context, path string) (string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", responseError(http.MethodGet, path, resp)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return "", err
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}

func widthQuery(width int) string {
	if width > 0 {
		return fmt.Sprintf("w=%d", width)
	}
	return ""
}

// ListExpensePhotoIDs returns kashi's ids of an expense's photos (empty when
// it has none, or kashi can't be reached).
func (a *App) ListExpensePhotoIDs(clientRef string) ([]int64, error) {
	return a.listPhotoIDs("/branch/expense_photos", clientRef), nil
}

// ListLoanPhotoIDs is ListExpensePhotoIDs for a loan.
func (a *App) ListLoanPhotoIDs(clientRef string) ([]int64, error) {
	return a.listPhotoIDs("/branch/loan_photos", clientRef), nil
}

func (a *App) listPhotoIDs(path, clientRef string) []int64 {
	client := a.photoClient()
	if client == nil || clientRef == "" {
		return []int64{}
	}
	ctx, cancel := context.WithTimeout(a.ctx, photoFetchTimeout)
	defer cancel()
	path += "?client_ref=" + url.QueryEscape(clientRef)
	req, err := client.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return []int64{}
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return []int64{}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return []int64{}
	}
	var out struct {
		Images []struct {
			ID int64 `json:"id"`
		} `json:"images"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return []int64{}
	}
	ids := make([]int64, 0, len(out.Images))
	for _, image := range out.Images {
		ids = append(ids, image.ID)
	}
	return ids
}

// GetExpensePhoto returns one expense photo as a data: URL ("" if missing).
func (a *App) GetExpensePhoto(id int64, width int) (string, error) {
	return a.getPhoto("/branch/expense_photos", id, width), nil
}

// GetExpenseFirstPhoto is the first photo of an expense by the till's own
// clientRef, as a data: URL, in one request ("" if none). The list rows use
// it instead of listing ids and then fetching a file: two round trips each.
func (a *App) GetExpenseFirstPhoto(clientRef string, width int) (string, error) {
	return a.firstPhoto("/branch/expense_photos/first", clientRef, width), nil
}

// GetLoanFirstPhoto is GetExpenseFirstPhoto for a loan.
func (a *App) GetLoanFirstPhoto(clientRef string, width int) (string, error) {
	return a.firstPhoto("/branch/loan_photos/first", clientRef, width), nil
}

func (a *App) firstPhoto(base, clientRef string, width int) string {
	client := a.photoClient()
	if client == nil || clientRef == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(a.ctx, photoFetchTimeout)
	defer cancel()
	params := url.Values{}
	params.Set("client_ref", clientRef)
	if width > 0 {
		params.Set("w", fmt.Sprint(width))
	}
	data, err := client.fetchImageDataURL(ctx, base+"?"+params.Encode())
	if err != nil {
		return ""
	}
	return data
}

// GetLoanPhoto is GetExpensePhoto for a loan's photo.
func (a *App) GetLoanPhoto(id int64, width int) (string, error) {
	return a.getPhoto("/branch/loan_photos", id, width), nil
}

func (a *App) getPhoto(base string, id int64, width int) string {
	client := a.photoClient()
	if client == nil || id <= 0 {
		return ""
	}
	ctx, cancel := context.WithTimeout(a.ctx, photoFetchTimeout)
	defer cancel()
	path := fmt.Sprintf("%s/%d/file", base, id)
	if q := widthQuery(width); q != "" {
		path += "?" + q
	}
	data, err := client.fetchImageDataURL(ctx, path)
	if err != nil {
		return ""
	}
	return data
}

// RequestExpenseUploadLink asks kashi for a new photo link for an expense
// whose photo wasn't sent when it was made, and returns the URL to show as a
// QR code. It fails if kashi hasn't received the expense yet or it already
// has 3 photos.
func (a *App) RequestExpenseUploadLink(clientRef string) (string, error) {
	client, token, err := a.requestUploadToken("/branch/expenses/upload_link", clientRef)
	if err != nil {
		return "", err
	}
	return client.UploadURL(token), nil
}

// RequestLoanUploadLink is RequestExpenseUploadLink for a loan.
func (a *App) RequestLoanUploadLink(clientRef string) (string, error) {
	client, token, err := a.requestUploadToken("/branch/loans/upload_link", clientRef)
	if err != nil {
		return "", err
	}
	return client.LoanUploadURL(token), nil
}

func (a *App) requestUploadToken(path, clientRef string) (*centralClient, string, error) {
	client := a.photoClient()
	if client == nil {
		return nil, "", fmt.Errorf("this till isn't connected to kashi")
	}
	if clientRef == "" {
		return nil, "", fmt.Errorf("this record has no reference to send")
	}
	body, _ := json.Marshal(map[string]string{"client_ref": clientRef})
	ctx, cancel := context.WithTimeout(a.ctx, photoFetchTimeout)
	defer cancel()
	req, err := client.newRequest(ctx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("couldn't reach kashi: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e centralErrorBody
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error == "" {
			e.Error = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return nil, "", fmt.Errorf("%s", e.Error)
	}
	var out struct {
		UploadToken string `json:"upload_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.UploadToken == "" {
		return nil, "", fmt.Errorf("kashi didn't send a link")
	}
	return client, out.UploadToken, nil
}

// GetTransferPhoto returns a transfer's photo as a data: URL ("" if none):
// by this till's clientRef for one it sent, by kashi's transferID for one it
// receives.
func (a *App) GetTransferPhoto(clientRef string, transferID int64, width int) (string, error) {
	client := a.photoClient()
	if client == nil {
		return "", nil
	}
	params := url.Values{}
	switch {
	case clientRef != "":
		params.Set("client_ref", clientRef)
	case transferID > 0:
		params.Set("transfer_id", fmt.Sprint(transferID))
	default:
		return "", nil
	}
	if width > 0 {
		params.Set("w", fmt.Sprint(width))
	}
	ctx, cancel := context.WithTimeout(a.ctx, photoFetchTimeout)
	defer cancel()
	data, err := client.fetchImageDataURL(ctx, "/branch/transfer_photo?"+params.Encode())
	if err != nil {
		return "", nil
	}
	return data, nil
}
