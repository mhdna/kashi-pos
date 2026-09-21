package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// errCentralSyncDisabled means this install has no central_api_url/api_key
// configured. Every centralClient method must check this first — sync is
// opt-in per branch, and an install that hasn't opted in must behave
// exactly as it does today, indefinitely, not just "until it's set up".
var errCentralSyncDisabled = errors.New("central sync is not configured for this branch")

// centralClient talks to the kashi central backend on behalf of this branch,
// authenticating with the branch code/key pair from settings (branchAuthMiddleware
// on the kashi side, not the PASETO user-token flow — see api/middleware.go
// in the kashi repo for why a branch uses a separate, non-expiring credential).
type centralClient struct {
	baseURL    string
	branchCode string
	apiKey     string
	httpClient *http.Client
}

func newCentralClient(baseURL, branchCode, apiKey string) *centralClient {
	return &centralClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		branchCode: branchCode,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *centralClient) enabled() bool {
	return c != nil && c.baseURL != "" && c.branchCode != "" && c.apiKey != ""
}

func (c *centralClient) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Branch-Code", c.branchCode)
	req.Header.Set("X-Branch-Key", c.apiKey)
	return req, nil
}

type centralErrorBody struct {
	Error string `json:"error"`
}

// permanentError marks a central response the branch must not keep retrying:
// a 4xx means kashi understood the request and rejected it (bad payload,
// unknown branch, a validation failure), and replaying the identical body on
// a 15s loop forever will never change that. 408 and 429 are the two 4xx
// codes that *are* transient, so they're deliberately excluded. Network
// errors and 5xx are not permanent — the branch may just be offline or kashi
// mid-deploy — and stay on the normal exponential-backoff retry path.
type permanentError struct {
	status int
	msg    string
}

func (e *permanentError) Error() string { return e.msg }

// isPermanentSyncError reports whether err (from any centralClient push
// method) is a permanentError, so the outbox worker can move the row to
// 'failed' instead of scheduling another attempt.
func isPermanentSyncError(err error) bool {
	var pe *permanentError
	return errors.As(err, &pe)
}

// responseError turns a non-2xx central response into an error, classifying
// 4xx (except 408/429) as permanent.
func responseError(method, path string, resp *http.Response) error {
	var body centralErrorBody
	json.NewDecoder(resp.Body).Decode(&body)
	msg := fmt.Sprintf("%s %s failed (status %d): %s", method, path, resp.StatusCode, body.Error)
	if resp.StatusCode >= 400 && resp.StatusCode < 500 &&
		resp.StatusCode != http.StatusRequestTimeout &&
		resp.StatusCode != http.StatusTooManyRequests {
		return &permanentError{status: resp.StatusCode, msg: msg}
	}
	return errors.New(msg)
}

// Health confirms the configured branch code/key round-trips against kashi's
// /branch/health. It does nothing else — no retry, no side effects — this is
// purely for manually verifying a branch is wired up correctly.
func (c *centralClient) Health(ctx context.Context) error {
	if !c.enabled() {
		return errCentralSyncDisabled
	}

	req, err := c.newRequest(ctx, http.MethodGet, "/branch/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var body centralErrorBody
		json.NewDecoder(resp.Body).Decode(&body)
		return fmt.Errorf("central health check failed (status %d): %s", resp.StatusCode, body.Error)
	}

	return nil
}

// PostInvoice sends a raw, already-serialized branchInvoicePayload JSON body
// to kashi. It's deliberately just []byte in, error out — the sync worker
// stores the exact bytes it needs to send (sync_outbox.payload_json) and
// has no reason to unmarshal/remarshal them, and the caller doesn't need
// the response body since kashi's idempotency handling means any 2xx
// (fresh create or "already have this one") is equally a success.
func (c *centralClient) PostInvoice(ctx context.Context, path string, payloadJSON []byte) error {
	return c.sendJSON(ctx, http.MethodPost, path, payloadJSON)
}

// postExpenseResponse is kashi's response body to POST /branch/expenses --
// see createBranchExpense's envelope in the kashi repo's api/branch_expense.go.
// UploadToken is empty on the idempotent "already exists" path (kashi only
// mints one on a fresh insert), which the caller treats as "no QR dialog
// this time" rather than an error.
type postExpenseResponse struct {
	UploadToken string `json:"upload_token"`
}

