package license

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// APIError is an answer from the server (as opposed to not reaching it).
type APIError struct {
	Status int
	Code   string
}

func (e *APIError) Error() string { return fmt.Sprintf("license server: %s (%d)", e.Code, e.Status) }

// Codes the server answers with (see kashi-license/server/api.go).
const (
	CodeInvalidKey     = "invalid_key"
	CodeOff            = "off"
	CodeNoSeats        = "no_seats"
	CodeTooMany        = "too_many_attempts"
	CodeUnknownInstall = "unknown_install"
	CodeWrongMachine   = "wrong_machine"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{Timeout: 10 * time.Second}}
}

type ActivateRequest struct {
	ProductKey  string `json:"productKey"`
	Fingerprint string `json:"fingerprint"`
	Hostname    string `json:"hostname"`
	AppVersion  string `json:"appVersion"`
}

type ActivateResponse struct {
	InstallID     string `json:"installId"`
	InstallSecret string `json:"installSecret"`
	DBKey         string `json:"dbKey"`
	Lease         Signed `json:"lease"`
}

type CheckInRequest struct {
	InstallID     string `json:"installId"`
	InstallSecret string `json:"installSecret"`
	Fingerprint   string `json:"fingerprint"`
	AppVersion    string `json:"appVersion"`
}

type checkInResponse struct {
	Lease Signed `json:"lease"`
}

func (c *Client) Activate(ctx context.Context, req ActivateRequest) (ActivateResponse, error) {
	var out ActivateResponse
	err := c.post(ctx, "/v1/activate", req, &out)
	return out, err
}

func (c *Client) CheckIn(ctx context.Context, req CheckInRequest) (Signed, error) {
	var out checkInResponse
	err := c.post(ctx, "/v1/checkin", req, &out)
	return out.Lease, err
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error == "" {
			// A proxy page, a 502 while the server restarts: not an answer.
			return fmt.Errorf("license server: http %d", resp.StatusCode)
		}
		return &APIError{Status: resp.StatusCode, Code: e.Error}
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
