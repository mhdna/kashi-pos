package main

import (
	"context"
	"fmt"
	"unicode/utf8"

	db "kashi-pos/db/sqlc"
)

// minIndexedSearchRunes is the shortest term the trigram index can answer;
// anything shorter falls back to the plain LIKE queries.
const minIndexedSearchRunes = 3

func searchUsesIndex(search string) bool {
	return utf8.RuneCountInString(search) >= minIndexedSearchRunes
}

// productSearchColumn picks the products_search column a search matches:
// everything, or only code and barcode for tills that hide product names.
func productSearchColumn(codeOnly bool) string {
	if codeOnly {
		return "code_text"
	}
	return "all_text"
}

// listProductsAfterIndexed is ListProductsAfter / ListProductsAfterByCode
// answered from the products_search trigram index (migration 000135). Same
// rows, same order, same filters; the pattern is wrapped in '%...%' by the
// caller exactly as for the LIKE queries.
func (a *App) listProductsAfterIndexed(ctx context.Context, afterID int64, pattern string, limit int64, codeOnly bool) ([]db.Product, error) {
	query := fmt.Sprintf(`SELECT p.id, p.barcode, p.code, p.name, p.description, p.is_active, p.price, p.discount,
       p.color, p.size, p.currency_code, p.created_at, p.brand, p.family, p.kind, p.season,
       p.product_year, p.uid, p.remote_updated_at
FROM products p
JOIN products_search s ON s.rowid = p.id
WHERE s.%s LIKE ?1
  AND p.id > ?2
  AND p.is_active = 1
  AND p.barcode != 'UNKNOWN'
ORDER BY p.id
LIMIT ?3`, productSearchColumn(codeOnly))
	rows, err := a.store.DB().QueryContext(ctx, query, pattern, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []db.Product{}
	for rows.Next() {
		var i db.Product
		if err := rows.Scan(
			&i.ID, &i.Barcode, &i.Code, &i.Name, &i.Description, &i.IsActive, &i.Price, &i.Discount,
			&i.Color, &i.Size, &i.CurrencyCode, &i.CreatedAt, &i.Brand, &i.Family, &i.Kind, &i.Season,
			&i.ProductYear, &i.Uid, &i.RemoteUpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// countProductsIndexed is CountProducts / CountProductsByCode from the index.
func (a *App) countProductsIndexed(ctx context.Context, pattern string, codeOnly bool) (int64, error) {
	query := fmt.Sprintf(`SELECT COUNT(*)
FROM products p
JOIN products_search s ON s.rowid = p.id
WHERE s.%s LIKE ?1
  AND p.is_active = 1
  AND p.barcode != 'UNKNOWN'`, productSearchColumn(codeOnly))
	var total int64
	err := a.store.DB().QueryRowContext(ctx, query, pattern).Scan(&total)
	return total, err
}
