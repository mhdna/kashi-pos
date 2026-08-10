package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	db "kashi-pos/db/sqlc"
)

var supportedSocialPlatforms = []string{"facebook", "tiktok", "instagram", "threads", "youtube"}

//go:embed db/kashi.db
var embeddedDB []byte

type App struct {
	ctx        context.Context
	store      *db.Store
	conn       *sql.DB
	dbPath     string
	admin      *adminServer
	startupErr error
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	dbPath, err := applicationDatabasePath()
	if err != nil {
		a.failStartup("resolve database path", err)
		return
	}
	log.Println("database path:", dbPath)
	a.dbPath = dbPath

	databaseExisted := false
	if _, err := os.Stat(dbPath); err == nil {
		databaseExisted = true
	} else if !os.IsNotExist(err) {
		a.failStartup("inspect database", err)
		return
	}
	if !databaseExisted {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			a.failStartup("create database directory", err)
			return
		}
		if err := os.WriteFile(dbPath, embeddedDB, 0600); err != nil {
			a.failStartup("create database", err)
			return
		}
	}

	a.conn, err = sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		a.failStartup("open database", err)
		return
	}
	if err := a.conn.PingContext(ctx); err != nil {
		a.failStartup("ping database", err)
		return
	}
	if err := applyMigrations(ctx, a.conn, dbPath, databaseExisted); err != nil {
		a.failStartup("migrate database", err)
		return
	}

	a.store = db.NewStore(a.conn)

	a.seedDefaults()
	if err := a.applyEnvironmentDefaults(); err != nil {
		log.Println("apply environment setting defaults:", err)
	}
	if err := a.closeStaleOpenShifts(); err != nil {
		log.Println("close stale open shifts:", err)
	}
	a.startAdminServer()
}

func applicationDatabasePath() (string, error) {
	if configured := os.Getenv("KASHI_DB_PATH"); configured != "" {
		return configured, nil
	}
	dataDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "kashi-pos", "database"), nil
}

func (a *App) failStartup(operation string, err error) {
	a.startupErr = fmt.Errorf("%s: %w", operation, err)
	log.Printf("backend startup failed: %v", a.startupErr)
	if a.conn != nil {
		_ = a.conn.Close()
		a.conn = nil
	}
	a.store = nil
}

// GetStartupError lets the frontend display an actionable failure instead of
// presenting a partially initialized application.
func (a *App) GetStartupError() string {
	if a.startupErr == nil {
		return ""
	}
	return a.startupErr.Error()
}

func (a *App) shutdown(ctx context.Context) {
	a.stopAdminServer(ctx)
	if a.conn != nil {
		a.conn.Close()
	}
}

func (a *App) VerifyLogin(username, password string) (map[string]interface{}, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	user, err := a.store.GetUserByUsername(a.ctx, username)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	if user.PasswordHash != hashPassword(password) {
		return nil, fmt.Errorf("wrong password")
	}
	if user.Active == 0 {
		return nil, fmt.Errorf("account disabled")
	}

	role, err := a.store.GetRoleByID(a.ctx, user.RoleID)
	if err != nil {
		return nil, fmt.Errorf("role not found")
	}
	if err := a.closeStaleOpenShifts(); err != nil {
		log.Printf("close stale open shifts after login: %v", err)
	}

	return map[string]interface{}{
		"id":       user.ID,
		"username": user.Username,
		"role":     role.Name,
	}, nil
}

func (a *App) ChangePassword(username, oldPassword, newPassword string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	user, err := a.store.GetUserByUsername(a.ctx, username)
	if err != nil {
		return fmt.Errorf("user not found")
	}
	if user.PasswordHash != hashPassword(oldPassword) {
		return fmt.Errorf("wrong password")
	}
	_, err = a.store.UpdateUser(a.ctx, db.UpdateUserParams{
		Username:     user.Username,
		PasswordHash: hashPassword(newPassword),
		RoleID:       user.RoleID,
		Active:       user.Active,
		ID:           user.ID,
	})
	return err
}

