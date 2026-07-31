package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	db "kashi-pos/db/sqlc"
)

type adminServer struct {
	server *http.Server
	app    *App
}

func (a *adminServer) backup(ctx context.Context) (map[string]any, error) {
	backupPath := fmt.Sprintf("%s.backup-%s", a.app.dbPath, time.Now().UTC().Format("20060102-150405.000"))
	if _, err := a.app.conn.ExecContext(ctx, `VACUUM INTO ?`, backupPath); err != nil {
		return nil, fmt.Errorf("create database backup: %w", err)
	}
	return map[string]any{"backup": backupPath}, nil
}

type adminConfig struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
	Token   string `json:"token"`
}

type adminProductInput struct {
	Barcode, Code, Name, Description, Color, Size, CurrencyCode string
	IsActive, Price, Discount                                   int64
	AddToInventory                                              bool
	InventoryName                                               string
	InventoryQuantity                                           int64
}

func adminBarcodeCheckDigit(base string) int {
	sum := 0
	for i, r := range base {
		v := int(r - '0')
		if i%2 == 1 {
			sum += v * 3
		} else {
			sum += v
		}
	}
	return (10 - sum%10) % 10
}

func (a *adminServer) newBarcode(ctx context.Context) (string, error) {
	for attempt := 0; attempt < 100; attempt++ {
		n, err := rand.Int(rand.Reader, big.NewInt(100000))
		if err != nil {
			return "", err
		}
		base := fmt.Sprintf("8000000%05d", n)
		code := base + fmt.Sprint(adminBarcodeCheckDigit(base))
		var exists int
		err = a.app.conn.QueryRowContext(ctx, `SELECT 1 FROM products WHERE barcode=? LIMIT 1`, code).Scan(&exists)
		if err == sql.ErrNoRows {
			return code, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("could not find an unused barcode")
}

func (a *adminServer) createProduct(ctx context.Context, input adminProductInput) (map[string]any, error) {
	if input.Barcode == "" {
		var err error
		input.Barcode, err = a.newBarcode(ctx)
		if err != nil {
			return nil, err
		}
	}
	if input.CurrencyCode == "" {
		input.CurrencyCode = "$"
	}
	tx, err := a.app.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO products(barcode,code,name,description,is_active,price,discount,color,size,currency_code) VALUES(?,?,?,?,?,?,?,?,?,?)`, input.Barcode, input.Code, input.Name, input.Description, input.IsActive, input.Price, input.Discount, input.Color, input.Size, input.CurrencyCode)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if input.AddToInventory {
		if input.InventoryQuantity < 1 {
			tx.Rollback()
			return nil, fmt.Errorf("inventory quantity must be at least 1")
		}
		name := input.InventoryName
		if name == "" {
			name = "Main Inventory"
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO inventories(name) VALUES(?) ON CONFLICT(name) DO NOTHING`, name); err != nil {
			tx.Rollback()
			return nil, err
		}
		var inventoryID int64
		if err = tx.QueryRowContext(ctx, `SELECT id FROM inventories WHERE name=?`, name).Scan(&inventoryID); err != nil {
			tx.Rollback()
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO inventory_items(inventory_id,product_id,quantity) VALUES(?,?,?) ON CONFLICT(inventory_id,product_id) DO UPDATE SET quantity=inventory_items.quantity+excluded.quantity`, inventoryID, id, input.InventoryQuantity); err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "barcode": input.Barcode}, nil
}

type adminOverrideInput struct {
	Name      string `json:"name"`
	ProductID int64  `json:"productId"`
	Price     int64  `json:"price"`
	Discount  int64  `json:"discount"`
}

func (a *App) startAdminServer() {
	if a.conn == nil {
		return
	}

	config := a.getAdminConfig()
	if !config.Enabled {
		return
	}
	listen, token := config.Listen, config.Token
	if token == "" && !strings.HasPrefix(listen, "127.0.0.1:") && !strings.HasPrefix(listen, "localhost:") && !strings.HasPrefix(listen, "[::1]:") {
		log.Printf("admin API disabled: KASHI_ADMIN_TOKEN is required when listening on %s", listen)
		return
	}
	admin := &adminServer{app: a}
	admin.server = &http.Server{
		Addr:              listen,
		Handler:           admin.auth(token),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
	}
	a.admin = admin
	go func() {
		log.Printf("admin API listening on %s", listen)
		if err := admin.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("admin API stopped: %v", err)
		}
	}()
}

func (a *App) getAdminConfig() adminConfig {
	config := adminConfig{Listen: "127.0.0.1:8787"}
	if path, err := adminConfigPath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, &config)
		}
	}
	if value := os.Getenv("KASHI_ADMIN_ENABLED"); value != "" {
		config.Enabled = value == "1" || strings.EqualFold(value, "true")
	}
	if value := os.Getenv("KASHI_ADMIN_LISTEN"); value != "" {
		config.Listen = value
	}
	if value := os.Getenv("KASHI_ADMIN_TOKEN"); value != "" {
		config.Token = value
	}
	if config.Listen == "" {
		config.Listen = "127.0.0.1:8787"
	}
	return config
}

func (a *App) GetAdminConfig() (map[string]any, error) {
	config := a.getAdminConfig()
	return map[string]any{"enabled": config.Enabled, "listen": config.Listen, "token": config.Token}, nil
}

func (a *App) UpdateAdminConfig(enabled bool, listen, token string) error {
	if listen == "" {
		listen = "127.0.0.1:8787"
	}
	if token == "" && !strings.HasPrefix(listen, "127.0.0.1:") && !strings.HasPrefix(listen, "localhost:") && !strings.HasPrefix(listen, "[::1]:") {
		return fmt.Errorf("an admin token is required for non-local listeners")
	}
	config := adminConfig{Enabled: enabled, Listen: listen, Token: token}
	path, err := adminConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	a.stopAdminServer(context.Background())
	if enabled {
		a.startAdminServer()
	}
	return nil
}

func adminConfigPath() (string, error) {
	dataDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "kashi-pos", "admin.json"), nil
}

func (a *App) stopAdminServer(ctx context.Context) {
	if a.admin != nil && a.admin.server != nil {
		if err := a.admin.server.Shutdown(ctx); err != nil {
			log.Printf("stop admin API: %v", err)
		}
	}
}

func (a *adminServer) auth(token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
			writeAdminError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		a.handle(w, r)
	})
}

func (a *adminServer) handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	path := strings.Trim(r.URL.Path, "/")
	var value any
	var err error

	switch {
	case r.Method == http.MethodGet && path == "health":
		value = map[string]any{"ok": true}
	case r.Method == http.MethodPost && path == "backup":
		value, err = a.backup(ctx)
	case r.Method == http.MethodGet && path == "products":
		value, err = a.products(ctx, r.URL.Query().Get("q"))
	case r.Method == http.MethodGet && path == "invoices":
		value, err = a.invoices(ctx, r.URL.Query().Get("date"))
	case r.Method == http.MethodGet && path == "daily-sales":
		value, err = a.dailySales(ctx, r.URL.Query().Get("date"))
	case r.Method == http.MethodGet && path == "salespersons":
		value, err = a.app.store.ListSalespersons(ctx)
	case r.Method == http.MethodGet && path == "settings":
		value, err = a.app.store.GetSettings(ctx)
	case r.Method == http.MethodPost && path == "products":
		var input adminProductInput
		err = decodeAdmin(r, &input)
		if err == nil {
			value, err = a.createProduct(ctx, input)
		}
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "products/"):
		value, err = a.updateProduct(ctx, r, strings.TrimPrefix(path, "products/"))
	case r.Method == http.MethodPost && path == "overrides/price":
		var input adminOverrideInput
		err = decodeAdmin(r, &input)
		if err == nil {
			value, err = a.app.store.CreatePriceOverride(ctx, db.CreatePriceOverrideParams{
				Name: input.Name, ProductID: sql.NullInt64{Int64: input.ProductID, Valid: input.ProductID != 0}, Price: input.Price,
			})
		}
	case r.Method == http.MethodPost && path == "overrides/discount":
		var input adminOverrideInput
		err = decodeAdmin(r, &input)
		if err == nil {
			value, err = a.app.store.CreateDiscountOverride(ctx, db.CreateDiscountOverrideParams{
				Name: input.Name, ProductID: sql.NullInt64{Int64: input.ProductID, Valid: input.ProductID != 0}, Discount: input.Discount,
			})
		}
	case r.Method == http.MethodPost && path == "salespersons":
		var input db.Salesperson
		err = decodeAdmin(r, &input)
		if err == nil {
			value, err = a.app.store.CreateSalesperson(ctx, input.FullName)
		}
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "salespersons/"):
		id, parseErr := strconv.ParseInt(strings.TrimPrefix(path, "salespersons/"), 10, 64)
		if parseErr != nil {
			err = parseErr
		} else {
			var input db.Salesperson
			err = decodeAdmin(r, &input)
			if err == nil {
				value, err = a.app.store.UpdateSalesperson(ctx, db.UpdateSalespersonParams{ID: id, FullName: input.FullName})
			}
		}
	case r.Method == http.MethodPut && path == "settings":
		var input map[string]any
		err = decodeAdmin(r, &input)
		if err == nil {
			err = a.updateSettings(ctx, input)
			value = map[string]any{"updated": true}
		}
	case r.Method == http.MethodPost && path == "reload":
		runtime.EventsEmit(a.app.ctx, "admin:reload")
		value = map[string]any{"reloaded": true, "method": "frontend-event"}
	default:
		writeAdminError(w, http.StatusNotFound, "not found")
		return
	}

	if err != nil {
		writeAdminError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func (a *adminServer) products(ctx context.Context, search string) ([]db.Product, error) {
	if search == "" {
		return a.app.store.ListProducts(ctx)
	}
	return a.app.store.ListProductsPage(ctx, db.ListProductsPageParams{Search: sql.NullString{String: search, Valid: true}, Limit: 200, Offset: 0})
}

func (a *adminServer) invoices(ctx context.Context, day string) ([]db.ListInvoicesPageRow, error) {
	items, err := a.app.store.ListInvoicesPage(ctx, db.ListInvoicesPageParams{Limit: 500, Offset: 0})
	if err != nil || day == "" {
		return items, err
	}
	filtered := make([]db.ListInvoicesPageRow, 0, len(items))
	for _, item := range items {
		if strings.HasPrefix(item.CreatedAt, day) {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (a *adminServer) dailySales(ctx context.Context, day string) (map[string]any, error) {
	if day == "" {
		day = time.Now().Format("2006-01-02")
	}
	var count, total int64
	err := a.app.conn.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(i.grand_total), 0) FROM invoices i JOIN sales_invoices s ON s.invoice_id=i.id WHERE date(i.created_at,'localtime')=?`, day).Scan(&count, &total)
	return map[string]any{"date": day, "invoiceCount": count, "total": total}, err
}

func (a *adminServer) updateProduct(ctx context.Context, r *http.Request, rawID string) (db.Product, error) {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return db.Product{}, err
	}
	var input adminProductInput
	if err := decodeAdmin(r, &input); err != nil {
		return db.Product{}, err
	}
	_, err = a.app.conn.ExecContext(ctx, `UPDATE products SET barcode=?,code=?,name=?,description=?,is_active=?,price=?,discount=?,color=?,size=?,currency_code=? WHERE id=?`, input.Barcode, input.Code, input.Name, input.Description, input.IsActive, input.Price, input.Discount, input.Color, input.Size, input.CurrencyCode, id)
	if err != nil {
		return db.Product{}, err
	}
	return a.app.store.GetProductByID(ctx, id)
}

