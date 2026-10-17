package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	db "kashi-pos/db/sqlc"
)

type exchangeLine struct {
	Code string `json:"code"`
	// Barcode, when set, names the exact variant: codes repeat across a
	// product's colours and sizes, so a code alone resolves to the first
	// (see lineProduct). Sales already go by barcode the same way.
	Barcode        string `json:"barcode,omitempty"`
	Qty            int64  `json:"qty"`
	UnitPriceCents *int64 `json:"unitPriceCents,omitempty"`
	UnitDiscount   *int64 `json:"unitDiscount,omitempty"`
	SalespersonID  int64  `json:"salespersonId,omitempty"`
	Note           string `json:"note,omitempty"`
	// The following only apply to replacement (positive-qty) lines - a
	// returned line's price/discount always comes from the original sold
	// line (UnitPriceCents/UnitDiscount above are only ever used to *match*
	// which original line is being returned, never to reprice it).
	CustomPriceCents *int64  `json:"customPriceCents,omitempty"`
	UnitPricesCents  []int64 `json:"unitPricesCents,omitempty"`
	UnitDiscounts    []int64 `json:"unitDiscounts,omitempty"`
}

type exchangeRequest struct {
	// returned is kept as the first returned line for compatibility with the
	// existing validation/tests. returnedItems contains the complete request.
	returned      exchangeLine
	returnedItems []exchangeLine
	replacements  []exchangeLine
}

