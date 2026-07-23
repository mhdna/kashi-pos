package main

import "fmt"

func (a *App) GetProductByBarcode(barcode string) (map[string]any, error) {
	if a.queries == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	product, err := a.queries.GetProductByBarcode(a.ctx, barcode)
	if err != nil {
		return nil, fmt.Errorf("product not found")
	}

	return map[string]any{
		"id":       product.ID,
		"barcode":  product.Barcode,
		"code":     product.Code,
		"name":     product.Name,
		"category": product.Description,
		"price":    float64(product.Price) / 100,
		"discount": product.Discount,
	}, nil
}

func (a *App) GetProductByCode(code string) (map[string]any, error) {
	if a.queries == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	product, err := a.queries.GetProductByCode(a.ctx, code)
	if err != nil {
		return nil, fmt.Errorf("product not found")
	}

	return map[string]any{
		"id":       product.ID,
		"barcode":  product.Barcode,
		"code":     product.Code,
		"name":     product.Name,
		"category": product.Description,
		"price":    float64(product.Price) / 100,
		"discount": product.Discount,
	}, nil
}

func (a *App) ListProducts() ([]map[string]any, error) {
	if a.queries == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	products, err := a.queries.ListProducts(a.ctx)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, len(products))
	for i, p := range products {
		result[i] = map[string]any{
			"id":       p.ID,
			"barcode":  p.Barcode,
			"code":     p.Code,
			"name":     p.Name,
			"category": p.Description,
			"price":    float64(p.Price) / 100,
			"discount": p.Discount,
		}
	}
	return result, nil
}
