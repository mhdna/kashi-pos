package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	db "kashi-pos/db/sqlc"
)

type exchangeLine struct {
	Code           string `json:"code"`
	Qty            int64  `json:"qty"`
	UnitPriceCents *int64 `json:"unitPriceCents,omitempty"`
	UnitDiscount   *int64 `json:"unitDiscount,omitempty"`
	SalespersonID  int64  `json:"salespersonId,omitempty"`
}

type exchangeRequest struct {
	// returned is kept as the first returned line for compatibility with the
	// existing validation/tests. returnedItems contains the complete request.
	returned      exchangeLine
	returnedItems []exchangeLine
	replacements  []exchangeLine
}

type pricedReturnedProduct struct {
	product   db.Product
	line      db.InvoiceProduct
	qty       int64
	lineTotal int64
}

type pricedExchange struct {
	returnedProduct  db.Product
	returnedLine     db.InvoiceProduct
	returnedTotal    int64
	returned         []pricedReturnedProduct
	replacements     []pricedInvoiceProduct
	replacementTotal int64
}

func parseExchangeRequest(raw string, returnOnly ...bool) (exchangeRequest, error) {
	var lines []exchangeLine
	minimumLines := 2
	if len(returnOnly) > 0 && returnOnly[0] {
		minimumLines = 1
	}
	if err := json.Unmarshal([]byte(raw), &lines); err != nil || len(lines) < minimumLines {
		return exchangeRequest{}, fmt.Errorf("an exchange needs one returned item and at least one replacement")
	}

	var request exchangeRequest
	replacementIndex := make(map[string]int)
	for _, line := range lines {
		line.Code = strings.TrimSpace(line.Code)
		if line.Code == "" || line.Qty == 0 {
			return exchangeRequest{}, fmt.Errorf("invalid exchange item")
		}
		if line.Qty < 0 {
			if line.UnitPriceCents != nil && *line.UnitPriceCents <= 0 {
				return exchangeRequest{}, fmt.Errorf("invalid returned item price")
			}
			if line.UnitDiscount != nil && (*line.UnitDiscount < 0 || *line.UnitDiscount > 100) {
				return exchangeRequest{}, fmt.Errorf("invalid returned item discount")
			}
			returnedIndex := -1
			for index, returned := range request.returnedItems {
				if strings.EqualFold(returned.Code, line.Code) &&
					(returned.UnitPriceCents == nil || line.UnitPriceCents == nil || *returned.UnitPriceCents == *line.UnitPriceCents) &&
					(returned.UnitDiscount == nil || line.UnitDiscount == nil || *returned.UnitDiscount == *line.UnitDiscount) {
					returnedIndex = index
					break
				}
			}
			if returnedIndex >= 0 {
				if line.Qty < -math.MaxInt64-request.returnedItems[returnedIndex].Qty {
					return exchangeRequest{}, fmt.Errorf("returned quantity is too large")
				}
				request.returnedItems[returnedIndex].Qty += line.Qty
			} else {
				request.returnedItems = append(request.returnedItems, line)
			}
			if request.returned.Code == "" {
				request.returned = line
			}
			continue
		}
		normalizedCode := strings.ToLower(line.Code)
		if len(request.returnedItems) > 0 && anyReturnedCode(request.returnedItems, line.Code) {
			return exchangeRequest{}, fmt.Errorf("choose different products as replacements")
		}
		if index, exists := replacementIndex[normalizedCode]; exists {
			current := request.replacements[index].Qty
			if line.Qty > math.MaxInt64-current {
				return exchangeRequest{}, fmt.Errorf("replacement quantity is too large")
			}
			request.replacements[index].Qty += line.Qty
			continue
		}
		replacementIndex[normalizedCode] = len(request.replacements)
		request.replacements = append(request.replacements, line)
	}
	if len(request.returnedItems) == 0 || (len(request.replacements) == 0 && (len(returnOnly) == 0 || !returnOnly[0])) {
		return exchangeRequest{}, fmt.Errorf("an exchange needs one returned item and at least one replacement")
	}
	for _, replacement := range request.replacements {
		if anyReturnedCode(request.returnedItems, replacement.Code) {
			return exchangeRequest{}, fmt.Errorf("choose different products as replacements")
		}
	}
	return request, nil
}