type pricedReturnedProduct struct {
	product       db.Product
	line          db.InvoiceProduct
	qty           int64
	lineTotal     int64
	note          string
	salespersonID int64
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
		line.Barcode = strings.TrimSpace(line.Barcode)
		line.Note = strings.TrimSpace(line.Note)
		if line.Code == "" || line.Qty == 0 {
			return exchangeRequest{}, fmt.Errorf("invalid exchange item")
		}
		if len([]rune(line.Note)) > 2000 {
			return exchangeRequest{}, fmt.Errorf("note is too long for item %q", line.Code)
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
				if strings.EqualFold(returned.Code, line.Code) && returned.Barcode == line.Barcode &&
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
		if line.CustomPriceCents != nil && (*line.CustomPriceCents <= 0 || *line.CustomPriceCents > 99999999) {
			return exchangeRequest{}, fmt.Errorf("invalid custom price for replacement %q", line.Code)
		}
		if len(line.UnitPricesCents) > 0 {
			if int64(len(line.UnitPricesCents)) != line.Qty {
				return exchangeRequest{}, fmt.Errorf("unit prices do not match quantity for replacement %q", line.Code)
			}
			for _, price := range line.UnitPricesCents {
				if price <= 0 || price > 99999999 {
					return exchangeRequest{}, fmt.Errorf("invalid unit price for replacement %q", line.Code)
				}
			}
		}
		if len(line.UnitDiscounts) > 0 {
			if int64(len(line.UnitDiscounts)) != line.Qty {
				return exchangeRequest{}, fmt.Errorf("unit discounts do not match quantity for replacement %q", line.Code)
			}
			for _, discount := range line.UnitDiscounts {
				if discount < 0 || discount > 100 {
					return exchangeRequest{}, fmt.Errorf("invalid unit discount for replacement %q", line.Code)
				}
			}
		}
		// A replacement is allowed to reuse the same product code as a
		// returned line (e.g. swapping a defective unit for a fresh one of
		// the same product) - it's kept as its own separate line rather than
		// netted against the return, so the returned line stays negative and
		// the replacement stays positive.
		normalizedCode := strings.ToLower(line.Code) + "\x00" + line.Barcode
		hasCustomPricing := line.CustomPriceCents != nil || len(line.UnitPricesCents) > 0 || len(line.UnitDiscounts) > 0
		if index, exists := replacementIndex[normalizedCode]; exists {
			existing := request.replacements[index]
			existingHasCustomPricing := existing.CustomPriceCents != nil || len(existing.UnitPricesCents) > 0 || len(existing.UnitDiscounts) > 0
			if hasCustomPricing || existingHasCustomPricing {
				return exchangeRequest{}, fmt.Errorf("send a custom-priced replacement %q as a single line", line.Code)
			}
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
	return request, nil
}

// lineProduct is the product an exchange line names: by barcode when it
// carries one, else the first product with its code.
func lineProduct(a *App, q *db.Queries, line exchangeLine) (db.Product, error) {
	if line.Barcode != "" {
		return q.GetProductByBarcode(a.ctx, line.Barcode)
	}
	return q.GetProductByCode(a.ctx, line.Code)
}

func (a *App) validateAndPriceExchange(q *db.Queries, sourceID int64, request exchangeRequest, roundingMode string, customPricingEnabled, perUnitPricingEnabled bool, listPricing activePricing) (pricedExchange, error) {
	var result pricedExchange
	sourceLines, err := q.ListInvoiceProducts(a.ctx, sourceID)
	if err != nil {
		return result, fmt.Errorf("returned product was not sold on invoice")
	}
	reserved := make(map[string]int64)
	for _, requested := range request.returnedItems {
		if requested.SalespersonID > 0 {
			if _, err := q.GetSalespersonByID(a.ctx, requested.SalespersonID); err != nil {
				return result, fmt.Errorf("salesperson for returned item %s: %w", requested.Code, err)
			}
		}
		product, err := lineProduct(a, q, requested)
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
		priced := pricedReturnedProduct{product: product, line: originalLine, qty: requestedQty, lineTotal: unitTotal * requestedQty, note: requested.Note, salespersonID: requested.SalespersonID}
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
		product, err := lineProduct(a, q, line)
		if err != nil || product.IsActive == 0 {
			return result, fmt.Errorf("replacement product %s not found or inactive", line.Code)
		}
		// A replacement is a sale, so it is priced from the active lists
		// exactly as validateAndPriceItems prices a normal line. Before the
		// lists became authoritative this path read product.Price straight
		// off the row and so escaped price/discount lists entirely -- taking
		// a replacement was the one way to sell at a price central had not
		// set. A replacement for a product in no list now fails to price,
		// like any other unsellable line.
		product.Price = 0
		product.Discount = 0
		if listed, ok := listPricing.priceFor(product.Uid.String); ok {
			product.Price = listed.cents
		}
		if listed, ok := listPricing.discountFor(product.Uid.String); ok {
			product.Discount = listed.percent
		}
		if line.CustomPriceCents != nil {
			if !customPricingEnabled {
				return result, fmt.Errorf("custom item pricing is disabled")
			}
			product.Price = *line.CustomPriceCents
		}
		// Mirrors validateAndPriceItems's per-unit bucketing: a replacement
		// line with per-unit price/discount overrides is split into one
		// priced sub-line per distinct (price, discount) pair, keeping unit
		// pricing consistent between a normal sale and a replacement line.
		priceQuantities := map[unitPricing]int64{
			{price: product.Price, discount: product.Discount}: line.Qty,
		}
		if len(line.UnitPricesCents) > 0 || len(line.UnitDiscounts) > 0 {
			if !perUnitPricingEnabled {
				return result, fmt.Errorf("per-unit item pricing is disabled")
			}
			priceQuantities = make(map[unitPricing]int64)
			for index := int64(0); index < line.Qty; index++ {
				price := product.Price
				if len(line.UnitPricesCents) > 0 {
					price = line.UnitPricesCents[index]
				}
				discount := product.Discount
				if len(line.UnitDiscounts) > 0 {
					discount = line.UnitDiscounts[index]
				}
				priceQuantities[unitPricing{price: price, discount: discount}]++
			}
		}
		prices := make([]unitPricing, 0, len(priceQuantities))
		for pricing := range priceQuantities {
			prices = append(prices, pricing)
		}
		sort.Slice(prices, func(i, j int) bool {
			if prices[i].price == prices[j].price {
				return prices[i].discount < prices[j].discount
			}
			return prices[i].price < prices[j].price
		})
		for _, pricing := range prices {
			qty := priceQuantities[pricing]
			pricedProduct := product
			pricedProduct.Price = pricing.price
			pricedProduct.Discount = pricing.discount
			priced, err := priceInvoiceProduct(pricedProduct, qty, roundingMode)
			if err != nil {
				return result, err
			}
			priced.salespersonID = line.SalespersonID
			priced.note = line.Note
			if result.replacementTotal > math.MaxInt64-priced.lineTotal {
				return result, fmt.Errorf("replacement total is too large")
			}
			result.replacementTotal += priced.lineTotal
			result.replacements = append(result.replacements, priced)
		}
	}
	return result, nil
}
