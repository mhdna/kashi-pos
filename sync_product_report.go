package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	db "kashi-pos/db/sqlc"
)

// productReportItem mirrors branchProductReportItem in kashi's
// api/branch_product_report.go -- one flat POS product row kashi has no
// variant for, sent up so an admin can adopt it into the catalog from the
// conflict screen.
type productReportItem struct {
	Barcode      string `json:"barcode"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Price        int64  `json:"price"`
	Color        string `json:"color"`
	Size         string `json:"size"`
	Brand        string `json:"brand"`
	Kind         string `json:"kind"`
	Season       string `json:"season"`
	Year         string `json:"year"`
	CurrencyCode string `json:"currency_code"`
}

type productReportRequest struct {
	Products []productReportItem `json:"products"`
}

// ReportProducts sends a batch of unresolved local products to kashi's
// /branch/products/report, which stages one 'unknown_product' conflict per
// new barcode. Returns how many kashi says it staged.
func (c *centralClient) ReportProducts(ctx context.Context, items []productReportItem) (int, error) {
	if !c.enabled() {
		return 0, errCentralSyncDisabled
	}
	body, err := json.Marshal(productReportRequest{Products: items})
	if err != nil {
		return 0, err
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/branch/products/report", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, responseError(http.MethodPost, "/branch/products/report", resp)
	}
	var out struct {
		Staged int `json:"staged"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, fmt.Errorf("decode product report response: %w", err)
	}
	return out.Staged, nil
}

// resyncProducts reconciles every local product barcode against kashi and
// reports the unknown ones for admin review. Products are intentionally
// one-way (kashi -> till) for the catalog itself; this only surfaces gaps,
// it never creates a central product directly.
func (a *App) resyncProducts(ctx context.Context, client *centralClient, opts ResyncOptions) (ResyncEntityReport, error) {
	rep := ResyncEntityReport{Entity: "products"}

	products, err := a.store.ListProducts(ctx)
	if err != nil {
		return rep, err
	}
	byBarcode := make(map[string]db.Product, len(products))
	refs := make([]string, 0, len(products))
	for _, p := range products {
		if p.Barcode == "" {
			continue
		}
		byBarcode[p.Barcode] = p
		refs = append(refs, p.Barcode)
	}
	rep.Local = len(refs)

	missing, err := a.reconcileEntity(ctx, client, "products", refs)
	if err != nil {
		return rep, err
	}
	rep.Missing = len(missing)
	rep.Central = rep.Local - rep.Missing
	a.emitResyncProgress("products", rep.Central, rep.Local)

	if opts.DryRun || len(missing) == 0 {
		return rep, nil
	}

	const batch = 200
	for start := 0; start < len(missing); start += batch {
		end := start + batch
		if end > len(missing) {
			end = len(missing)
		}
		items := make([]productReportItem, 0, end-start)
		for _, barcode := range missing[start:end] {
			p := byBarcode[barcode]
			items = append(items, productReportItem{
				Barcode:      p.Barcode,
				Code:         p.Code,
				Name:         p.Name,
				Description:  p.Description,
				Price:        p.Price,
				Color:        p.Color,
				Size:         p.Size,
				Brand:        p.Brand,
				Kind:         p.Kind,
				Season:       p.Season,
				Year:         strconv.FormatInt(p.ProductYear, 10),
				CurrencyCode: p.CurrencyCode,
			})
		}
		staged, err := client.ReportProducts(ctx, items)
		if err != nil {
			return rep, err
		}
		rep.Queued += staged
	}
	return rep, nil
}