// PostExpense is PostInvoice's counterpart for expenses specifically: unlike
// every other synced entity, an expense's response body carries a one-time
// receipt-upload token the caller needs, so this can't reuse PostInvoice's
// discard-the-body shape.
func (c *centralClient) PostExpense(ctx context.Context, path string, payloadJSON []byte) (*postExpenseResponse, error) {
	if !c.enabled() {
		return nil, errCentralSyncDisabled
	}

	req, err := c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(payloadJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, responseError(http.MethodPost, path, resp)
	}

	var out postExpenseResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode expense response: %w", err)
	}
	return &out, nil
}

// branchClientsBatchPayload is the body of POST /branch/clients_batch: the
// same per-client objects as the single /branch/clients route, wrapped.
type branchClientsBatchPayload struct {
	Clients []branchClientPayload `json:"clients"`
}

// branchClientBatchResult is kashi's verdict on one client of a batch (see
// branchClientBatchResult in kashi's api/branch_client_batch.go). Status is
// "ok" (stored/linked, possibly with a staged conflict), "invalid" (kashi
// will never accept these bytes) or "error" (server-side failure, retry).
type branchClientBatchResult struct {
	BranchClientID int64  `json:"branch_client_id"`
	Status         string `json:"status"`
	ConflictKind   string `json:"conflict_kind"`
	Error          string `json:"error"`
}

// clientsBatchTimeout is longer than the shared 10s client timeout: kashi
// resolves each client in the batch with a few queries, so a full chunk
// legitimately takes longer than a single-row POST.
const clientsBatchTimeout = 60 * time.Second

// PostClientsBatch sends many clients in one round trip and returns kashi's
// per-client results. A non-nil error means the whole call failed (network,
// 5xx, or a 4xx such as 404 from a kashi that predates the route) and no
// client was necessarily stored; the caller decides whether to fall back to
// per-client POSTs. The batch is idempotent per client, so replaying it after
// an ambiguous failure is safe.
func (c *centralClient) PostClientsBatch(ctx context.Context, payload branchClientsBatchPayload) ([]branchClientBatchResult, error) {
	if !c.enabled() {
		return nil, errCentralSyncDisabled
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	const path = "/branch/clients_batch"
	req, err := c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := *c.httpClient
	httpClient.Timeout = clientsBatchTimeout
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, responseError(http.MethodPost, path, resp)
	}

	var out struct {
		Results []branchClientBatchResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode clients batch response: %w", err)
	}
	return out.Results, nil
}

// postShiftCloseResponse is kashi's body for POST /branch/shift_closes.
// UploadToken is empty on the idempotent "already exists" path; the caller
// then falls back to its own local upload server.
type postShiftCloseResponse struct {
	UploadToken string `json:"upload_token"`
}

// PostShiftClose sends the shift-close payload synchronously (unlike the
// normal outbox drain, which fires and forgets) so the hybrid close path
// can read back the one-time upload token and point its QR straight at
// kashi. The normal outbox row for this close is marked synced by the
// caller afterwards so the worker doesn't re-send it.
func (c *centralClient) PostShiftClose(ctx context.Context, payloadJSON []byte) (*postShiftCloseResponse, error) {
	if !c.enabled() {
		return nil, errCentralSyncDisabled
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/branch/shift_closes", bytes.NewReader(payloadJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, responseError(http.MethodPost, "/branch/shift_closes", resp)
	}

	var out postShiftCloseResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode shift close response: %w", err)
	}
	return &out, nil
}

