package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

// ProductInput is one product as entered in the form.
type ProductInput struct {
	Barcode     string `json:"barcode"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Size        string `json:"size"`
	Brand       string `json:"brand"`
	Family      string `json:"family"`
	Kind        string `json:"kind"`
	Season      string `json:"season"`
	Price       string `json:"price"`
	Discount    int64  `json:"discount"`
	Year        int64  `json:"year"`
	IsActive    bool   `json:"isActive"`
	QuickAccess bool   `json:"quickAccess"`
}

type product struct {
	ProductInput
	PriceCents int64
}

// RowResult reports the outcome for one queued product.
type RowResult struct {
	File    string `json:"file,omitempty"`
	Index   int    `json:"index"`
	Barcode string `json:"barcode"`
	Status  string `json:"status"` // "imported" | "error"
	Message string `json:"message"`
}

type ImportResult struct {
	Imported int         `json:"imported"`
	Failed   int         `json:"failed"`
	Rows     []RowResult `json:"rows"`
}

// DatabaseInfo describes the chosen database for the form: existing values
// for the dropdowns, and where prices will be written.
type DatabaseInfo struct {
	Brands   []string `json:"brands"`
	Families []string `json:"families"`
	Kinds    []string `json:"kinds"`
	Seasons  []string `json:"seasons"`
	Colors   []string `json:"colors"`
	Sizes    []string `json:"sizes"`
	// PriceList is the default price list's name, or "" on databases that
	// predate price lists (000097) and still sell at the product row price.
	PriceList    string `json:"priceList"`
	DiscountList string `json:"discountList"`
	// ListPricing: this POS version sells only at price-list prices.
	ListPricing bool `json:"listPricing"`
	Connected   bool `json:"connected"`
}

type kashiDB struct {
	conn *sql.DB
	// listPricing: the till ignores products.price/discount and sells only
	// at prices found in the active price/discount lists, keyed by uid.
	listPricing    bool
	priceListID    int64
	priceListName  string
	discountListID int64
	discountName   string
}

func openKashiDB(dbPath string) (*kashiDB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("choose a database file first")
	}
	// busy_timeout: the POS may have the database open while we write.
	conn, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	k := &kashiDB{conn: conn}
	if err := k.inspect(); err != nil {
		conn.Close()
		return nil, err
	}
	return k, nil
}

func (k *kashiDB) inspect() error {
	if err := k.conn.Ping(); err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	if ok, err := k.hasColumn("products", "barcode"); err != nil || !ok {
		return fmt.Errorf("this does not look like a Kashi database (no products table)")
	}
	if ok, _ := k.hasColumn("products", "brand"); !ok {
		return fmt.Errorf("this database is from a Kashi version older than 0.8; update the POS first")
	}
	k.listPricing, _ = k.hasColumn("price_lists", "is_default")
	if !k.listPricing {
		return nil
	}
	// No active default list (e.g. a till kashi never sent one): prices are
	// then only saved on the product row.
	err := k.conn.QueryRow(`SELECT id, name FROM price_lists WHERE is_default = 1 AND is_active = 1 ORDER BY id LIMIT 1`).
		Scan(&k.priceListID, &k.priceListName)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	err = k.conn.QueryRow(`SELECT id, name FROM discount_lists WHERE is_default = 1 AND is_active = 1 ORDER BY id LIMIT 1`).
		Scan(&k.discountListID, &k.discountName)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

func (k *kashiDB) hasColumn(table, column string) (bool, error) {
	var n int
	err := k.conn.QueryRow(`SELECT count(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&n)
	return n > 0, err
}

func (k *kashiDB) distinct(column string) []string {
	// column is one of a fixed set chosen in databaseInfo, never user input.
	rows, err := k.conn.Query(`SELECT DISTINCT ` + column + ` FROM products WHERE trim(` + column + `) <> '' ORDER BY 1 LIMIT 500`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if rows.Scan(&v) == nil {
			out = append(out, v)
		}
	}
	return out
}

