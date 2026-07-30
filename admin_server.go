package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
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

type adminConfig struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
	Token   string `json:"token"`
}

type adminProductInput struct {
	Barcode, Code, Name, Description string
	IsActive, Price, Discount        int64
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
			value, err = a.app.store.CreateProduct(ctx, db.CreateProductParams{
				Barcode: input.Barcode, Code: input.Code, Name: input.Name,
				Description: input.Description, IsActive: input.IsActive,
				Price: input.Price, Discount: input.Discount,
			})
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
	return a.app.store.UpdateProduct(ctx, db.UpdateProductParams{
		ID: id, Barcode: input.Barcode, Code: input.Code, Name: input.Name,
		Description: input.Description, IsActive: input.IsActive,
		Price: input.Price, Discount: input.Discount,
	})
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
