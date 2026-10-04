package main

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	db "kashi-pos/db/sqlc"
)

// Product CSV export/import. The column set matches tools/product-importer,
// so a file exported here can be imported there and vice versa.
var productCSVHeader = []string{"Barcode", "Code", "Name", "Description", "Color", "Size", "Price",
	"Discount", "Brand", "Family", "Kind", "Season", "Year", "IsActive"}

const productCSVMaxProblems = 200

// ProductCSVProblem is a CSV line that won't be imported, and why.
type ProductCSVProblem struct {
	Line    int    `json:"line"`
	Barcode string `json:"barcode"`
	Message string `json:"message"`
}

// ProductCSVPreview summarises a CSV before anything is written.
type ProductCSVPreview struct {
	File     string              `json:"file"`
	Total    int                 `json:"total"`
	New      int                 `json:"new"`
	Existing int                 `json:"existing"`
	Invalid  int                 `json:"invalid"`
	Problems []ProductCSVProblem `json:"problems"`
}

// ProductCSVImportResult is what ImportPendingProductsCSV wrote.
type ProductCSVImportResult struct {
	Imported int                 `json:"imported"`
	Failed   int                 `json:"failed"`
	Problems []ProductCSVProblem `json:"problems"`
}

type productCSVRow struct {
	line   int
	params db.CreateProductParams
}

func (a *App) saveCSVDialog(title, defaultName string) (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           title,
		DefaultFilename: defaultName,
		Filters:         []runtime.FileFilter{{DisplayName: "CSV files (*.csv)", Pattern: "*.csv"}},
	})
}