// ShiftReceiptUploadStatus polls kashi for whether the close-shift receipt
// photo has landed -- the "central" mode counterpart to watching the local
// shift_receipt_uploads row, used by the QR dialog while it waits.
func (c *centralClient) ShiftReceiptUploadStatus(ctx context.Context, shiftRef string) (bool, error) {
	if !c.enabled() {
		return false, errCentralSyncDisabled
	}

	req, err := c.newRequest(ctx, http.MethodGet, "/branch/shift_closes/upload_status?shift_ref="+url.QueryEscape(shiftRef), nil)
	if err != nil {
		return false, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var body centralErrorBody
		json.NewDecoder(resp.Body).Decode(&body)
		return false, fmt.Errorf("shift upload status check failed (status %d): %s", resp.StatusCode, body.Error)
	}

	var out struct {
		ImagesUploaded bool `json:"images_uploaded"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, fmt.Errorf("decode shift upload status response: %w", err)
	}
	return out.ImagesUploaded, nil
}

// ShiftUploadURL builds the central QR link a phone scans in "central" mode.
func (c *centralClient) ShiftUploadURL(token string) string {
	return c.baseURL + "/shift_uploads/" + token
}

// ExpenseUploadStatus polls kashi for whether receipt images have landed on
// a given expense yet -- the poll-based companion to the
// "expense_images_uploaded" WS push, used by kashi-pos's QR dialog while
// it's waiting.
func (c *centralClient) ExpenseUploadStatus(ctx context.Context, clientRef string) (bool, error) {
	if !c.enabled() {
		return false, errCentralSyncDisabled
	}

	req, err := c.newRequest(ctx, http.MethodGet, "/branch/expenses/upload_status?client_ref="+url.QueryEscape(clientRef), nil)
	if err != nil {
		return false, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var body centralErrorBody
		json.NewDecoder(resp.Body).Decode(&body)
		return false, fmt.Errorf("upload status check failed (status %d): %s", resp.StatusCode, body.Error)
	}

	var out struct {
		ImagesUploaded bool `json:"images_uploaded"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, fmt.Errorf("decode upload status response: %w", err)
	}
	return out.ImagesUploaded, nil
}

// UploadURL builds the full one-time receipt-upload link a phone scans via
// QR code, from this branch's own configured central_api_url -- kashi never
// needs to know its own public address, kashi-pos already does (it's the
// same host this client talks to for everything else).
func (c *centralClient) UploadURL(token string) string {
	return c.baseURL + "/expense_uploads/" + token
}

// errShiftNotReceivedYet is kashi's 409 on POST /branch/shift_receipt_images
// when the branch_shifts row the photo attaches to hasn't synced yet. It's
// retryable (the shift-close outbox entry just hasn't drained), so the
// uploader treats it as a plain transient error, not a permanent one.
var errShiftNotReceivedYet = errors.New("kashi has not received this shift close yet")

