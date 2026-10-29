package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	db "kashi-pos/db/sqlc"
)

// "Unknown item": something a customer wants that has no code here. The
// cashier rings it up as the placeholder product below at the price they
// type; before payment, a phone takes a picture of every line, whatever its
// qty (the QR link kashi hands back), the pictures go straight to kashi, and
// the sale is held only until each unit has one -- or a kashi user with the
// right permission lets it through. Afterwards someone in kashi names what
// each picture was (see api/unknown_items.go in the kashi repo).

const (
	// defaultUnknownBarcode is the built-in placeholder's barcode, used while
	// the branch hasn't named a product of its own (settings.unknown_product_barcode).
	defaultUnknownBarcode = "UNKNOWN"
	unknownProductName    = "Unknown item"

	unknownCallTimeout = 10 * time.Second

	// When on, the phone's QR code only shows once the cashier pays (the
	// original flow). Off, the default, the sale screen shows it as soon as
	// an unknown item is in the cart, so pictures are taken while ringing up.
	unknownPicturesAtCheckoutFlag = "unknown_pictures_at_checkout"
)

// SetUnknownPicturesAtCheckout picks when the unknown item QR code shows: at
// checkout (true) or right after an unknown item is added (false).
func (a *App) SetUnknownPicturesAtCheckout(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := a.setFeatureFlagBool(a.ctx, unknownPicturesAtCheckoutFlag, enabled); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

// unknownBarcode is the barcode of the product that stands for "an item I
// can't name": the one kashi's branch settings point at, else the built-in
// placeholder.
func (a *App) unknownBarcode() string {
	if a.store != nil {
		if settings, err := a.store.GetSettings(a.ctx); err == nil {
			if barcode := strings.TrimSpace(settings.UnknownProductBarcode); barcode != "" {
				return barcode
			}
		}
	}
	return defaultUnknownBarcode
}

// unknownCode is the code of that same product. The sale screen's cart lines
// carry a code but no barcode, so a cart is matched on this one.
func (a *App) unknownCode() string {
	barcode := a.unknownBarcode()
	if a.store != nil {
		if product, err := a.store.GetProductByBarcode(a.ctx, barcode); err == nil && product.Code != "" {
			return product.Code
		}
	}
	return barcode
}

// SetUnknownProductBarcode names the product the "Unknown item" button adds
// ("" goes back to the built-in placeholder). Called by an update_settings
// command from kashi.
func (a *App) SetUnknownProductBarcode(barcode string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := a.store.SetUnknownProductBarcode(a.ctx, strings.TrimSpace(barcode)); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

// ensureUnknownProduct returns the unknown product. A product kashi named must
// already be in this till's catalog; the built-in placeholder is created on
// first use. Either way it is priced by what the cashier types (see
// validateAndPriceItems), and kashi never moves stock for it.
func ensureUnknownProduct(ctx context.Context, q *db.Queries, barcode string) (db.Product, error) {
	product, err := q.GetProductByBarcode(ctx, barcode)
	if err == nil {
		// Earlier versions let the full catalog check switch the built-in
		// placeholder off as a product kashi doesn't have; turn it back on.
		if product.IsActive == 0 && barcode == defaultUnknownBarcode {
			if err := q.SetProductActive(ctx, db.SetProductActiveParams{IsActive: 1, ID: product.ID}); err != nil {
				return db.Product{}, err
			}
			product.IsActive = 1
		}
		return product, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return db.Product{}, err
	}
	if barcode != defaultUnknownBarcode {
		return db.Product{}, fmt.Errorf("the unknown product %q isn't in this till's catalog yet", barcode)
	}
	return q.CreateProduct(ctx, db.CreateProductParams{
		Barcode: barcode, Code: barcode, Name: unknownProductName,
		IsActive: 1, Price: 0, CurrencyCode: "USD",
	})
}

// GetUnknownProduct is the sale screen's "Unknown item" button: the
// placeholder as a cart line, still without a price.
func (a *App) GetUnknownProduct() (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	product, err := ensureUnknownProduct(a.ctx, a.store.Queries, a.unknownBarcode())
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id": product.ID, "name": product.Name, "code": product.Code, "barcode": product.Barcode,
		"category": "", "brand": "", "color": "", "size": "", "price": 0, "discount": 0, "qty": 1,
	}, nil
}

// unknownUnitCount is how many units of the placeholder a cart sells. A
// negative quantity is an exchange's returned line, not a sale: it needs no
// picture.
func unknownUnitCount(items []salesInvoiceItem, barcode, code string) int64 {
	var units int64
	for _, item := range items {
		if item.Qty <= 0 {
			continue
		}
		if item.Barcode == barcode || (item.Barcode == "" && item.Code == code) {
			units += int64(item.Qty)
		}
	}
	return units
}

// unknownCall is one JSON round trip to kashi's /branch API for the unknown
// item pictures.
func (a *App) unknownCall(method, path string, in, out any) error {
	client := a.photoClient()
	if client == nil {
		return fmt.Errorf("this till isn't connected to kashi")
	}
	var body bytes.Buffer
	if in != nil {
		if err := json.NewEncoder(&body).Encode(in); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(a.ctx, unknownCallTimeout)
	defer cancel()
	req, err := client.newRequest(ctx, method, path, &body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("couldn't reach kashi: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e centralErrorBody
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error == "" {
			e.Error = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return fmt.Errorf("%s", e.Error)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type unknownSlotInput struct {
	Label          string `json:"label"`
	UnitPriceCents int64  `json:"unitPriceCents"`
	Discount       int64  `json:"discount"`
	// The cart line the unit is on; kashi takes one picture per line.
	LineRef string `json:"lineRef"`
	// Stable across re-sends of the same cart, so a unit keeps its picture
	// as lines around it come and go. Empty: its position in the cart.
	SlotRef string `json:"slotRef"`
}

type unknownSessionResponse struct {
	UploadToken string `json:"upload_token"`
	Complete    bool   `json:"complete"`
	Slots       []struct {
		SlotRef  string `json:"slot_ref"`
		LineRef  string `json:"line_ref"`
		Label    string `json:"label"`
		HasImage bool   `json:"has_image"`
		Waived   bool   `json:"waived"`
	} `json:"slots"`
}

func doneUnknownUnits(resp unknownSessionResponse) int {
	done := 0
	for _, slot := range resp.Slots {
		if slot.HasImage || slot.Waived {
			done++
		}
	}
	return done
}

func unknownSessionResult(clientRef, uploadURL string, resp unknownSessionResponse) map[string]any {
	slots := make([]map[string]any, 0, len(resp.Slots))
	for _, slot := range resp.Slots {
		slots = append(slots, map[string]any{
			"slotRef": slot.SlotRef, "lineRef": slot.LineRef, "label": slot.Label,
			"hasImage": slot.HasImage, "waived": slot.Waived, "done": slot.HasImage || slot.Waived,
		})
	}
	return map[string]any{"clientRef": clientRef, "uploadUrl": uploadURL, "slots": slots, "complete": resp.Complete}
}

// StartUnknownItemSession opens the picture session of the cart being paid:
// one slot per unit, one picture per cart line. It returns the link the phone's QR code carries. The
// session is remembered so CheckoutInvoice can tie the sale to it.
func (a *App) StartUnknownItemSession(slotsJSON string) (map[string]any, error) {
	return a.SyncUnknownItemSession("", slotsJSON)
}

// SyncUnknownItemSession is StartUnknownItemSession for a cart still being
// rung up: given the clientRef of an open session it re-sends that session's
// units, so the same QR link (and the phone already on it) follows the cart
// as unknown items are added or taken off. "" opens a new session.
func (a *App) SyncUnknownItemSession(clientRef, slotsJSON string) (map[string]any, error) {
	var slots []unknownSlotInput
	if err := json.Unmarshal([]byte(slotsJSON), &slots); err != nil || len(slots) == 0 {
		return nil, fmt.Errorf("no unknown items to take pictures of")
	}
	type slotBody struct {
		SlotRef   string `json:"slot_ref"`
		Label     string `json:"label"`
		UnitPrice int64  `json:"unit_price"`
		Discount  int64  `json:"discount"`
		LineRef   string `json:"line_ref"`
	}
	body := struct {
		ClientRef string     `json:"client_ref"`
		Slots     []slotBody `json:"slots"`
	}{ClientRef: clientRef}
	if body.ClientRef == "" {
		body.ClientRef = uuid.NewString()
	}
	for i, slot := range slots {
		slotRef := slot.SlotRef
		if slotRef == "" {
			slotRef = strconv.Itoa(i + 1)
		}
		body.Slots = append(body.Slots, slotBody{
			SlotRef: slotRef, Label: slot.Label, UnitPrice: slot.UnitPriceCents, Discount: slot.Discount,
			LineRef: slot.LineRef,
		})
	}
	var resp unknownSessionResponse
	if err := a.unknownCall(http.MethodPost, "/branch/unknown_sessions", body, &resp); err != nil {
		return nil, err
	}
	if resp.UploadToken == "" {
		return nil, fmt.Errorf("kashi didn't send a link")
	}
	client := a.photoClient()
	a.unknownMu.Lock()
	a.unknownSessionRef = body.ClientRef
	a.unknownDone = doneUnknownUnits(resp)
	a.unknownMu.Unlock()
	return unknownSessionResult(body.ClientRef, client.baseURL+"/unknown_uploads/"+resp.UploadToken, resp), nil
}

// UnknownItemSessionStatus is the dialog's poll: which units have a picture or
// a manager's pass. Kashi holds the call until that changes (a few seconds at
// most), so a picture shows up the moment it lands.
func (a *App) UnknownItemSessionStatus(clientRef string) (map[string]any, error) {
	var resp unknownSessionResponse
	a.unknownMu.Lock()
	known := a.unknownDone
	a.unknownMu.Unlock()
	path := "/branch/unknown_sessions/status?client_ref=" + url.QueryEscape(clientRef) + "&known=" + strconv.Itoa(known)
	if err := a.unknownCall(http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	a.unknownMu.Lock()
	a.unknownDone = doneUnknownUnits(resp)
	a.unknownMu.Unlock()
	return unknownSessionResult(clientRef, "", resp), nil
}

// RequestUnknownItemHelp is the cashier's "ask a manager" button: the session
// jumps to the top of kashi's Waiting list.
func (a *App) RequestUnknownItemHelp(clientRef string) error {
	return a.unknownCall(http.MethodPost, "/branch/unknown_sessions/help", map[string]string{"client_ref": clientRef}, nil)
}

// verifyUnknownPictures is checkout's gate: a cart with unknown items is only
// sold once the remembered session has a picture or a pass for every unit.
// It returns the session's ref for the sync payload ("" for an ordinary cart).
func (a *App) verifyUnknownPictures(itemsJSON string) (string, error) {
	var items []salesInvoiceItem
	if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
		return "", nil // the sale's own validation reports a bad cart
	}
	units := unknownUnitCount(items, a.unknownBarcode(), a.unknownCode())
	if units == 0 {
		return "", nil
	}
	a.unknownMu.Lock()
	ref := a.unknownSessionRef
	a.unknownMu.Unlock()
	if ref == "" {
		return "", fmt.Errorf("take pictures of the unknown items before paying")
	}
	var resp unknownSessionResponse
	if err := a.unknownCall(http.MethodGet, "/branch/unknown_sessions/status?client_ref="+url.QueryEscape(ref), nil, &resp); err != nil {
		return "", fmt.Errorf("unknown items need kashi to take their pictures: %w", err)
	}
	if !resp.Complete || int64(len(resp.Slots)) != units {
		return "", fmt.Errorf("pictures of the unknown items are missing")
	}
	return ref, nil
}

// clearUnknownSession forgets the session once its sale is made.
func (a *App) clearUnknownSession(ref string) {
	if ref == "" {
		return
	}
	a.unknownMu.Lock()
	if a.unknownSessionRef == ref {
		a.unknownSessionRef = ""
	}
	a.unknownMu.Unlock()
}