func writeCSVFile(path string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err := w.WriteAll(rows); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func centsString(cents int64) string {
	return strconv.FormatFloat(float64(cents)/100, 'f', 2, 64)
}

// ExportProductsCSV writes every product, active or not, to a CSV the user
// picks. Returns "" if the user cancels.
func (a *App) ExportProductsCSV() (string, error) {
	if a.store == nil {
		return "", fmt.Errorf("database not initialized")
	}
	path, err := a.saveCSVDialog("Export products", "products-"+time.Now().Format("2006-01-02")+".csv")
	if err != nil || path == "" {
		return "", err
	}
	rows, err := a.productsCSVRows()
	if err != nil {
		return "", err
	}
	if err := writeCSVFile(path, rows); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

// productsCSVRows: Price and Discount are what the till charges, i.e. the
// active price and discount lists. A product no list prices has an empty
// Price, because the till can't sell it.
func (a *App) productsCSVRows() ([][]string, error) {
	pricing, err := loadActivePricing(a.ctx, a.store.Queries)
	if err != nil {
		return nil, err
	}
	rows := [][]string{productCSVHeader}
	for afterID := int64(0); ; {
		page, err := a.store.ListProductsForManagement(a.ctx, db.ListProductsForManagementParams{AfterID: afterID, Search: "", Limit: 1000})
		if err != nil {
			return nil, err
		}
		for _, p := range page {
			price, discount := "", int64(0)
			if listed, ok := pricing.priceFor(p.Uid.String); ok {
				price = centsString(listed.cents)
			}
			if listed, ok := pricing.discountFor(p.Uid.String); ok {
				discount = listed.percent
			}
			year := ""
			if p.ProductYear > 0 {
				year = strconv.FormatInt(p.ProductYear, 10)
			}
			rows = append(rows, []string{p.Barcode, p.Code, p.Name, p.Description, p.Color, p.Size, price,
				strconv.FormatInt(discount, 10), p.Brand, p.Family, p.Kind, p.Season, year, strconv.FormatInt(p.IsActive, 10)})
			afterID = p.ID
		}
		if len(page) < 1000 {
			return rows, nil
		}
	}
}

// ExportQuickAccessCSV writes the quick-access list, in order, with each
// card's colours. Returns "" if the user cancels.
func (a *App) ExportQuickAccessCSV() (string, error) {
	if a.store == nil {
		return "", fmt.Errorf("database not initialized")
	}
	path, err := a.saveCSVDialog("Export quick access", "quick-access-"+time.Now().Format("2006-01-02")+".csv")
	if err != nil || path == "" {
		return "", err
	}
	rows, err := a.quickAccessCSVRows()
	if err != nil {
		return "", err
	}
	if err := writeCSVFile(path, rows); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

func (a *App) quickAccessCSVRows() ([][]string, error) {
	products, err := a.store.ListQuickAccessProducts(a.ctx)
	if err != nil {
		return nil, err
	}
	colors, err := a.store.ListQuickAccessColors(a.ctx)
	if err != nil {
		return nil, err
	}
	colorByID := make(map[int64]db.ListQuickAccessColorsRow, len(colors))
	for _, c := range colors {
		colorByID[c.ProductID] = c
	}
	pricing, err := loadActivePricing(a.ctx, a.store.Queries)
	if err != nil {
		return nil, err
	}
	rows := [][]string{{"Position", "Barcode", "Code", "Name", "Color", "Size", "Price", "CardColor", "TextColor"}}
	for i, p := range products {
		price := ""
		if listed, ok := pricing.priceFor(p.Uid.String); ok {
			price = centsString(listed.cents)
		}
		c := colorByID[p.ID]
		rows = append(rows, []string{strconv.Itoa(i + 1), p.Barcode, p.Code, p.Name, p.Color, p.Size, price, c.Color, c.TextColor})
	}
	return rows, nil
}

var productCSVColumns = map[string]string{
	"barcode": "barcode", "code": "code", "name": "name", "description": "description",
	"color": "color", "colour": "color", "size": "size", "price": "price", "discount": "discount",
	"brand": "brand", "family": "family", "kind": "kind", "season": "season", "year": "year",
	"isactive": "isactive", "active": "isactive",
}

// parseProductsCSV reads a products CSV. Required columns: Barcode, Name,
// Price; Code defaults to the barcode, as in CreateProduct.
func parseProductsCSV(r io.Reader) ([]productCSVRow, []ProductCSVProblem, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	header, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("could not read the CSV header: %w", err)
	}
	index := map[string]int{}
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))
		key = strings.NewReplacer(" ", "", "_", "").Replace(key)
		if col, ok := productCSVColumns[key]; ok {
			index[col] = i
		}
	}
	for _, required := range []string{"barcode", "name", "price"} {
		if _, ok := index[required]; !ok {
			return nil, nil, fmt.Errorf("the CSV needs a %q column (required: Barcode, Name, Price)", required)
		}
	}

	var rows []productCSVRow
	var problems []ProductCSVProblem
	for line := 2; ; line++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			problems = append(problems, ProductCSVProblem{Line: line, Message: err.Error()})
			continue
		}
		if strings.TrimSpace(strings.Join(record, "")) == "" {
			continue
		}
		get := func(col string) string {
			i, ok := index[col]
			if !ok || i >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[i])
		}
		barcode := get("barcode")
		fail := func(msg string) {
			problems = append(problems, ProductCSVProblem{Line: line, Barcode: barcode, Message: msg})
		}
		p := db.CreateProductParams{
			Barcode: barcode, Code: get("code"), Name: get("name"), Description: get("description"),
			Color: get("color"), Size: get("size"), CurrencyCode: "USD", IsActive: 1,
			Brand: get("brand"), Family: get("family"), Kind: get("kind"), Season: get("season"),
		}
		if p.Code == "" {
			p.Code = p.Barcode
		}
		switch {
		case p.Barcode == "":
			fail("barcode is required")
			continue
		case p.Name == "":
			fail("name is required")
			continue
		}
		price, err := strconv.ParseFloat(strings.ReplaceAll(get("price"), ",", ""), 64)
		if err != nil || math.IsNaN(price) || price > math.MaxInt64/100 {
			fail("price must be a number, like 9.99")
			continue
		}
		if p.Price = int64(math.Round(price * 100)); p.Price <= 0 {
			// priceInvoiceProduct rejects a 0 price, so it could never be sold.
			fail("price must be greater than 0")
			continue
		}
		var ok bool
		if p.Discount, ok = optionalCSVInt(get("discount")); !ok || p.Discount < 0 || p.Discount > 100 {
			fail("discount must be a whole number between 0 and 100")
			continue
		}
		if p.ProductYear, ok = optionalCSVInt(get("year")); !ok || p.ProductYear < 0 {
			fail("year must be a whole number")
			continue
		}
		switch strings.ToLower(get("isactive")) {
		case "", "1", "true", "yes", "y":
		case "0", "false", "no", "n":
			p.IsActive = 0
		default:
			fail("isActive must be 1 or 0")
			continue
		}
		rows = append(rows, productCSVRow{line: line, params: p})
	}
	return rows, problems, nil
}

