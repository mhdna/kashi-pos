package main

import (
	"encoding/json"
	"fmt"
	"math"

	db "kashi-pos/db/sqlc"
)

type exchangeLine struct {
	Code string `json:"code"`
	Qty  int64  `json:"qty"`
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

func parseExchangeRequest(raw string) (exchangeRequest, error) {
	var lines []exchangeLine
	if err := json.Unmarshal([]byte(raw), &lines); err != nil || len(lines) < 2 {
		return exchangeRequest{}, fmt.Errorf("an exchange needs one returned item and at least one replacement")
	}

	var request exchangeRequest
	replacementIndex := make(map[string]int)
	for _, line := range lines {
		if line.Code == "" || line.Qty == 0 {
			return exchangeRequest{}, fmt.Errorf("invalid exchange item")
		}
		if line.Qty < 0 {
			if line.Qty != -1 || request.returned.Code != "" {
				return exchangeRequest{}, fmt.Errorf("an exchange supports exactly one returned item")
			}
			request.returned = line
			continue
		}
		if line.Code == request.returned.Code {
			return exchangeRequest{}, fmt.Errorf("choose different products as replacements")
		}
		if index, exists := replacementIndex[line.Code]; exists {
			current := request.replacements[index].Qty
			if line.Qty > math.MaxInt64-current {
				return exchangeRequest{}, fmt.Errorf("replacement quantity is too large")
			}
			request.replacements[index].Qty += line.Qty
			continue
		}
		replacementIndex[line.Code] = len(request.replacements)
		request.replacements = append(request.replacements, line)
	}
	if request.returned.Code == "" || len(request.replacements) == 0 {
		return exchangeRequest{}, fmt.Errorf("an exchange needs one returned item and at least one replacement")
	}
	for _, replacement := range request.replacements {
		if replacement.Code == request.returned.Code {
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
	originalLine, err := q.GetInvoiceProduct(a.ctx, db.GetInvoiceProductParams{
		InvoiceID: sourceID,
		ProductID: returnedProduct.ID,
	})
	if err != nil {
		return result, fmt.Errorf("returned product was not sold on invoice")
	}
	returnedQty, err := q.GetReturnedQuantityForProduct(a.ctx, db.GetReturnedQuantityForProductParams{
		SalesInvoiceID: sourceID,
		ProductID:      returnedProduct.ID,
	})
	if err != nil || databaseInt64(returnedQty)+1 > originalLine.Quantity {
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
	if result.replacementTotal < result.returnedTotal {
		return result, fmt.Errorf("combined replacement total must equal or exceed the returned item")
	}
	return result, nil
}