// PostShiftReceiptImage uploads the single close-shift receipt photo to
// kashi, keyed by shift_ref (shift-<localID>) for idempotency -- kashi
// no-ops a second upload for a shift that already has one. Multipart rather
// than JSON because the body is an image file; this is the shift-side
// counterpart to a phone hitting kashi's public /expense_uploads endpoint.
func (c *centralClient) PostShiftReceiptImage(ctx context.Context, shiftRef, filePath, contentType string) error {
	if !c.enabled() {
		return errCentralSyncDisabled
	}

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("shift_ref", shiftRef); err != nil {
		return err
	}
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="%s"`, filepath.Base(filePath)))
	if contentType != "" {
		partHeader.Set("Content-Type", contentType)
	}
	part, err := mw.CreatePart(partHeader)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/branch/shift_receipt_images", &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return errShiftNotReceivedYet
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError(http.MethodPost, "/branch/shift_receipt_images", resp)
	}
	return nil
}

// PutSettings pushes this branch's current business-identity settings
// snapshot to kashi. It's a plain upsert on kashi's side (see
// putBranchSettings in api/branch_settings.go), so pushing the same
// snapshot repeatedly is harmless -- unlike invoices, settings has no
// client_ref/idempotency concept because there's nothing to deduplicate.
func (c *centralClient) PutSettings(ctx context.Context, payloadJSON []byte) error {
	return c.sendJSON(ctx, http.MethodPut, "/branch/settings", payloadJSON)
}

func (c *centralClient) sendJSON(ctx context.Context, method, path string, payloadJSON []byte) error {
	if !c.enabled() {
		return errCentralSyncDisabled
	}

	req, err := c.newRequest(ctx, method, path, bytes.NewReader(payloadJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError(method, path, resp)
	}

	return nil
}

type branchSyncChangesResponse struct {
	Entity     string          `json:"entity"`
	Items      json.RawMessage `json:"items"`
	ServerTime time.Time       `json:"server_time"`
	// NextCursor is non-empty only when kashi truncated the response to
	// `limit` rows and there are more; the branch feeds it back as `cursor`
	// to fetch the next page (see api/branch_catchup.go). Empty for every
	// non-paged entity and for the last page of a paged one.
	NextCursor string `json:"next_cursor"`
}

// fetchChangesParams is the argument bag for FetchChanges. Since is the
// entity's stored watermark (empty = "from the beginning"); Cursor, when
// set, is a next_cursor from a previous page and supersedes Since; Limit
// caps a paged entity's rows (ignored by kashi for the small entities).
type fetchChangesParams struct {
	Entity string
	Since  string
	Cursor string
	Limit  int
}

// FetchChanges calls kashi's generalized catch-up endpoint for one entity.
// since may be empty, meaning "everything" (a fresh install or one just
// turning sync on) — that's not a special case on either side, just an
// empty `since` query param. Items is left as raw JSON since each entity
// unmarshals into a different local shape; the caller decodes it.
func (c *centralClient) FetchChanges(ctx context.Context, p fetchChangesParams) (*branchSyncChangesResponse, error) {
	if !c.enabled() {
		return nil, errCentralSyncDisabled
	}

	path := "/branch/sync/changes?entity=" + url.QueryEscape(p.Entity)
	if p.Since != "" {
		path += "&since=" + url.QueryEscape(p.Since)
	}
	if p.Cursor != "" {
		path += "&cursor=" + url.QueryEscape(p.Cursor)
	}
	if p.Limit > 0 {
		path += "&limit=" + strconv.Itoa(p.Limit)
	}

	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var body centralErrorBody
		json.NewDecoder(resp.Body).Decode(&body)
		return nil, fmt.Errorf("fetch changes for %s failed (status %d): %s", p.Entity, resp.StatusCode, body.Error)
	}

	var out branchSyncChangesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode changes response: %w", err)
	}
	return &out, nil
}

type branchCommand struct {
	ID      int64           `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type branchPendingCommandsResponse struct {
	Commands []branchCommand `json:"commands"`
}

// FetchPendingCommands polls kashi for commands issued to this branch that
// haven't been acked yet. Called every sync tick regardless of whether a
// WebSocket "command" push was received (see sync_ws.go) -- the same
// dual-path guarantee every other pushed entity gets, so a branch that was
// offline when a command was issued still picks it up once it reconnects.
func (c *centralClient) FetchPendingCommands(ctx context.Context) ([]branchCommand, error) {
	if !c.enabled() {
		return nil, errCentralSyncDisabled
	}

	req, err := c.newRequest(ctx, http.MethodGet, "/branch/commands/pending", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var body centralErrorBody
		json.NewDecoder(resp.Body).Decode(&body)
		return nil, fmt.Errorf("fetch pending commands failed (status %d): %s", resp.StatusCode, body.Error)
	}

	var out branchPendingCommandsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode pending commands response: %w", err)
	}
	return out.Commands, nil
}

// AckCommand reports the outcome of executing one command back to kashi --
// the audit trail's completion record. status is "success" or "failed";
// execErr, when non-nil, becomes the stored error text.
func (c *centralClient) AckCommand(ctx context.Context, commandID int64, status string, execErr error) error {
	body := struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}{Status: status}
	if execErr != nil {
		body.Error = execErr.Error()
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal command ack: %w", err)
	}
	return c.sendJSON(ctx, http.MethodPost, fmt.Sprintf("/branch/commands/%d/ack", commandID), payload)
}

// DialWS connects to kashi's push channel. kashi-pos is a Go client
// dialing directly (not a browser page), so — unlike a JS WebSocket — it
// can set the same X-Branch-Code/X-Branch-Key headers used everywhere else
// on the initial handshake request, no separate auth scheme needed for
// this one connection type.
func (c *centralClient) DialWS(ctx context.Context) (*websocket.Conn, error) {
	if !c.enabled() {
		return nil, errCentralSyncDisabled
	}

	wsURL := strings.Replace(c.baseURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)

	header := http.Header{}
	header.Set("X-Branch-Code", c.branchCode)
	header.Set("X-Branch-Key", c.apiKey)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL+"/branch/ws", header)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