func databaseInfo(dbPath string) (DatabaseInfo, error) {
	k, err := openKashiDB(dbPath)
	if err != nil {
		return DatabaseInfo{}, err
	}
	defer k.conn.Close()
	info := DatabaseInfo{
		Brands: k.distinct("brand"), Families: k.distinct("family"), Kinds: k.distinct("kind"),
		Seasons: k.distinct("season"), Colors: k.distinct("color"), Sizes: k.distinct("size"),
		PriceList: k.priceListName, DiscountList: k.discountName, ListPricing: k.listPricing,
	}
	info.Connected = k.connected()
	return info, nil
}

func (in ProductInput) normalize() (product, error) {
	for _, s := range []*string{&in.Barcode, &in.Code, &in.Name, &in.Description, &in.Color, &in.Size,
		&in.Brand, &in.Family, &in.Kind, &in.Season, &in.Price} {
		*s = strings.TrimSpace(*s)
	}
	p := product{ProductInput: in}
	switch {
	case in.Name == "":
		return p, fmt.Errorf("name is required")
	case in.Code == "":
		return p, fmt.Errorf("code is required")
	case in.Price == "":
		return p, fmt.Errorf("price is required")
	case in.Discount < 0 || in.Discount > 100:
		return p, fmt.Errorf("discount must be between 0 and 100")
	case in.Year < 0:
		return p, fmt.Errorf("year cannot be negative")
	}
	price, err := strconv.ParseFloat(strings.ReplaceAll(in.Price, ",", ""), 64)
	if err != nil || math.IsNaN(price) || price > math.MaxInt64/100 {
		return p, fmt.Errorf("price must be a number, like 9.99")
	}
	p.PriceCents = int64(math.Round(price * 100))
	if p.PriceCents <= 0 {
		// The till refuses to sell anything priced at 0.
		return p, fmt.Errorf("price must be greater than 0")
	}
	// Any barcode is accepted, as in the POS: stores carry UPC-A codes and
	// in-house labels as well as EAN-13.
	return p, nil
}

func checkProduct(dbPath string, in ProductInput) error {
	p, err := in.normalize()
	if err != nil {
		return err
	}
	k, err := openKashiDB(dbPath)
	if err != nil {
		return err
	}
	defer k.conn.Close()
	if p.Barcode != "" {
		exists, err := barcodeExists(context.Background(), k.conn, p.Barcode)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("barcode %s already belongs to another product", p.Barcode)
		}
	}
	return nil
}

func importProducts(dbPath string, items []ProductInput) (ImportResult, error) {
	k, err := openKashiDB(dbPath)
	if err != nil {
		return ImportResult{}, err
	}
	defer k.conn.Close()

	ctx := context.Background()
	result := ImportResult{Rows: make([]RowResult, 0, len(items))}
	seen := map[string]bool{}
	for i, in := range items {
		row := RowResult{Index: i, Barcode: strings.TrimSpace(in.Barcode)}
		barcode, err := k.insert(ctx, in, seen)
		if err != nil {
			row.Status, row.Message = "error", err.Error()
			result.Failed++
		} else {
			row.Status, row.Barcode = "imported", barcode
			seen[barcode] = true
			result.Imported++
		}
		result.Rows = append(result.Rows, row)
	}
	return result, nil
}