func (a *App) seedDefaults() {
	if a.store == nil {
		return
	}

	ctx := context.Background()

	roles, err := a.store.ListRoles(ctx)
	if err != nil || len(roles) > 0 {
		return
	}

	adminRole, err := a.store.CreateRole(ctx, db.CreateRoleParams{
		Name:        "admin",
		Description: "Administrator",
	})
	if err != nil {
		log.Println("create admin role:", err)
		return
	}

	cashierRole, err := a.store.CreateRole(ctx, db.CreateRoleParams{
		Name:        "cashier",
		Description: "Cashier",
	})
	if err != nil {
		log.Println("create cashier role:", err)
		return
	}

	_, err = a.store.CreateUser(ctx, db.CreateUserParams{
		Username:     "admin",
		PasswordHash: hashPassword("admin"),
		RoleID:       adminRole.ID,
		Active:       1,
	})
	if err != nil {
		log.Println("create default admin:", err)
		return
	}
	_, err = a.store.CreateUser(ctx, db.CreateUserParams{
		Username:     "user",
		PasswordHash: hashPassword("user"),
		RoleID:       cashierRole.ID,
		Active:       1,
	})
	if err != nil {
		log.Println("create default user:", err)
		return
	}

	log.Println("seeded default roles and admin user")
	_, err = a.store.GetSettings(ctx)
	if err != nil {
		if err := a.store.UpsertSettings(ctx, defaultSettingsParams()); err != nil {
			log.Println("upsert settings:", err)
		}
	}
	a.seedCashboxAccounts()
	a.seedSalespersons()
	a.seedDefaults2()
}

func (a *App) seedSalespersons() {
	existing, _ := a.store.ListSalespersons(context.Background())
	if len(existing) > 0 {
		return
	}
	for _, name := range []string{"Sarah M", "Fatima G", "Ali A", "Zahraa M", "Alex A"} {
		if _, err := a.store.CreateSalesperson(context.Background(), name); err != nil {
			log.Println("seed salesperson:", err)
		}
	}
	log.Println("seeded salespersons")
}

func (a *App) seedCashboxAccounts() {
	existing, _ := a.store.ListCashboxAccounts(context.Background())
	for _, acc := range []struct{ name, currency string }{
		{"USD", "USD"},
		{"LBP", "LBP"},
		{"Visa", "USD"},
		{"Whish", "USD"},
		{"On Account", "USD"},
		{"On Coupon", "USD"},
	} {
		found := false
		for _, current := range existing {
			if current.Name == acc.name {
				found = true
				break
			}
		}
		if found {
			continue
		}
		if _, err := a.store.CreateCashboxAccount(context.Background(), db.CreateCashboxAccountParams{
			Name:         acc.name,
			CurrencyCode: acc.currency,
		}); err != nil {
			log.Println("seed cashbox account:", err)
		}
	}
	log.Println("seeded cashbox accounts")
}

func hashPassword(pw string) string {
	h := sha256.Sum256([]byte(pw))
	return fmt.Sprintf("%x", h)
}

func (a *App) GetTheme() (string, error) {
	if a.store == nil {
		return "light", nil
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return "light", nil
	}
	return settings.Theme, nil
}

func (a *App) SetTheme(theme string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	err := a.store.UpdateTheme(a.ctx, theme)
	if err != nil {
		return err
	}
	return nil
}

