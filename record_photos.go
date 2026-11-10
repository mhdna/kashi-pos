package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Photos of this till's expenses, loans and transfers. The phone uploads them
// straight to kashi (QR links), so the till has no copy of its own: the
// Expenses, Loans and Transfers pages fetch them from kashi's
// /branch/expense_photos, /branch/loan_photos and /branch/transfer_photo,
// which only ever serve this branch's own. A width asks kashi for a
// thumbnail. No photo, or kashi unreachable with nothing saved, comes back
// empty, which the pages show as an icon instead of a picture.
//
// Every answer is kept in the encrypted photoCache (photo_cache.go), so a
// photo seen once opens from disk after that, offline too. A saved copy is
// shown at once and, if kashi last confirmed it over photoRecheckAfter ago,
// checked in the background with its ETag (kashi answers 304 when it's
// unchanged). When kashi has something else now -- the photo was turned or
// replaced, or one was added -- the copy is replaced and
// "record_photos_changed" tells the page which record to redraw.

const photoFetchTimeout = 15 * time.Second

// photoRecheckAfter is how long a saved copy is trusted before the next
// showing asks kashi again (in the background; the copy still shows first).
const photoRecheckAfter = 30 * time.Second

// photoRecheckSlots caps the background checks running at once, so opening
// a long list doesn't fire a request per row together.
var photoRecheckSlots = make(chan struct{}, 4)

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

// cachedPhotos opens the photo cache the first time it's needed; a
// till where it can't be opened just fetches every photo from kashi.
func (a *App) cachedPhotos() *photoCache {
	a.photoCacheOnce.Do(func() {
		if a.photoCache != nil {
			return
		}
		path, err := photoCachePath()
		if err != nil {
			log.Printf("photo cache: %v", err)
			return
		}
		cache, err := openPhotoCache(path)
		if err != nil {
			log.Printf("photo cache: open: %v", err)
			return
		}
		a.photoCache = cache
	})
	return a.photoCache
}

// closePhotoCache waits for background checks and closes the cache.
func (a *App) closePhotoCache() {
	a.photoWG.Wait()
	if a.photoCache != nil {
		_ = a.photoCache.close()
	}
}

// photoCacheKey names a copy by the kashi, branch and request it came from,
// so a till moved to another branch or server never shows the old one's.
func photoCacheKey(client *centralClient, path string) string {
	sum := sha256.Sum256([]byte(client.baseURL + "\x00" + client.branchCode + "\x00" + path))
	return hex.EncodeToString(sum[:])
}

// fetchPhoto GETs one image (or photo list) from kashi. With an etag it asks
// for it only if it changed: notModified means the saved copy is current.
// A 404 is a Missing photo, not an error.
func (c *centralClient) fetchPhoto(ctx context.Context, path, etag string) (p cachedPhoto, notModified bool, err error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return cachedPhoto{}, false, err
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return cachedPhoto{}, false, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotModified:
		return cachedPhoto{}, true, nil
	case http.StatusNotFound:
		return cachedPhoto{Missing: true}, false, nil
	case http.StatusOK:
	default:
		return cachedPhoto{}, false, responseError(http.MethodGet, path, resp)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return cachedPhoto{}, false, err
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}
	return cachedPhoto{Body: body, ContentType: contentType, ETag: resp.Header.Get("ETag")}, false, nil
}

// cachedPhoto returns kashi's answer for path: the saved copy if there is
// one (checked in the background when due), else fetched now and saved. ref
// is the record it belongs to. ok is false when there's no copy and kashi
// couldn't be reached.
func (a *App) cachedPhoto(client *centralClient, path, ref string) (cachedPhoto, bool) {
	cache := a.cachedPhotos()
	key := photoCacheKey(client, path)
	if cache != nil {
		if p, ok := cache.get(key); ok {
			if cache.now().Sub(p.CheckedAt) >= photoRecheckAfter {
				a.recheckPhoto(cache, client, path, ref, key, p)
			}
			return p, true
		}
	}
	ctx, cancel := context.WithTimeout(a.ctx, photoFetchTimeout)
	defer cancel()
	p, _, err := client.fetchPhoto(ctx, path, "")
	if err != nil {
		return cachedPhoto{}, false
	}
	if cache != nil {
		if err := cache.put(key, ref, p); err != nil {
			log.Printf("photo cache: save: %v", err)
		}
	}
	return p, true
}

// livePhoto asks kashi first and falls back to the saved copy only when
// kashi can't be reached -- for the photo id lists, which the upload dialog
// polls to see a new photo arrive and so must never answer from a copy
// while kashi is there to ask.
func (a *App) livePhoto(client *centralClient, path, ref string) (cachedPhoto, bool) {
	cache := a.cachedPhotos()
	key := photoCacheKey(client, path)
	ctx, cancel := context.WithTimeout(a.ctx, photoFetchTimeout)
	defer cancel()
	p, _, err := client.fetchPhoto(ctx, path, "")
	if err == nil {
		if cache != nil {
			_ = cache.put(key, ref, p)
		}
		return p, true
	}
	if cache != nil {
		return cache.get(key)
	}
	return cachedPhoto{}, false
}