func optionalCSVInt(s string) (int64, bool) {
	if s == "" {
		return 0, true
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}

// previewProductsCSV keeps the rows whose barcode is new to this till (and
// not repeated earlier in the file).
func (a *App) previewProductsCSV(r io.Reader) (ProductCSVPreview, []productCSVRow, error) {
	rows, problems, err := parseProductsCSV(r)
	if err != nil {
		return ProductCSVPreview{}, nil, err
	}
	preview := ProductCSVPreview{Total: len(rows) + len(problems)}
	firstLine := map[string]int{}
	var fresh []productCSVRow
	for _, row := range rows {
		barcode := row.params.Barcode
		if line, dup := firstLine[barcode]; dup {
			problems = append(problems, ProductCSVProblem{Line: row.line, Barcode: barcode,
				Message: fmt.Sprintf("same barcode as line %d; only the first is imported", line)})
			continue
		}
		firstLine[barcode] = row.line
		if _, err := a.store.GetProductByBarcode(a.ctx, barcode); err == nil {
			preview.Existing++
			continue
		} else if !errors.Is(err, sql.ErrNoRows) {
			return ProductCSVPreview{}, nil, err
		}
		fresh = append(fresh, row)
	}
	preview.New = len(fresh)
	preview.Invalid = len(problems)
	if len(problems) > productCSVMaxProblems {
		problems = problems[:productCSVMaxProblems]
	}
	preview.Problems = problems
	return preview, fresh, nil
}

// PreviewProductsCSV asks for a CSV and reports which of its products are
// new to this till, by barcode. Nothing is written; the new rows are kept for
// ImportPendingProductsCSV. Standalone mode only, like CreateProduct. Returns
// an empty preview if the user cancels.
func (a *App) PreviewProductsCSV() (ProductCSVPreview, error) {
	if err := a.requireStandalone(); err != nil {
		return ProductCSVPreview{}, err
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Import products from CSV",
		Filters: []runtime.FileFilter{{DisplayName: "CSV files (*.csv)", Pattern: "*.csv"}},
	})
	if err != nil || path == "" {
		return ProductCSVPreview{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return ProductCSVPreview{}, err
	}
	defer f.Close()
	preview, rows, err := a.previewProductsCSV(f)
	a.csvMu.Lock()
	a.pendingProductsCSV = rows
	a.csvMu.Unlock()
	if err != nil {
		return ProductCSVPreview{}, err
	}
	preview.File = path
	return preview, nil
}

// ImportPendingProductsCSV adds the products found by the last
// PreviewProductsCSV. Each one also gets its price (and discount) in the
// active default list -- created if this standalone till has none -- since
// the till sells only at list prices (000097).
func (a *App) ImportPendingProductsCSV() (ProductCSVImportResult, error) {
	if err := a.requireStandalone(); err != nil {
		return ProductCSVImportResult{}, err
	}
	a.csvMu.Lock()
	rows := a.pendingProductsCSV
	a.pendingProductsCSV = nil
	a.csvMu.Unlock()
	if rows == nil {
		return ProductCSVImportResult{}, fmt.Errorf("choose the CSV file again")
	}

	needDiscounts := false
	for _, row := range rows {
		needDiscounts = needDiscounts || row.params.Discount > 0
	}
	var priceListID, discountListID int64
	err := a.store.ExecTx(a.ctx, func(q *db.Queries) error {
		var err error
		if priceListID, err = ensureDefaultPriceList(a, q); err != nil || !needDiscounts {
			return err
		}
		discountListID, err = ensureDefaultDiscountList(a, q)
		return err
	})
	if err != nil {
		return ProductCSVImportResult{}, err
	}

	result := ProductCSVImportResult{Problems: []ProductCSVProblem{}}
	for _, row := range rows {
		err := a.store.ExecTx(a.ctx, func(q *db.Queries) error {
			if _, err := q.GetProductByBarcode(a.ctx, row.params.Barcode); err == nil {
				return fmt.Errorf("barcode already belongs to another product")
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			created, err := q.CreateProduct(a.ctx, row.params)
			if err != nil {
				return err
			}
			// The uid is minted by an AFTER INSERT trigger, which the
			// INSERT's RETURNING row doesn't include.
			product, err := q.GetProductByID(a.ctx, created.ID)
			if err != nil {
				return err
			}
			if err := q.InsertPriceListItem(a.ctx, db.InsertPriceListItemParams{
				PriceListID: priceListID, ProductUid: product.Uid.String, Price: row.params.Price,
			}); err != nil {
				return err
			}
			if row.params.Discount == 0 {
				return nil
			}
			return q.InsertDiscountListItem(a.ctx, db.InsertDiscountListItemParams{
				DiscountListID: discountListID, ProductUid: product.Uid.String, Discount: row.params.Discount,
			})
		})
		if err != nil {
			result.Failed++
			if len(result.Problems) < productCSVMaxProblems {
				result.Problems = append(result.Problems, ProductCSVProblem{Line: row.line, Barcode: row.params.Barcode, Message: err.Error()})
			}
			continue
		}
		result.Imported++
	}
	return result, nil
}

// ensureDefaultPriceList returns the active default price list, creating one
// if there is none. Only reachable in standalone mode, where no kashi sync
// will ever send one.
func ensureDefaultPriceList(a *App, q *db.Queries) (int64, error) {
	lists, err := q.ListPriceLists(a.ctx)
	if err != nil {
		return 0, err
	}
	for _, l := range lists {
		if l.IsDefault == 1 && l.IsActive == 1 {
			return l.ID, nil
		}
	}
	created, err := q.UpsertPriceListByUID(a.ctx, db.UpsertPriceListByUIDParams{
		Uid: sql.NullString{String: uuid.NewString(), Valid: true}, Name: "Default", IsActive: 1, IsDefault: 1,
	})
	return created.ID, err
}

func ensureDefaultDiscountList(a *App, q *db.Queries) (int64, error) {
	lists, err := q.ListDiscountLists(a.ctx)
	if err != nil {
		return 0, err
	}
	for _, l := range lists {
		if l.IsDefault == 1 && l.IsActive == 1 {
			return l.ID, nil
		}
	}
	created, err := q.UpsertDiscountListByUID(a.ctx, db.UpsertDiscountListByUIDParams{
		Uid: sql.NullString{String: uuid.NewString(), Valid: true}, Name: "Default", IsActive: 1, IsDefault: 1,
	})
	return created.ID, err
}