func (a *App) GetAppSettings() (map[string]interface{}, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return nil, err
	}
	result := map[string]interface{}{
		"branchName":          settings.BranchName,
		"branchCode":          settings.BranchCode,
		"apiKey":              settings.ApiKey,
		"printerID":           settings.PrinterID,
		"printerSize":         settings.PrinterSize,
		"receiptPrinter":      settings.ReceiptPrinter,
		"screenPort":          settings.ScreenPort,
		"receiptWidth":        settings.ReceiptWidth,
		"receiptHeight":       settings.ReceiptHeight,
		"taxRate":             settings.TaxRate,
		"roundingMode":        settings.RoundingMode,
		"roundingCurrency":    normalizedRoundingCurrency(settings.RoundingCurrency),
		"receiptEnabled":      settings.ReceiptEnabled != 0,
		"salesInventoryId":    settings.SalesInventoryID,
		"exchangeRate":        settings.ExchangeRate,
		"maxOverpaymentCents": settings.MaxOverpaymentCents,
		"marketName":          settings.MarketName,
		"marketPhone":         settings.MarketPhone,
		"branchPhone":         settings.BranchPhone,
		"counterNumber":       settings.CounterNumber,
		"registerName":        settings.CounterNumber,
		"marketDescription":   settings.MarketDescription,
		"returnPolicy":        settings.ReturnPolicy,
		"website":             settings.Website,
		"instagram":           settings.Instagram,
		"socialPlatforms":     decodeSocialPlatforms(settings.SocialPlatforms),
		"socialHandles":       decodeSocialHandles(settings.SocialHandles),
		"receiptFont":         settings.ReceiptFont,
		"receiptTitleSize":    settings.ReceiptTitleSize,
		"receiptBodySize":     settings.ReceiptBodySize,
		"receiptBodyFont":     settings.ReceiptBodyFont,
		"receiptCutoff":       settings.ReceiptCutoff != 0,
	}
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, autoCloseStaleShiftsFlag); flagErr == nil {
		result["autoCloseStaleShiftsEnabled"] = enabled != 0
	} else {
		result["autoCloseStaleShiftsEnabled"] = false
	}
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, customItemDiscountsFlag); flagErr == nil {
		result["customItemDiscountsEnabled"] = enabled != 0
	} else {
		result["customItemDiscountsEnabled"] = false
	}
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, customItemPricesFlag); flagErr == nil {
		result["customItemPricesEnabled"] = enabled != 0
	} else {
		result["customItemPricesEnabled"] = false
	}
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, perUnitItemPricesFlag); flagErr == nil {
		result["perUnitItemPricesEnabled"] = enabled != 0
	} else {
		result["perUnitItemPricesEnabled"] = false
	}
	if config, configErr := a.GetAttendanceConfig(); configErr == nil {
		result["attendance"] = config
	}
	return result, nil
}

func (a *App) SetCustomItemDiscountsEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.SetFeatureFlag(a.ctx, db.SetFeatureFlagParams{
		Key: customItemDiscountsFlag, Enabled: boolInt(enabled),
	})
}

func (a *App) SetCustomItemPricesEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.SetFeatureFlag(a.ctx, db.SetFeatureFlagParams{
		Key: customItemPricesFlag, Enabled: boolInt(enabled),
	})
}

func (a *App) SetPerUnitItemPricesEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.SetFeatureFlag(a.ctx, db.SetFeatureFlagParams{
		Key: perUnitItemPricesFlag, Enabled: boolInt(enabled),
	})
}