func anyReturnedCode(returned []exchangeLine, code string) bool {
	for _, line := range returned {
		if strings.EqualFold(line.Code, code) {
			return true
		}
	}
	return false
}

func (a *App) validateAndPriceExchange(q *db.Queries, sourceID int64, request exchangeRequest, roundingMode string) (pricedExchange, error) {
	var result pricedExchange
	sourceLines, err := q.ListInvoiceProducts(a.ctx, sourceID)
	if err != nil {
		return result, fmt.Errorf("returned product was not sold on invoice")
	}
	reserved := make(map[string]int64)
	for _, requested := range request.returnedItems {
		product, err := q.GetProductByCode(a.ctx, requested.Code)
		if err != nil {
			return result, fmt.Errorf("returned product: %w", err)
		}
		var originalLine db.InvoiceProduct
		foundAvailableLine := false
		requestedQty := -requested.Qty
		for _, line := range sourceLines {
			if line.ProductID != product.ID || (requested.UnitPriceCents != nil && line.UnitPrice != *requested.UnitPriceCents) || (requested.UnitDiscount != nil && line.Discount != *requested.UnitDiscount) {
				continue
			}
			returnedQty, returnedErr := q.GetReturnedQuantityForProductPrice(a.ctx, db.GetReturnedQuantityForProductPriceParams{SalesInvoiceID: sourceID, ProductID: product.ID, UnitPrice: line.UnitPrice, Discount: line.Discount})
			lineKey := fmt.Sprintf("%d:%d:%d", product.ID, line.UnitPrice, line.Discount)
			if returnedErr == nil && requestedQty <= line.Quantity-databaseInt64(returnedQty)-reserved[lineKey] {
				originalLine = line
				foundAvailableLine = true
				reserved[lineKey] += requestedQty
				break
			}
		}
		if !foundAvailableLine {
			return result, fmt.Errorf("item has already been returned")
		}
		unitTotal := originalLine.UnitPrice - roundRatio(originalLine.UnitPrice*originalLine.Discount, 100, roundingMode)
		if requestedQty > math.MaxInt64/unitTotal {
			return result, fmt.Errorf("returned total is too large")
		}
		priced := pricedReturnedProduct{product: product, line: originalLine, qty: requestedQty, lineTotal: unitTotal * requestedQty}
		result.returned = append(result.returned, priced)
		result.returnedTotal += priced.lineTotal
	}
	if len(result.returned) > 0 {
		result.returnedProduct = result.returned[0].product
		result.returnedLine = result.returned[0].line
	}
	result.replacements = make([]pricedInvoiceProduct, 0, len(request.replacements))

	for _, line := range request.replacements {
		if line.SalespersonID > 0 {
			if _, err := q.GetSalespersonByID(a.ctx, line.SalespersonID); err != nil {
				return result, fmt.Errorf("salesperson for replacement %s: %w", line.Code, err)
			}
		}
		product, err := q.GetProductByCode(a.ctx, line.Code)
		if err != nil || product.IsActive == 0 {
			return result, fmt.Errorf("replacement product %s not found or inactive", line.Code)
		}
		priced, err := priceInvoiceProduct(product, line.Qty, roundingMode)
		if err != nil {
			return result, err
		}
		priced.salespersonID = line.SalespersonID
		if result.replacementTotal > math.MaxInt64-priced.lineTotal {
			return result, fmt.Errorf("replacement total is too large")
		}
		result.replacementTotal += priced.lineTotal
		result.replacements = append(result.replacements, priced)
	}
	return result, nil
}
