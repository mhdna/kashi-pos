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
}

type exchangeRequest struct {
	returned     exchangeLine
	replacements []exchangeLine
}

type pricedExchange struct {
	returnedProduct  db.Product
	returnedLine     db.InvoiceProduct
	returnedTotal    int64
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
			if line.Qty != -1 || request.returned.Code != "" {
				return exchangeRequest{}, fmt.Errorf("an exchange supports exactly one returned item")
			}
			if line.UnitPriceCents != nil && *line.UnitPriceCents <= 0 {
				return exchangeRequest{}, fmt.Errorf("invalid returned item price")
			}
			if line.UnitDiscount != nil && (*line.UnitDiscount < 0 || *line.UnitDiscount > 100) {
				return exchangeRequest{}, fmt.Errorf("invalid returned item discount")
			}
			request.returned = line
			continue
		}
		normalizedCode := strings.ToLower(line.Code)
		if request.returned.Code != "" && strings.EqualFold(line.Code, request.returned.Code) {
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
	if request.returned.Code == "" || (len(request.replacements) == 0 && (len(returnOnly) == 0 || !returnOnly[0])) {
		return exchangeRequest{}, fmt.Errorf("an exchange needs one returned item and at least one replacement")
	}
	for _, replacement := range request.replacements {
		if strings.EqualFold(replacement.Code, request.returned.Code) {
			return exchangeRequest{}, fmt.Errorf("choose different products as replacements")
		}
	}
	return request, nil
}

func (a *App) validateAndPriceExchange(q *db.Queries, sourceID int64, request exchangeRequest, roundingMode string) (pricedExchange, error) {
	var result pricedExchange
	returnedProduct, err := q.GetProductByCode(a.ctx, request.returned.Code)
	if err != nil {
		return result, fmt.Errorf("returned product: %w", err)
	}
	sourceLines, err := q.ListInvoiceProducts(a.ctx, sourceID)
	if err != nil {
		return result, fmt.Errorf("returned product was not sold on invoice")
	}
	var originalLine db.InvoiceProduct
	foundAvailableLine := false
	for _, line := range sourceLines {
		if line.ProductID != returnedProduct.ID {
			continue
		}
		if request.returned.UnitPriceCents != nil && line.UnitPrice != *request.returned.UnitPriceCents {
			continue
		}
		if request.returned.UnitDiscount != nil && line.Discount != *request.returned.UnitDiscount {
			continue
		}
		returnedQty, returnedErr := q.GetReturnedQuantityForProductPrice(a.ctx, db.GetReturnedQuantityForProductPriceParams{
			SalesInvoiceID: sourceID,
			ProductID:      returnedProduct.ID,
			UnitPrice:      line.UnitPrice,
			Discount:       line.Discount,
		})
		if returnedErr == nil && databaseInt64(returnedQty) < line.Quantity {
			originalLine = line
			foundAvailableLine = true
			break
		}
	}
	if !foundAvailableLine {
		return result, fmt.Errorf("item has already been returned")
	}

	result.returnedProduct = returnedProduct
	result.returnedLine = originalLine
	result.returnedTotal = originalLine.UnitPrice -
		roundRatio(originalLine.UnitPrice*originalLine.Discount, 100, roundingMode)
	result.replacements = make([]pricedInvoiceProduct, 0, len(request.replacements))

	for _, line := range request.replacements {
		product, err := q.GetProductByCode(a.ctx, line.Code)
		if err != nil || product.IsActive == 0 {
			return result, fmt.Errorf("replacement product %s not found or inactive", line.Code)
		}
		priced, err := priceInvoiceProduct(product, line.Qty, roundingMode)
		if err != nil {
			return result, err
		}
		if result.replacementTotal > math.MaxInt64-priced.lineTotal {
			return result, fmt.Errorf("replacement total is too large")
		}
		result.replacementTotal += priced.lineTotal
		result.replacements = append(result.replacements, priced)
	}
	return result, nil
}
