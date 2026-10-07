package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// CSVPreview summarises one or more CSV files against the chosen database
// before anything is written.
type CSVPreview struct {
	Files    []string    `json:"files"`
	Total    int         `json:"total"`
	New      int         `json:"new"`
	Existing int         `json:"existing"`
	Invalid  int         `json:"invalid"`
	Problems []RowResult `json:"problems"`
	Sample   []CSVRow    `json:"sample"`
}

// CSVRow is a parsed CSV line ready to import.
type CSVRow struct {
	File  string       `json:"file"`
	Line  int          `json:"line"`
	Input ProductInput `json:"input"`
}

type pendingCSV struct {
	dbPath string
	rows   []CSVRow
}

const maxProblemsShown = 200

var csvColumns = map[string]string{
	"barcode": "barcode", "code": "code", "name": "name", "description": "description",
	"color": "color", "colour": "color", "size": "size", "price": "price", "discount": "discount",
	"brand": "brand", "family": "family", "kind": "kind", "season": "season", "year": "year",
	"isactive": "isactive", "active": "isactive",
}

func parseCSVFile(path string) ([]CSVRow, []RowResult, error) {
	name := filepath.Base(path)
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	header, err := r.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("%s: could not read the CSV header: %w", name, err)
	}
	index := map[string]int{}
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))
		key = strings.NewReplacer(" ", "", "_", "").Replace(key)
		if col, ok := csvColumns[key]; ok {
			index[col] = i
		}
	}
	for _, required := range []string{"barcode", "name", "price"} {
		if _, ok := index[required]; !ok {
			return nil, nil, fmt.Errorf("%s: the CSV needs a %q column (required: Barcode, Name, Price)", name, required)
		}
	}

	var rows []CSVRow
	var problems []RowResult
	for line := 2; ; line++ {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			problems = append(problems, RowResult{File: name, Index: line, Status: "error", Message: err.Error()})
			continue
		}
		get := func(col string) string {
			i, ok := index[col]
			if !ok || i >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[i])
		}
		if strings.Join(record, "") == "" {
			continue
		}
		in := ProductInput{
			Barcode: get("barcode"), Code: get("code"), Name: get("name"), Description: get("description"),
			Color: get("color"), Size: get("size"), Brand: get("brand"), Family: get("family"),
			Kind: get("kind"), Season: get("season"), Price: get("price"), IsActive: true,
		}
		if in.Code == "" {
			in.Code = in.Barcode
		}
		fail := func(msg string) {
			problems = append(problems, RowResult{File: name, Index: line, Barcode: in.Barcode, Status: "error", Message: msg})
		}
		var ok bool
		if in.Discount, ok = optInt(get("discount")); !ok {
			fail("discount must be a whole number")
			continue
		}
		if in.Year, ok = optInt(get("year")); !ok {
			fail("year must be a whole number")
			continue
		}
		switch strings.ToLower(get("isactive")) {
		case "", "1", "true", "yes", "y":
		case "0", "false", "no", "n":
			in.IsActive = false
		default:
			fail("isActive must be 1 or 0")
			continue
		}
		rows = append(rows, CSVRow{File: name, Line: line, Input: in})
	}
	return rows, problems, nil
}

func optInt(s string) (int64, bool) {
	if s == "" {
		return 0, true
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}

// previewCSV keeps only rows whose barcode is not in the database yet (and
// not repeated earlier in the same or an earlier file), and validates them.
func previewCSV(dbPath string, paths []string) (CSVPreview, []CSVRow, error) {
	var rows []CSVRow
	var problems []RowResult
	order := map[string]int{}
	for i, path := range paths {
		r, p, err := parseCSVFile(path)
		if err != nil {
			return CSVPreview{}, nil, err
		}
		order[filepath.Base(path)] = i
		rows = append(rows, r...)
		problems = append(problems, p...)
	}
	k, err := openKashiDB(dbPath)
	if err != nil {
		return CSVPreview{}, nil, err
	}
	defer k.conn.Close()

	existing := map[string]bool{}
	dbRows, err := k.conn.Query(`SELECT barcode FROM products`)
	if err != nil {
		return CSVPreview{}, nil, err
	}
	for dbRows.Next() {
		var b string
		if dbRows.Scan(&b) == nil {
			existing[b] = true
		}
	}
	dbRows.Close()

	preview := CSVPreview{Files: paths, Total: len(rows) + len(problems)}
	seen := map[string]CSVRow{}
	var fresh []CSVRow
	for _, row := range rows {
		p, err := row.Input.normalize()
		fail := func(barcode, msg string) {
			problems = append(problems, RowResult{File: row.File, Index: row.Line, Barcode: barcode, Status: "error", Message: msg})
		}
		switch {
		case err != nil:
			fail(row.Input.Barcode, err.Error())
		case p.Barcode == "":
			fail("", "barcode is required to match products")
		case existing[p.Barcode]:
			preview.Existing++
		case seen[p.Barcode].Line != 0:
			first := seen[p.Barcode]
			where := fmt.Sprintf("line %d", first.Line)
			if first.File != row.File {
				where += " of " + first.File
			}
			fail(p.Barcode, "same barcode as "+where+"; only the first is imported")
		default:
			seen[p.Barcode] = row
			fresh = append(fresh, row)
		}
	}
	sort.SliceStable(problems, func(i, j int) bool {
		a, b := problems[i], problems[j]
		if order[a.File] != order[b.File] {
			return order[a.File] < order[b.File]
		}
		return a.Index < b.Index
	})
	preview.New = len(fresh)
	preview.Invalid = len(problems)
	if len(problems) > maxProblemsShown {
		problems = problems[:maxProblemsShown]
	}
	preview.Problems = problems
	preview.Sample = fresh[:min(len(fresh), 50)]
	return preview, fresh, nil
}

// importCSVRows imports rows prepared by previewCSV. Barcodes are re-checked
// inside each insert, so anything added since the preview is skipped safely.
func importCSVRows(pending pendingCSV) (ImportResult, error) {
	k, err := openKashiDB(pending.dbPath)
	if err != nil {
		return ImportResult{}, err
	}
	defer k.conn.Close()
	ctx := context.Background()
	result := ImportResult{Rows: []RowResult{}}
	for _, row := range pending.rows {
		if _, err := k.insert(ctx, row.Input, nil); err != nil {
			result.Failed++
			if len(result.Rows) < maxProblemsShown {
				result.Rows = append(result.Rows, RowResult{File: row.File, Index: row.Line, Barcode: row.Input.Barcode, Status: "error", Message: err.Error()})
			}
			continue
		}
		result.Imported++
	}
	return result, nil
}
