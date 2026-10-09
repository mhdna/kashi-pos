package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// errNoAppUpdateRoute means this kashi predates remote updates (404 on
// /branch/app_update); the updater just stays quiet.
var errNoAppUpdateRoute = errors.New("kashi has no app update route")

// appUpdateManifest is kashi's answer to "what should this till run?" --
// see api/app_update.go there.
type appUpdateManifest struct {
	ID        int64  `json:"id"`
	Version   string `json:"version"`
	Flavor    string `json:"flavor"`
	FileName  string `json:"file_name"`
	SizeBytes int64  `json:"size_bytes"`
	Sha256    string `json:"sha256"`
	Signature string `json:"signature"`
	Notes     string `json:"notes"`
}

// GetAppUpdate returns this register's target release, or nil if it has none.
func (c *centralClient) GetAppUpdate(ctx context.Context) (*appUpdateManifest, error) {
	if !c.enabled() {
		return nil, errCentralSyncDisabled
	}
	req, err := c.newRequest(ctx, http.MethodGet, "/branch/app_update", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errNoAppUpdateRoute
	}
	if resp.StatusCode != http.StatusOK {
		return nil, responseError(http.MethodGet, "/branch/app_update", resp)
	}
	var out struct {
		Release *appUpdateManifest `json:"release"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode app update: %w", err)
	}
	return out.Release, nil
}

// DownloadAppUpdate appends release id to the partial file at path,
// resuming from its current size, and calls progress with the bytes it has.
// The 10s client timeout would cut off a 30MB download on a shop line, so
// this uses its own client bounded only by ctx.
func (c *centralClient) DownloadAppUpdate(ctx context.Context, id int64, path string, progress func(have int64)) error {
	if !c.enabled() {
		return errCentralSyncDisabled
	}
	// Not O_APPEND: on Windows an append-only handle can't be truncated
	// ("Access is denied"), and a 200 below has to start the file over.
	// Seeking to the end gives the same resume point.
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	have, err := out.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	urlPath := fmt.Sprintf("/branch/app_update/download/%d", id)
	req, err := c.newRequest(ctx, http.MethodGet, urlPath, nil)
	if err != nil {
		return err
	}
	if have > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
	}
	client := &http.Client{Transport: c.httpClient.Transport}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusPartialContent:
	case http.StatusOK:
		// The server sent the whole file (no range support, or a stale
		// partial): start over.
		if have > 0 {
			if err := out.Truncate(0); err != nil {
				return err
			}
			if _, err := out.Seek(0, io.SeekStart); err != nil {
				return err
			}
		}
		have = 0
	case http.StatusRequestedRangeNotSatisfiable:
		return nil // already complete; verification decides
	default:
		return responseError(http.MethodGet, urlPath, resp)
	}

	buf := make([]byte, 64<<10)
	lastReport := time.Time{}
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				return err
			}
			have += int64(n)
			if time.Since(lastReport) > time.Second {
				lastReport = time.Now()
				progress(have)
			}
		}
		if readErr == io.EOF {
			progress(have)
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

// appUpdateStatusReport is what the till tells kashi about itself and its
// update -- see branchAppUpdateStatus there.
type appUpdateStatusReport struct {
	AppVersion string `json:"app_version"`
	AppFlavor  string `json:"app_flavor"`
	State      string `json:"state"`
	ReleaseID  int64  `json:"release_id"`
	Detail     string `json:"detail"`
	Progress   int    `json:"progress"`
}

func (c *centralClient) PostAppUpdateStatus(ctx context.Context, report appUpdateStatusReport) error {
	body, err := json.Marshal(report)
	if err != nil {
		return err
	}
	return c.sendJSON(ctx, http.MethodPost, "/branch/app_update/status", body)
}
