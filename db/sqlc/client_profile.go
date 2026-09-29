package db

import (
	"context"
	"database/sql"
)

// Hand-written queries for the client profile view (not SQLC generated).

// ClientProfileStats holds aggregate purchase stats for one client.
type ClientProfileStats struct {
	TotalSpentCents int64
	ItemsBought     int64
	FirstPurchaseAt sql.NullString
}

const clientProfileStats = `
SELECT
    COALESCE(SUM(i.grand_total), 0),
    COALESCE((
        SELECT SUM(ip2.quantity)
        FROM invoice_products ip2
        JOIN invoices i2 ON i2.id = ip2.invoice_id
        JOIN sales_invoices si2 ON si2.invoice_id = i2.id
        WHERE i2.client_id = ? AND ip2.quantity > 0
    ), 0),
    MIN(i.created_at)
FROM invoices i
JOIN sales_invoices si ON si.invoice_id = i.id
WHERE i.client_id = ?
`

func (q *Queries) GetClientProfileStats(ctx context.Context, clientID int64) (ClientProfileStats, error) {
	var r ClientProfileStats
	err := q.db.QueryRowContext(ctx, clientProfileStats, clientID, clientID).Scan(
		&r.TotalSpentCents, &r.ItemsBought, &r.FirstPurchaseAt,
	)
	return r, err
}

// ClientHomeBranch is the branch where the client has made the most purchases.
const clientHomeBranch = `
SELECT inv.name
FROM invoices i
JOIN sales_invoices si ON si.invoice_id = i.id
JOIN inventories inv ON inv.id = i.inventory_id
WHERE i.client_id = ?
GROUP BY inv.id, inv.name
ORDER BY COUNT(*) DESC
LIMIT 1
`

func (q *Queries) GetClientHomeBranch(ctx context.Context, clientID int64) (string, error) {
	var name string
	err := q.db.QueryRowContext(ctx, clientHomeBranch, clientID).Scan(&name)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return name, err
}

// ClientFamilySpendRow is one row of the spend-by-category breakdown.
type ClientFamilySpendRow struct {
	Family     string
	Kind       string
	TotalCents int64
}

const clientFamilySpend = `
SELECT
    CASE WHEN TRIM(p.family) = '' THEN 'Other' ELSE p.family END,
    CASE WHEN TRIM(p.kind)   = '' THEN 'Other' ELSE p.kind   END,
    SUM(ip.line_total)
FROM invoices i
JOIN sales_invoices si ON si.invoice_id = i.id
JOIN invoice_products ip ON ip.invoice_id = i.id AND ip.quantity > 0
JOIN products p ON p.id = ip.product_id
WHERE i.client_id = ?
GROUP BY p.family, p.kind
ORDER BY SUM(ip.line_total) DESC
`

func (q *Queries) ListClientFamilySpend(ctx context.Context, clientID int64) ([]ClientFamilySpendRow, error) {
	rows, err := q.db.QueryContext(ctx, clientFamilySpend, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ClientFamilySpendRow
	for rows.Next() {
		var r ClientFamilySpendRow
		if err := rows.Scan(&r.Family, &r.Kind, &r.TotalCents); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// ClientPaymentMixRow is one slice of the payment-mix donut.
type ClientPaymentMixRow struct {
	AccountName string
	TotalCents  int64
}

const clientPaymentMix = `
SELECT ca.name, SUM(ipay.amount)
FROM invoices i
JOIN sales_invoices si ON si.invoice_id = i.id
JOIN invoice_payments ipay ON ipay.invoice_id = i.id AND ipay.amount > 0
JOIN cashbox_accounts ca ON ca.id = ipay.cashbox_id
WHERE i.client_id = ?
GROUP BY ca.id, ca.name
ORDER BY SUM(ipay.amount) DESC
`

func (q *Queries) ListClientPaymentMix(ctx context.Context, clientID int64) ([]ClientPaymentMixRow, error) {
	rows, err := q.db.QueryContext(ctx, clientPaymentMix, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ClientPaymentMixRow
	for rows.Next() {
		var r ClientPaymentMixRow
		if err := rows.Scan(&r.AccountName, &r.TotalCents); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// ClientRecentInvoiceRow is one row in the recent-purchases table.
type ClientRecentInvoiceRow struct {
	ID             int64
	InvoiceCode    string
	GrandTotal     int64
	CreatedAt      string
	BranchName     string
	ItemCount      int64
	PaymentMethods string // comma-separated account names
}

const clientRecentInvoices = `
SELECT
    i.id,
    i.invoice_code,
    i.grand_total,
    i.created_at,
    inv.name,
    (SELECT SUM(ip2.quantity) FROM invoice_products ip2 WHERE ip2.invoice_id = i.id AND ip2.quantity > 0),
    GROUP_CONCAT(DISTINCT ca.name)
FROM invoices i
JOIN sales_invoices si ON si.invoice_id = i.id
JOIN inventories inv ON inv.id = i.inventory_id
LEFT JOIN invoice_payments ipay ON ipay.invoice_id = i.id AND ipay.amount > 0
LEFT JOIN cashbox_accounts ca ON ca.id = ipay.cashbox_id
WHERE i.client_id = ?
GROUP BY i.id
ORDER BY i.id DESC
LIMIT 10
`

func (q *Queries) ListClientRecentInvoices(ctx context.Context, clientID int64) ([]ClientRecentInvoiceRow, error) {
	rows, err := q.db.QueryContext(ctx, clientRecentInvoices, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ClientRecentInvoiceRow
	for rows.Next() {
		var r ClientRecentInvoiceRow
		var itemCount sql.NullInt64
		var payMethods sql.NullString
		if err := rows.Scan(&r.ID, &r.InvoiceCode, &r.GrandTotal, &r.CreatedAt,
			&r.BranchName, &itemCount, &payMethods); err != nil {
			return nil, err
		}
		r.ItemCount = itemCount.Int64
		r.PaymentMethods = payMethods.String
		result = append(result, r)
	}
	return result, rows.Err()
}

// ClientMonthlySpendRow is one month of the sparkline series.
type ClientMonthlySpendRow struct {
	Month      string // "YYYY-MM"
	TotalCents int64
	ItemCount  int64
}

const clientMonthlySpend = `
SELECT
    strftime('%Y-%m', i.created_at, 'localtime') AS month,
    SUM(i.grand_total),
    COALESCE((
        SELECT SUM(ip2.quantity)
        FROM invoice_products ip2
        WHERE ip2.invoice_id = i.id AND ip2.quantity > 0
    ), 0)
FROM invoices i
JOIN sales_invoices si ON si.invoice_id = i.id
WHERE i.client_id = ?
  AND i.created_at >= date('now', '-12 months', 'localtime')
GROUP BY month
ORDER BY month
`

func (q *Queries) ListClientMonthlySpend(ctx context.Context, clientID int64) ([]ClientMonthlySpendRow, error) {
	rows, err := q.db.QueryContext(ctx, clientMonthlySpend, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ClientMonthlySpendRow
	for rows.Next() {
		var r ClientMonthlySpendRow
		if err := rows.Scan(&r.Month, &r.TotalCents, &r.ItemCount); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