func (a *App) UpdateReceiptSettings(marketName, marketPhone, branchPhone, marketDescription, returnPolicy, website, instagram, socialPlatformsJSON, socialHandlesJSON, registerName, receiptFont, bodyFont string, titleSize, bodySize float64, cutoff bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	current, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return err
	}
	if titleSize < 8 || titleSize > 40 {
		titleSize = 21
	}
	if bodySize < 6 || bodySize > 24 {
		bodySize = 11
	}
	if receiptFont == "" {
		receiptFont = "Belleza"
	}
	if bodyFont == "" {
		bodyFont = "Printer Font A"
	}
	socialPlatforms, err := normalizeSocialPlatforms(socialPlatformsJSON)
	if err != nil {
		return err
	}
	socialHandles, err := normalizeSocialHandles(socialHandlesJSON, socialPlatforms)
	if err != nil {
		return err
	}
	socialPlatformsJSON, err = marshalJSONString(socialPlatforms)
	if err != nil {
		return err
	}
	socialHandlesJSON, err = marshalJSONString(socialHandles)
	if err != nil {
		return err
	}
	return a.store.UpdateSettings(a.ctx, db.UpdateSettingsParams{
		BranchName: current.BranchName, BranchCode: current.BranchCode, ApiKey: current.ApiKey, PrinterID: current.PrinterID,
		PrinterSize: current.PrinterSize, ReceiptPrinter: current.ReceiptPrinter, ScreenPort: current.ScreenPort,
		ReceiptWidth: current.ReceiptWidth, ReceiptHeight: current.ReceiptHeight, Theme: current.Theme, TaxRate: current.TaxRate,
		RoundingMode: current.RoundingMode, RoundingCurrency: current.RoundingCurrency, ReceiptEnabled: current.ReceiptEnabled, SalesInventoryID: current.SalesInventoryID,
		ExchangeRate: current.ExchangeRate, MaxOverpaymentCents: current.MaxOverpaymentCents, MarketName: marketName, MarketPhone: marketPhone, BranchPhone: branchPhone, MarketDescription: marketDescription,
		ReturnPolicy: returnPolicy, Website: website, Instagram: instagram, SocialPlatforms: socialPlatformsJSON, SocialHandles: socialHandlesJSON, ReceiptFont: receiptFont, ReceiptTitleSize: titleSize,
		ReceiptBodySize: bodySize, ReceiptCutoff: boolInt(cutoff), ReceiptBodyFont: bodyFont, CounterNumber: strings.TrimSpace(registerName),
	})
}

func (a *App) UpdateSettings(branchName, branchCode, apiKey, printerID, printerSize, receiptPrinter, screenPort string, receiptWidth, receiptHeight int64, taxRate float64, roundingMode, roundingCurrency string, receiptEnabled bool, salesInventoryID, exchangeRate, maxOverpaymentCents int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	current, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return err
	}
	if math.IsNaN(taxRate) || math.IsInf(taxRate, 0) || taxRate < 0 || taxRate > 1 {
		return fmt.Errorf("tax rate must be between 0 and 1")
	}
	if roundingMode, err = normalizeRoundingMode(roundingMode); err != nil {
		return err
	}
	roundingCurrency = normalizedRoundingCurrency(roundingCurrency)
	if exchangeRate <= 0 {
		return fmt.Errorf("exchange rate must be greater than zero")
	}
	if maxOverpaymentCents < 0 {
		return fmt.Errorf("maximum overpayment cannot be negative")
	}
	if branchCode, err = normalizeBranchCode(branchCode); err != nil {
		return err
	}
	if salesInventoryID > 0 {
		if _, err := a.store.GetInventoryByID(a.ctx, salesInventoryID); err != nil {
			return fmt.Errorf("sales inventory: %w", err)
		}
	}
	return a.store.UpdateSettings(a.ctx, db.UpdateSettingsParams{
		BranchName:       branchName,
		BranchCode:       branchCode,
		ApiKey:           apiKey,
		PrinterID:        printerID,
		PrinterSize:      printerSize,
		ReceiptPrinter:   receiptPrinter,
		ScreenPort:       screenPort,
		ReceiptWidth:     receiptWidth,
		ReceiptHeight:    receiptHeight,
		Theme:            current.Theme,
		TaxRate:          taxRate,
		RoundingMode:     roundingMode,
		RoundingCurrency: roundingCurrency,
		ReceiptEnabled:   boolInt(receiptEnabled),
		SalesInventoryID: salesInventoryID,
		ExchangeRate:     exchangeRate, MaxOverpaymentCents: maxOverpaymentCents,
		MarketName: current.MarketName, MarketPhone: current.MarketPhone, MarketDescription: current.MarketDescription,
		ReturnPolicy: current.ReturnPolicy, Website: current.Website, Instagram: current.Instagram, SocialPlatforms: current.SocialPlatforms, SocialHandles: current.SocialHandles, ReceiptFont: current.ReceiptFont,
		ReceiptTitleSize: current.ReceiptTitleSize, ReceiptBodySize: current.ReceiptBodySize, ReceiptCutoff: current.ReceiptCutoff,
		ReceiptBodyFont: current.ReceiptBodyFont, BranchPhone: current.BranchPhone, CounterNumber: current.CounterNumber,
	})
}