// insert adds one product and, on list-priced databases, its price (and
// discount) to the default lists, all in one transaction.
func (k *kashiDB) insert(ctx context.Context, in ProductInput, seen map[string]bool) (string, error) {
	p, err := in.normalize()
	if err != nil {
		return "", err
	}
	if p.Barcode != "" && seen[p.Barcode] {
		return "", fmt.Errorf("barcode %s is used twice in this list", p.Barcode)
	}

	tx, err := k.conn.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	barcode := p.Barcode
	if barcode == "" {
		if barcode, err = randomUnusedBarcode(ctx, tx); err != nil {
			return "", fmt.Errorf("generate barcode: %w", err)
		}
	} else if exists, err := barcodeExists(ctx, tx, barcode); err != nil {
		return "", err
	} else if exists {
		return "", fmt.Errorf("barcode %s already belongs to another product", barcode)
	}

	res, err := tx.ExecContext(ctx, `INSERT INTO products
		(barcode, code, name, description, is_active, price, discount, color, size, currency_code, brand, family, kind, season, product_year)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'USD', ?, ?, ?, ?, ?)`,
		barcode, p.Code, p.Name, p.Description, boolInt(p.IsActive), p.PriceCents, p.Discount,
		p.Color, p.Size, p.Brand, p.Family, p.Kind, p.Season, p.Year)
	if err != nil {
		return "", err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return "", err
	}
	if p.QuickAccess {
		if _, err := tx.ExecContext(ctx, `INSERT INTO quick_access_items (product_id, position)
			VALUES (?, COALESCE((SELECT MAX(position) + 1 FROM quick_access_items), 0))
			ON CONFLICT (product_id) DO NOTHING`, id); err != nil {
			return "", fmt.Errorf("add to quick access: %w", err)
		}
	}

	if k.priceListID != 0 || k.discountListID != 0 {
		// The uid is minted by the products_mint_uid AFTER INSERT trigger.
		var uid sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT uid FROM products WHERE id = ?`, id).Scan(&uid); err != nil {
			return "", err
		}
		if !uid.Valid || uid.String == "" {
			return "", fmt.Errorf("the database did not assign the product a uid")
		}
		if k.priceListID != 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO price_list_items (price_list_id, product_uid, price) VALUES (?, ?, ?)
				ON CONFLICT (price_list_id, product_uid) DO UPDATE SET price = excluded.price`,
				k.priceListID, uid.String, p.PriceCents); err != nil {
				return "", fmt.Errorf("add to price list: %w", err)
			}
		}
		if p.Discount > 0 && k.discountListID != 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO discount_list_items (discount_list_id, product_uid, discount) VALUES (?, ?, ?)
				ON CONFLICT (discount_list_id, product_uid) DO UPDATE SET discount = excluded.discount`,
				k.discountListID, uid.String, p.Discount); err != nil {
				return "", fmt.Errorf("add to discount list: %w", err)
			}
		}
	}
	return barcode, tx.Commit()
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

type rowQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func barcodeExists(ctx context.Context, q rowQueryer, barcode string) (bool, error) {
	var id int64
	err := q.QueryRowContext(ctx, `SELECT id FROM products WHERE barcode = ?`, barcode).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func randomUnusedBarcode(ctx context.Context, q rowQueryer) (string, error) {
	for attempt := 0; attempt < 50; attempt++ {
		digits := make([]byte, 12)
		for i := range digits {
			n, err := rand.Int(rand.Reader, big.NewInt(10))
			if err != nil {
				return "", err
			}
			digits[i] = byte('0' + n.Int64())
		}
		base := string(digits)
		check, err := ean13CheckDigit(base)
		if err != nil {
			return "", err
		}
		barcode := base + strconv.Itoa(check)
		exists, err := barcodeExists(ctx, q, barcode)
		if err != nil {
			return "", err
		}
		if !exists {
			return barcode, nil
		}
	}
	return "", fmt.Errorf("could not generate a unique barcode after 50 attempts")
}

func ean13CheckDigit(base string) (int, error) {
	if len(base) != 12 {
		return 0, fmt.Errorf("base must be 12 digits")
	}
	sum := 0
	for i, r := range base {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("base must be numeric")
		}
		d := int(r - '0')
		if i%2 == 0 {
			sum += d
		} else {
			sum += d * 3
		}
	}
	return (10 - (sum % 10)) % 10, nil
}

// QuickItem is one product as shown in the quick-access editor. CardColor and
// CardTextColor are the POS's per-card colours (#rrggbb, "" = default /
// automatic); Color is the product's own colour attribute.
type QuickItem struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Code          string `json:"code"`
	Barcode       string `json:"barcode"`
	Color         string `json:"color"`
	Size          string `json:"size"`
	IsActive      bool   `json:"isActive"`
	CardColor     string `json:"cardColor"`
	CardTextColor string `json:"cardTextColor"`
}

// QuickAccessData is the list plus which colour columns this database has
// (000105 added the card colour, 000106 the text colour).
type QuickAccessData struct {
	Items      []QuickItem `json:"items"`
	CardColors bool        `json:"cardColors"`
	TextColors bool        `json:"textColors"`
}

var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (k *kashiDB) quickColorSupport() (card, text bool) {
	card, _ = k.hasColumn("quick_access_items", "color")
	text, _ = k.hasColumn("quick_access_items", "text_color")
	return card, text
}

func scanQuickItems(rows *sql.Rows) ([]QuickItem, error) {
	defer rows.Close()
	items := []QuickItem{}
	for rows.Next() {
		var it QuickItem
		var active int64
		if err := rows.Scan(&it.ID, &it.Name, &it.Code, &it.Barcode, &it.Color, &it.Size, &active, &it.CardColor, &it.CardTextColor); err != nil {
			return nil, err
		}
		it.IsActive = active != 0
		items = append(items, it)
	}
	return items, rows.Err()
}

// quickAccessList returns the list in the POS's order. Inactive products are
// included so they can be removed; the POS itself hides them.
func quickAccessList(dbPath string) (QuickAccessData, error) {
	k, err := openKashiDB(dbPath)
	if err != nil {
		return QuickAccessData{}, err
	}
	defer k.conn.Close()
	data := QuickAccessData{}
	data.CardColors, data.TextColors = k.quickColorSupport()
	cardExpr, textExpr := "''", "''"
	if data.CardColors {
		cardExpr = "q.color"
	}
	if data.TextColors {
		textExpr = "q.text_color"
	}
	rows, err := k.conn.Query(`SELECT p.id, p.name, p.code, p.barcode, p.color, p.size, p.is_active, ` + cardExpr + `, ` + textExpr + `
		FROM quick_access_items q JOIN products p ON p.id = q.product_id
		ORDER BY q.position, q.created_at, q.product_id`)
	if err != nil {
		return data, err
	}
	data.Items, err = scanQuickItems(rows)
	return data, err
}

func searchProducts(dbPath, query string) ([]QuickItem, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []QuickItem{}, nil
	}
	k, err := openKashiDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer k.conn.Close()
	like := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query) + "%"
	rows, err := k.conn.Query(`SELECT id, name, code, barcode, color, size, is_active, '', '' FROM products
		WHERE is_active = 1 AND (name LIKE ?1 ESCAPE '\' OR code LIKE ?1 ESCAPE '\' OR barcode LIKE ?1 ESCAPE '\')
		ORDER BY barcode = ?2 DESC, code = ?2 DESC, name, code, color, size LIMIT 50`, like, query)
	if err != nil {
		return nil, err
	}
	return scanQuickItems(rows)
}

// saveQuickAccess replaces the whole quick-access list with items, in order,
// including each card's colours where the database supports them.
func saveQuickAccess(dbPath string, items []QuickItem) error {
	for _, it := range items {
		for _, c := range []string{it.CardColor, it.CardTextColor} {
			if c != "" && !hexColorPattern.MatchString(c) {
				return fmt.Errorf("invalid color %q", c)
			}
		}
	}
	k, err := openKashiDB(dbPath)
	if err != nil {
		return err
	}
	defer k.conn.Close()
	cardColors, textColors := k.quickColorSupport()
	tx, err := k.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM quick_access_items`); err != nil {
		return err
	}
	for i, it := range items {
		res, err := tx.Exec(`INSERT INTO quick_access_items (product_id, position)
			SELECT id, ? FROM products WHERE id = ? ON CONFLICT (product_id) DO NOTHING`, i, it.ID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("product %d no longer exists; reload the list", it.ID)
		}
		if cardColors {
			if _, err := tx.Exec(`UPDATE quick_access_items SET color = ? WHERE product_id = ?`, strings.ToLower(it.CardColor), it.ID); err != nil {
				return err
			}
		}
		if textColors {
			if _, err := tx.Exec(`UPDATE quick_access_items SET text_color = ? WHERE product_id = ?`, strings.ToLower(it.CardTextColor), it.ID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
