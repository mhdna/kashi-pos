package main

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
)

// A branch added on a till starts its own empty database. Products and
// clients are central's, the same for every branch, so instead of
// downloading all of them again (tens of thousands of rows) the new branch
// copies them from a branch already set up on this till. Its first sync then
// compares ids and versions with central (seedCursorFromLocalCatalog, the
// full check) and fetches only what is missing or newer. What is the
// branch's own (stock, prices, shifts, invoices, points) is never copied.

// seedClientColumns are the client columns central owns: loyalty points
// are the branch's, so a copy starts them at zero.
var seedClientColumns = []string{"id", "name", "phone", "is_active", "uid", "remote_updated_at", "created_at", "referral_code"}

// seedCatalogFromBranch copies central's products (with their sale options)
// and clients from the database at srcPath into conn's. Only rows central
// sent (a uid and a version) are copied; the walk-in client is the new
// branch's own. Rows that clash with one already there are left out: the
// first sync fetches them.
func seedCatalogFromBranch(ctx context.Context, conn *sql.DB, srcPath string) (products, clients int64, err error) {
	c, err := conn.Conn(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer c.Close()
	if _, err := c.ExecContext(ctx, `ATTACH DATABASE ? AS seedsrc`, srcPath); err != nil {
		return 0, 0, fmt.Errorf("open the other branch: %w", err)
	}
	defer c.ExecContext(context.Background(), `DETACH DATABASE seedsrc`)

	shared := func(table string, only []string) ([]string, error) {
		cols := func(schema string) ([]string, error) {
			rows, err := c.QueryContext(ctx, fmt.Sprintf(`SELECT name FROM pragma_table_info('%s', '%s')`, table, schema))
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			var out []string
			for rows.Next() {
				var n string
				if err := rows.Scan(&n); err != nil {
					return nil, err
				}
				out = append(out, n)
			}
			return out, rows.Err()
		}
		mine, err := cols("main")
		if err != nil {
			return nil, err
		}
		theirs, err := cols("seedsrc")
		if err != nil {
			return nil, err
		}
		var out []string
		for _, n := range mine {
			if slices.Contains(theirs, n) && (only == nil || slices.Contains(only, n)) {
				out = append(out, `"`+n+`"`)
			}
		}
		return out, nil
	}
	copyRows := func(table string, only []string, where string) (int64, error) {
		cols, err := shared(table, only)
		if err != nil || len(cols) == 0 {
			return 0, err
		}
		list := strings.Join(cols, ", ")
		res, err := c.ExecContext(ctx, fmt.Sprintf(`INSERT OR IGNORE INTO main.%s (%s) SELECT %s FROM seedsrc.%s WHERE %s`, table, list, list, table, where))
		if err != nil {
			return 0, fmt.Errorf("copy %s: %w", table, err)
		}
		return res.RowsAffected()
	}

	if _, err := c.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return 0, 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = c.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()
	if products, err = copyRows("products", nil, `uid IS NOT NULL AND remote_updated_at IS NOT NULL`); err != nil {
		return 0, 0, err
	}
	if _, err = copyRows("product_sale_options", nil, `product_id IN (SELECT id FROM main.products)`); err != nil {
		return 0, 0, err
	}
	if clients, err = copyRows("clients", seedClientColumns,
		fmt.Sprintf(`uid IS NOT NULL AND remote_updated_at IS NOT NULL AND phone <> '%s'`, walkInClientPhone)); err != nil {
		return 0, 0, err
	}
	if _, err := c.ExecContext(ctx, `COMMIT`); err != nil {
		return 0, 0, err
	}
	committed = true
	return products, clients, nil
}