func (a *App) SetTaxRate(rate float64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	current, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return err
	}
	return a.store.UpdateSettings(a.ctx, db.UpdateSettingsParams{
		BranchName:       current.BranchName,
		BranchCode:       current.BranchCode,
		ApiKey:           current.ApiKey,
		PrinterID:        current.PrinterID,
		PrinterSize:      current.PrinterSize,
		ReceiptPrinter:   current.ReceiptPrinter,
		ScreenPort:       current.ScreenPort,
		ReceiptWidth:     current.ReceiptWidth,
		ReceiptHeight:    current.ReceiptHeight,
		Theme:            current.Theme,
		TaxRate:          rate,
		RoundingMode:     current.RoundingMode,
		RoundingCurrency: current.RoundingCurrency,
		ReceiptEnabled:   current.ReceiptEnabled,
		SalesInventoryID: current.SalesInventoryID,
		ExchangeRate:     current.ExchangeRate, MaxOverpaymentCents: current.MaxOverpaymentCents,
		MarketName: current.MarketName, MarketPhone: current.MarketPhone, MarketDescription: current.MarketDescription,
		ReturnPolicy: current.ReturnPolicy, Website: current.Website, Instagram: current.Instagram, SocialPlatforms: current.SocialPlatforms, SocialHandles: current.SocialHandles, ReceiptFont: current.ReceiptFont,
		ReceiptTitleSize: current.ReceiptTitleSize, ReceiptBodySize: current.ReceiptBodySize, ReceiptCutoff: current.ReceiptCutoff,
		ReceiptBodyFont: current.ReceiptBodyFont, BranchPhone: current.BranchPhone, CounterNumber: current.CounterNumber,
	})
}

func boolInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func marshalJSONString(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func decodeSocialPlatforms(raw string) []string {
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return normalizePlatformSlice(values)
}

func decodeSocialHandles(raw string) map[string]string {
	var values map[string]string
	if err := json.Unmarshal([]byte(raw), &values); err != nil || values == nil {
		return map[string]string{}
	}
	out := map[string]string{}
	for _, platform := range supportedSocialPlatforms {
		if handle := strings.TrimSpace(values[platform]); handle != "" {
			out[platform] = handle
		}
	}
	return out
}

func normalizeSocialPlatforms(raw string) ([]string, error) {
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, fmt.Errorf("invalid social platform selection")
	}
	return normalizePlatformSlice(values), nil
}

func normalizeSocialHandles(raw string, platforms []string) (map[string]string, error) {
	var values map[string]string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, fmt.Errorf("invalid social handles")
	}
	out := map[string]string{}
	for _, platform := range platforms {
		handle := strings.TrimSpace(values[platform])
		if handle != "" {
			out[platform] = handle
		}
	}
	return out, nil
}

func normalizePlatformSlice(values []string) []string {
	allowed := map[string]struct{}{}
	for _, platform := range supportedSocialPlatforms {
		allowed[platform] = struct{}{}
	}
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if _, ok := allowed[value]; !ok {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (a *App) seedDefaults2() {
	ctx := context.Background()

	shifts, _ := a.store.ListShifts(ctx)
	if len(shifts) == 0 {
		if _, err := a.store.CreateShift(ctx, 0); err != nil {
			log.Println("seed shift:", err)
		}
		log.Println("seeded default shift")
	}

	inventories, _ := a.store.ListInventories(ctx)
	if len(inventories) == 0 {
		if _, err := a.store.CreateInventory(ctx, "Default"); err != nil {
			log.Println("seed inventory:", err)
		}
		log.Println("seeded default inventory")
	}

	clients, _ := a.store.ListClients(ctx)
	if len(clients) == 0 {
		if _, err := a.store.CreateClient(ctx, db.CreateClientParams{
			Name:               "Walk-in",
			Phone:              "00000000",
			TotalLoyaltyPoints: 0,
			ValidLoyaltyPoints: 0,
		}); err != nil {
			log.Println("seed client:", err)
		}
		log.Println("seeded walk-in client")
	}
}