// recheckPhoto asks kashi, in the background, whether the saved copy old is
// still what path returns, and replaces it if not. One check per copy at a
// time; a failed check (offline) leaves the copy as it is.
func (a *App) recheckPhoto(cache *photoCache, client *centralClient, path, ref, key string, old cachedPhoto) {
	if _, busy := a.photoRechecks.LoadOrStore(key, true); busy {
		return
	}
	a.photoWG.Add(1)
	go func() {
		defer a.photoWG.Done()
		defer a.photoRechecks.Delete(key)
		photoRecheckSlots <- struct{}{}
		defer func() { <-photoRecheckSlots }()

		ctx, cancel := context.WithTimeout(a.ctx, photoFetchTimeout)
		defer cancel()
		etag := old.ETag
		if old.Missing {
			etag = ""
		}
		p, notModified, err := client.fetchPhoto(ctx, path, etag)
		switch {
		case err != nil:
			return
		case notModified, p.Missing && old.Missing,
			!p.Missing && !old.Missing && p.ETag == old.ETag && bytes.Equal(p.Body, old.Body):
			cache.markChecked(key)
			return
		}
		if err := cache.put(key, ref, p); err != nil {
			log.Printf("photo cache: save: %v", err)
			return
		}
		a.photosChanged(ref)
	}()
}

// photosChanged tells the screen that ref's photos are different now.
func (a *App) photosChanged(ref string) {
	if a.onPhotosChanged != nil {
		a.onPhotosChanged(ref)
		return
	}
	select {
	case <-a.ready:
		runtime.EventsEmit(a.ctx, "record_photos_changed", ref)
	default:
	}
}

// forgetPhotos drops a record's saved photos once the till knows new ones
// arrived, so its "no photo" or old first photo isn't shown again.
func (a *App) forgetPhotos(ref string) {
	if cache := a.cachedPhotos(); cache != nil {
		cache.forgetRef(ref)
	}
}

// photoDataURL is a copy as a data: URL for an <img>, "" for no photo.
func photoDataURL(p cachedPhoto, ok bool) string {
	if !ok || p.Missing || len(p.Body) == 0 {
		return ""
	}
	return "data:" + p.ContentType + ";base64," + base64.StdEncoding.EncodeToString(p.Body)
}

func widthQuery(width int) string {
	if width > 0 {
		return fmt.Sprintf("w=%d", width)
	}
	return ""
}

// ListExpensePhotoIDs returns kashi's ids of an expense's photos (empty when
// it has none, or kashi can't be reached and none were saved).
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
	p, ok := a.livePhoto(client, path+"?client_ref="+url.QueryEscape(clientRef), clientRef)
	if !ok || p.Missing {
		return []int64{}
	}
	var out struct {
		Images []struct {
			ID int64 `json:"id"`
		} `json:"images"`
	}
	if err := json.Unmarshal(p.Body, &out); err != nil {
		return []int64{}
	}
	ids := make([]int64, 0, len(out.Images))
	for _, image := range out.Images {
		ids = append(ids, image.ID)
	}
	return ids
}

// GetExpensePhoto returns one photo of the expense clientRef as a data: URL
// ("" if missing).
func (a *App) GetExpensePhoto(clientRef string, id int64, width int) (string, error) {
	return a.getPhoto("/branch/expense_photos", clientRef, id, width), nil
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
	params := url.Values{}
	params.Set("client_ref", clientRef)
	if width > 0 {
		params.Set("w", fmt.Sprint(width))
	}
	return photoDataURL(a.cachedPhoto(client, base+"?"+params.Encode(), clientRef))
}

// GetLoanPhoto is GetExpensePhoto for a loan's photo.
func (a *App) GetLoanPhoto(clientRef string, id int64, width int) (string, error) {
	return a.getPhoto("/branch/loan_photos", clientRef, id, width), nil
}

func (a *App) getPhoto(base, clientRef string, id int64, width int) string {
	client := a.photoClient()
	if client == nil || id <= 0 {
		return ""
	}
	path := fmt.Sprintf("%s/%d/file", base, id)
	if q := widthQuery(width); q != "" {
		path += "?" + q
	}
	return photoDataURL(a.cachedPhoto(client, path, clientRef))
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
// receives. Its record ref is transferPhotoRef.
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
	return photoDataURL(a.cachedPhoto(client, "/branch/transfer_photo?"+params.Encode(), transferPhotoRef(clientRef, transferID))), nil
}

// transferPhotoRef is the ref "record_photos_changed" names a transfer by:
// its clientRef when this till sent it, else "transfer-<id>".
func transferPhotoRef(clientRef string, transferID int64) string {
	if clientRef != "" {
		return clientRef
	}
	return fmt.Sprintf("transfer-%d", transferID)
}
