package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// The live cart: kashi can see the sale open on this till and change it.
//
// The screen reports its open sale (ReportLiveCart) on every change and every
// 20 s while it has one; the till forwards it to kashi's /branch/live_cart.
// It's a view, not a record: nothing is queued, and a report that fails is
// simply replaced by the next one.
//
// Someone in kashi can then send a cart_override command for that sale: turn
// every deal off, or give a line a price, a discount or no deal. The till keeps
// the overrides here and priceInvoiceTotals applies them itself, past the
// branch's custom price/discount and deal/discount toggle settings: the point
// is that kashi can do what the till's settings don't let a salesperson do.
// Overrides belong to one sale (its saleRef); a report of another sale drops
// them, so they can never leak into the next customer's.

const (
	liveCartReportTimeout = 8 * time.Second
	// Most a price override can be; the same cap as a custom price.
	liveCartMaxPriceCents = 99999999
)

// liveCartLineOverride is what kashi set on one line, keyed by barcode.
type liveCartLineOverride struct {
	Barcode string `json:"barcode"`
	// The unit price in cents; nil keeps the list price.
	PriceCents *int64 `json:"price_cents,omitempty"`
	// The discount percent; nil keeps the list discount.
	Discount *int64 `json:"discount,omitempty"`
	// The line counts towards no deal.
	NoDeal bool `json:"no_deal,omitempty"`
}

// liveCartOverrides is the whole set kashi wants on one sale. A command
// replaces it, so sending it twice is harmless.
type liveCartOverrides struct {
	SaleRef     string                 `json:"sale_ref"`
	AllDealsOff bool                   `json:"all_deals_off"`
	Lines       []liveCartLineOverride `json:"lines"`
	// Who in kashi set it, for the screen.
	By string `json:"by,omitempty"`
}

func (o liveCartOverrides) empty() bool {
	return !o.AllDealsOff && len(o.Lines) == 0
}

func (o liveCartOverrides) line(barcode string) (liveCartLineOverride, bool) {
	barcode = strings.TrimSpace(barcode)
	if barcode == "" {
		return liveCartLineOverride{}, false
	}
	for _, l := range o.Lines {
		if l.Barcode == barcode {
			return l, true
		}
	}
	return liveCartLineOverride{}, false
}

// liveCartState is the sale the screen last reported and kashi's overrides
// for it.
type liveCartState struct {
	mu sync.Mutex
	// The sale on screen ("" when the cart is empty) and its mode: only a
	// plain sale takes overrides.
	saleRef   string
	mode      string
	overrides liveCartOverrides
	// Reports go up one at a time; a newer one waiting replaces an older one.
	sending bool
	next    []byte
}

// activeOverrides is what pricing applies now: kashi's overrides when the
// screen is on that sale and selling, nothing otherwise.
func (s *liveCartState) activeOverrides() liveCartOverrides {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mode != "sale" || s.saleRef == "" || s.overrides.SaleRef != s.saleRef {
		return liveCartOverrides{}
	}
	return s.overrides
}

// liveCartReport is what the screen sends; kashi stores it as given, so only
// the fields the till needs itself are decoded.
type liveCartReport struct {
	SaleRef string          `json:"saleRef"`
	Mode    string          `json:"mode"`
	Lines   json.RawMessage `json:"lines"`
}

// ReportLiveCart takes the screen's open sale and sends it to kashi. A report
// of a different sale drops the overrides of the last one. It never fails
// the screen: no kashi, or kashi unreachable, is just nothing sent.
func (a *App) ReportLiveCart(reportJSON string) error {
	var report liveCartReport
	if err := json.Unmarshal([]byte(reportJSON), &report); err != nil {
		return fmt.Errorf("live cart report: %w", err)
	}
	s := &a.liveCart
	s.mu.Lock()
	changedSale := s.saleRef != report.SaleRef
	s.saleRef, s.mode = report.SaleRef, report.Mode
	dropped := false
	if s.overrides.SaleRef != "" && s.overrides.SaleRef != report.SaleRef {
		s.overrides = liveCartOverrides{}
		dropped = true
	}
	// The report carries the overrides in force, so kashi shows what the
	// till really applies.
	var body map[string]any
	if err := json.Unmarshal([]byte(reportJSON), &body); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("live cart report: %w", err)
	}
	if s.mode == "sale" && s.overrides.SaleRef == s.saleRef && !s.overrides.empty() {
		body["overrides"] = s.overrides
	}
	payload, err := json.Marshal(body)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	s.next = payload
	start := !s.sending
	s.sending = true
	s.mu.Unlock()

	if dropped && changedSale {
		a.emitLiveCartChanged()
	}
	if start {
		go a.sendLiveCartReports()
	}
	return nil
}

