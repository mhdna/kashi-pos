package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Photos of this till's expenses and transfers. The phone uploads them
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
	client := a.photoClient()
	if client == nil || clientRef == "" {
		return []int64{}, nil
	}
	ctx, cancel := context.WithTimeout(a.ctx, photoFetchTimeout)
	defer cancel()
	path := "/branch/expense_photos?client_ref=" + url.QueryEscape(clientRef)
	req, err := client.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return []int64{}, nil
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return []int64{}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return []int64{}, nil
	}
	var out struct {
		Images []struct {
			ID int64 `json:"id"`
		} `json:"images"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return []int64{}, nil
	}
	ids := make([]int64, 0, len(out.Images))
	for _, image := range out.Images {
		ids = append(ids, image.ID)
	}
	return ids, nil
}

// GetExpensePhoto returns one expense photo as a data: URL ("" if missing).
func (a *App) GetExpensePhoto(id int64, width int) (string, error) {
	client := a.photoClient()
	if client == nil || id <= 0 {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(a.ctx, photoFetchTimeout)
	defer cancel()
	path := fmt.Sprintf("/branch/expense_photos/%d/file", id)
	if q := widthQuery(width); q != "" {
		path += "?" + q
	}
	data, err := client.fetchImageDataURL(ctx, path)
	if err != nil {
		return "", nil
	}
	return data, nil
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
