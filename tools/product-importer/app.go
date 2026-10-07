package main

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct holds the Wails runtime context.
type App struct {
	ctx        context.Context
	pendingCSV *pendingCSV
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// SelectDatabase opens a native file picker for choosing the Kashi SQLite
// database file. Returns an empty string if the user cancels.
func (a *App) SelectDatabase() (string, error) {
	// No filter: the Kashi database is usually an extensionless file named
	// "database", which extension-based filters grey out on macOS.
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Kashi database file",
	})
}

// DatabaseInfo validates the database and returns dropdown values for the form.
func (a *App) DatabaseInfo(dbPath string) (DatabaseInfo, error) {
	return databaseInfo(dbPath)
}

// CheckProduct validates one product before it's added to the import list.
func (a *App) CheckProduct(dbPath string, in ProductInput) error {
	return checkProduct(dbPath, in)
}

// ConfirmImport asks the user, via a native dialog, to confirm writing to
// dbPath. JS confirm() is a no-op in the macOS webview, so this lives in Go.
func (a *App) ConfirmImport(dbPath string, count int) (bool, error) {
	choice, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "Import products?",
		Message:       fmt.Sprintf("Add %d product(s) to:\n%s\n\nThis writes directly to the database. Continue?", count, dbPath),
		Buttons:       []string{"Yes", "No"},
		DefaultButton: "No",
		CancelButton:  "No",
	})
	if err != nil {
		return false, err
	}
	return choice == "Yes", nil
}

// QuickAccessList returns the database's quick-access products in order, with
// their card colours.
func (a *App) QuickAccessList(dbPath string) (QuickAccessData, error) {
	return quickAccessList(dbPath)
}

// SearchProducts finds active products by name, code or barcode.
func (a *App) SearchProducts(dbPath string, query string) ([]QuickItem, error) {
	return searchProducts(dbPath, query)
}

// SaveQuickAccess replaces the quick-access list with items, in order.
func (a *App) SaveQuickAccess(dbPath string, items []QuickItem) error {
	return saveQuickAccess(dbPath, items)
}

// ImportProducts inserts each product and adds its price to the default
// price list. A failing product is reported and the rest still import.
func (a *App) ImportProducts(dbPath string, items []ProductInput) (ImportResult, error) {
	return importProducts(dbPath, items)
}

// PreviewCSVFile asks for one or more CSV files and reports which of their
// products are new to the database (by barcode). Nothing is written; the new
// rows are kept for ImportPendingCSV. Returns an empty preview if cancelled.
func (a *App) PreviewCSVFile(dbPath string) (CSVPreview, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select products CSV files",
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV files (*.csv)", Pattern: "*.csv"},
		},
	})
	if err != nil || len(paths) == 0 {
		return CSVPreview{}, err
	}
	preview, rows, err := previewCSV(dbPath, paths)
	if err != nil {
		a.pendingCSV = nil
		return CSVPreview{}, err
	}
	a.pendingCSV = &pendingCSV{dbPath: dbPath, rows: rows}
	return preview, nil
}

// ImportPendingCSV imports the new products found by the last PreviewCSVFile.
func (a *App) ImportPendingCSV(dbPath string) (ImportResult, error) {
	if a.pendingCSV == nil || a.pendingCSV.dbPath != dbPath {
		return ImportResult{}, fmt.Errorf("load the CSV again for this database")
	}
	pending := *a.pendingCSV
	a.pendingCSV = nil
	return importCSVRows(pending)
}

// PriceStatus counts products the POS would show at $0 (no list price).
func (a *App) PriceStatus(dbPath string) (PriceStatus, error) {
	return priceStatus(dbPath)
}

// FixPrices puts every unpriced product's own price into the default list.
func (a *App) FixPrices(dbPath string) (FixResult, error) {
	return fixPrices(dbPath)
}

// Confirm shows a native Yes/No dialog (JS confirm() is a no-op on macOS).
func (a *App) Confirm(title, message string) (bool, error) {
	choice, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type: runtime.QuestionDialog, Title: title, Message: message,
		Buttons: []string{"Yes", "No"}, DefaultButton: "No", CancelButton: "No",
	})
	if err != nil {
		return false, err
	}
	return choice == "Yes", nil
}
