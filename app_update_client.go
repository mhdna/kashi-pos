package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// errNoAppUpdateRoute means this central predates remote updates (404 on
// /branch/app_update); the updater just stays quiet.
var errNoAppUpdateRoute = errors.New("Central has no app update route")

// appUpdateManifest is central's answer to "what should this till run?" --
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
	// Force: install as soon as it's here, even with a shift open.
	Force bool `json:"force"`
	// Set only because the request said it understands them (update_compact.go):
	// the full download is zstd of DownloadBytes, and Patch rebuilds the
	// release from the exe this till runs.
	Encoding      string              `json:"encoding"`
	DownloadBytes int64               `json:"download_bytes"`
	Patch         *appUpdatePatchInfo `json:"patch"`
}

type appUpdatePatchInfo struct {
	ID         int64  `json:"id"`
	BaseSha256 string `json:"base_sha256"`
	SizeBytes  int64  `json:"size_bytes"`
}

// GetAppUpdate returns this register's target release, or nil if it has none.
// have is the SHA-256 of the exe this till runs ("" if unknown): central offers
// a patch from exactly that build when it has one.
func (c *centralClient) GetAppUpdate(ctx context.Context, have string) (*appUpdateManifest, error) {
	if !c.enabled() {
		return nil, errCentralSyncDisabled
	}
	query := url.Values{"accept": {"zstd,patch"}}
	if have != "" {
		query.Set("have", have)
	}
	req, err := c.newRequest(ctx, http.MethodGet, "/branch/app_update?"+query.Encode(), nil)
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
	return c.downloadResumable(ctx, fmt.Sprintf("/branch/app_update/download/%d", id), path, progress)
}

// downloadResumable appends urlPath to the partial file at path, resuming
// from its size; the full build, its zstd form and a patch all come this way.
func (c *centralClient) downloadResumable(ctx context.Context, urlPath, path string, progress func(have int64)) error {
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

// appUpdateStatusReport is what the till tells central about itself and its
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