// sendLiveCartReports sends the waiting report, and any that arrive meanwhile,
// one at a time.
func (a *App) sendLiveCartReports() {
	s := &a.liveCart
	for {
		s.mu.Lock()
		payload := s.next
		s.next = nil
		if payload == nil {
			s.sending = false
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()

		client := a.photoClient()
		if client == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), liveCartReportTimeout)
		err := client.sendJSON(ctx, http.MethodPost, "/branch/live_cart", payload)
		cancel()
		// An older kashi has no live cart: it answers 404, which isn't worth
		// a log line on every scan.
		var perm *permanentError
		if err != nil && !errors.Is(err, errCentralSyncDisabled) &&
			!(errors.As(err, &perm) && perm.status == http.StatusNotFound) {
			log.Printf("live cart report: %v", err)
		}
	}
}

// GetLiveCartOverrides is what kashi set on the sale on screen, for the
// cart to show and lock (empty when nothing).
func (a *App) GetLiveCartOverrides() liveCartOverrides {
	o := a.liveCart.activeOverrides()
	if o.Lines == nil {
		o.Lines = []liveCartLineOverride{}
	}
	return o
}

func (a *App) emitLiveCartChanged() {
	a.emitAttendanceEvent("live_cart_overrides_changed")
}

// executeCartOverrideCommand applies kashi's overrides to the sale on screen.
// They only fit the sale they were made for: once the screen has moved on,
// the command fails and kashi says so.
func (a *App) executeCartOverrideCommand(payloadJSON json.RawMessage) error {
	var o liveCartOverrides
	if err := json.Unmarshal(payloadJSON, &o); err != nil {
		return fmt.Errorf("cart_override: %w", err)
	}
	if o.SaleRef == "" {
		return errors.New("cart_override needs the sale's sale_ref")
	}
	seen := map[string]bool{}
	lines := make([]liveCartLineOverride, 0, len(o.Lines))
	for _, l := range o.Lines {
		l.Barcode = strings.TrimSpace(l.Barcode)
		if l.Barcode == "" || seen[l.Barcode] {
			continue
		}
		if l.PriceCents != nil && (*l.PriceCents <= 0 || *l.PriceCents > liveCartMaxPriceCents) {
			return fmt.Errorf("cart_override: price for %s must be above 0", l.Barcode)
		}
		if l.Discount != nil && (*l.Discount < 0 || *l.Discount > 100) {
			return fmt.Errorf("cart_override: discount for %s must be 0 to 100", l.Barcode)
		}
		if l.PriceCents == nil && l.Discount == nil && !l.NoDeal {
			continue
		}
		seen[l.Barcode] = true
		lines = append(lines, l)
	}
	o.Lines = lines

	s := &a.liveCart
	s.mu.Lock()
	if s.saleRef != o.SaleRef {
		s.mu.Unlock()
		return errors.New("that sale is no longer open on the till")
	}
	if s.mode != "sale" {
		s.mu.Unlock()
		return fmt.Errorf("the till is in %s mode; only a sale can be changed", s.mode)
	}
	s.overrides = o
	s.mu.Unlock()
	a.emitLiveCartChanged()
	return nil
}

// applyLiveCartOverride prices a line as kashi set it. It runs where
// validateAndPriceItems reads the line's list price and discount, and returns
// whether the line is kashi's: the line's own custom price, per-unit prices
// and toggles are then ignored, whatever the branch's settings.
func applyLiveCartOverride(o liveCartOverrides, item *salesInvoiceItem) (liveCartLineOverride, bool) {
	if item.Qty <= 0 {
		return liveCartLineOverride{}, false
	}
	l, ok := o.line(item.Barcode)
	if !ok {
		return liveCartLineOverride{}, false
	}
	item.CustomPriceCents = nil
	item.UnitPricesCents = nil
	item.UnitDiscounts = nil
	item.NoDiscount = false
	item.NoDeal = false
	return l, true
}