func (a *adminServer) updateSettings(ctx context.Context, input map[string]any) error {
	allowed := map[string]bool{
		"branch_name": true, "branch_code": true, "api_key": true, "printer_id": true, "printer_size": true,
		"receipt_printer": true, "screen_port": true, "receipt_width": true, "receipt_height": true,
		"theme": true, "tax_rate": true, "rounding_mode": true, "receipt_enabled": true, "sales_inventory_id": true,
		"exchange_rate": true, "market_name": true, "market_phone": true, "market_description": true,
		"return_policy": true, "website": true, "instagram": true, "social_platforms": true, "social_handles": true,
		"receipt_font": true, "receipt_title_size": true, "receipt_body_size": true, "receipt_cutoff": true,
		"receipt_body_font": true, "branch_phone": true, "counter_number": true,
	}
	tx, err := a.app.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for key, value := range input {
		key = adminSettingColumn(key)
		if !allowed[key] {
			_ = tx.Rollback()
			return fmt.Errorf("unsupported setting: %s", key)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE settings SET `+key+`=?`, value); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func adminSettingColumn(key string) string {
	var out strings.Builder
	for _, char := range key {
		if char >= 'A' && char <= 'Z' {
			out.WriteByte('_')
			out.WriteByte(byte(char + ('a' - 'A')))
		} else {
			out.WriteRune(char)
		}
	}
	return out.String()
}

func decodeAdmin(r *http.Request, target any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func writeAdminError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func (a *adminServer) reloadCommand() error {
	command := os.Getenv("KASHI_ADMIN_RELOAD_COMMAND")
	if command == "" {
		return nil
	}
	return exec.Command("sh", "-c", command).Run()
}
