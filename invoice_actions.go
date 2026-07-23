package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/go-pdf/fpdf"

	db "kashi-pos/db/sqlc"
)

type invoiceItem struct {
	Name     string  `json:"name"`
	Code     string  `json:"code"`
	Category string  `json:"category"`
	Price    float64 `json:"price"`
	Discount float64 `json:"discount"`
	Qty      float64 `json:"qty"`
}

func (a *App) HoldInvoice(itemsJSON string) (int64, error) {
	if a.queries == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	var items []struct {
		Code string  `json:"code"`
		Qty  float64 `json:"qty"`
	}
	if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
		return 0, err
	}
	inv, err := a.queries.CreateHoldInvoice(a.ctx)
	if err != nil {
		return 0, err
	}
	for _, it := range items {
		product, err := a.queries.GetProductByCode(a.ctx, it.Code)
		if err != nil {
			continue
		}
		a.queries.AddHoldInvoiceItem(a.ctx, db.AddHoldInvoiceItemParams{
			HoldInvoiceID: inv.ID,
			ProductID:     product.ID,
			Qty:           int64(it.Qty),
		})
	}
	return inv.ID, nil
}

func (a *App) ListHoldInvoices() ([]map[string]any, error) {
	if a.queries == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	list, err := a.queries.ListHoldInvoices(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(list))
	for i, inv := range list {
		items, _ := a.queries.ListHoldInvoiceItems(a.ctx, inv.ID)
		itemCount := len(items)
		var itemList []map[string]any
		for _, it := range items {
			itemList = append(itemList, map[string]any{
				"name":     it.Name,
				"code":     it.Code,
				"price":    float64(it.Price) / 100,
				"discount": it.Discount,
				"qty":      it.Qty,
				"category": it.Category,
			})
		}
		result[i] = map[string]any{
			"id":        inv.ID,
			"itemCount": itemCount,
			"items":     itemList,
			"createdAt": inv.CreatedAt,
		}
	}
	return result, nil
}

func (a *App) DeleteHoldInvoice(id int64) error {
	if a.queries == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.queries.DeleteHoldInvoice(a.ctx, id)
}

func (a *App) VoidInvoice(itemsJSON string, reason string) error {
	if a.queries == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := a.queries.CreateVoidInvoice(a.ctx, db.CreateVoidInvoiceParams{
		ItemsJson: itemsJSON,
		Reason:    reason,
	})
	return err
}

func (a *App) CheckoutInvoice(itemsJSON string, storeName, branch, phone, salesperson, receiptNo, receiptPrinter string) (string, error) {
	var items []invoiceItem
	if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", fmt.Errorf("no items")
	}

	pdfPath, err := a.generateReceipt(items, storeName, branch, phone, salesperson, receiptNo)
	if err != nil {
		return "", err
	}
	return pdfPath, nil
}

func (a *App) generateReceipt(items []invoiceItem, storeName, branch, phone, salesperson, receiptNo string) (string, error) {
	const (
		pageW    = 80.0
		margin   = 4.0
		pageH    = 260.0
		contentW = pageW - 2*margin
	)

	pdf := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "mm",
		SizeStr:        "",
		Size:           fpdf.SizeType{Wd: pageW, Ht: pageH},
	})
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()

	const fontFamily = "Courier"

	boldText := func(s string, size float64, align string) {
		pdf.SetFont(fontFamily, "B", size)
		w := contentW
		if align == "C" {
			pdf.SetXY(margin, pdf.GetY())
			w = contentW
		}
		pdf.CellFormat(w, 4, s, "", 0, align, false, 0, "")
	}

	twoCol := func(left, right string, size float64) {
		pdf.SetFont(fontFamily, "B", size)
		y := pdf.GetY()
		pdf.SetXY(margin, y)
		pdf.CellFormat(contentW/2, 4, left, "", 0, "L", false, 0, "")
		pdf.SetXY(margin+contentW/2, y)
		pdf.CellFormat(contentW/2, 4, right, "", 0, "R", false, 0, "")
	}

	center := func(s string, size float64) { boldText(s, size, "C") }
	down := func(mm float64) { pdf.SetY(pdf.GetY() + mm) }

	dashedRule := func() {
		pdf.SetDashPattern([]float64{1.1, 1.0}, 0)
		pdf.SetLineWidth(0.35)
		y := pdf.GetY()
		pdf.Line(margin, y, pageW-margin, y)
		pdf.SetDashPattern([]float64{}, 0)
	}

	pdf.SetY(8)
	center(storeName, 21)
	down(8)
	center(branch, 12)
	down(4)
	center(phone, 12)
	down(5)
	dashedRule()
	down(4)

	twoCol("Receipt: "+receiptNo, "", 11)
	down(4)
	now := time.Now()
	twoCol("Date: "+now.Format("02/01/2006"), "Time: "+now.Format("15:04"), 11)
	down(4)
	twoCol(salesperson, "", 11)
	down(5)
	dashedRule()
	down(4)

	twoCol("ITEM", "TOTAL", 11)
	down(3)
	dashedRule()
	down(4)

	var subtotal, totalDiscount float64
	for _, it := range items {
		gross := it.Qty * it.Price
		discAmt := gross * (it.Discount / 100.0)
		lineTotal := gross - discAmt
		subtotal += gross
		totalDiscount += discAmt

		twoCol(it.Name, fmt.Sprintf("$%.2f", lineTotal), 11)
		down(4)

		pdf.SetFont(fontFamily, "", 10)
		qtyLine := fmt.Sprintf("%.0f x $%.2f", it.Qty, it.Price)
		if it.Discount > 0 {
			qtyLine += fmt.Sprintf("  (-%.0f%% = -$%.2f)", it.Discount, discAmt)
		}
		pdf.SetXY(margin+2, pdf.GetY())
		pdf.CellFormat(contentW-2, 4, qtyLine, "", 0, "L", false, 0, "")
		down(5)
	}

	dashedRule()
	down(4)
	taxable := subtotal - totalDiscount
	grandTotal := taxable

	twoCol("Subtotal", fmt.Sprintf("$%.2f", subtotal), 11)
	down(4)
	twoCol("Discount", fmt.Sprintf("-$%.2f", totalDiscount), 11)
	down(5)
	dashedRule()
	down(5)
	twoCol("TOTAL", fmt.Sprintf("$%.2f", grandTotal), 13)
	down(5)
	twoCol("LBP TOTAL", fmt.Sprintf("%.0f L.L.", grandTotal*90000), 11)
	down(6)
	dashedRule()
	down(4)
	center("Thank you for shopping with us!", 10)
	down(4)
	center(receiptNo, 11)
	down(8)

	seed := int64(0)
	for _, c := range receiptNo {
		seed += int64(c)
	}
	rnd := rand.New(rand.NewSource(seed))
	widths := []float64{0.35, 0.55, 0.8, 1.05}
	cx := margin
	top := pdf.GetY()
	const barH = 9.0
	for cx < margin+contentW {
		w := widths[rnd.Intn(len(widths))]
		if rnd.Float64() > 0.35 {
			pdf.SetLineWidth(w)
			pdf.Line(cx, top, cx, top+barH)
		}
		cx += w + 0.3
	}
	down(barH + 3)
	center(receiptNo, 11)
	down(6)

	path := fmt.Sprintf("%s.png", receiptNo)
	if err := pdf.OutputFileAndClose(path); err != nil {
		return "", err
	}
	log.Println("receipt saved:", path)
	return path, nil
}
