package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	db "kashi-pos/db/sqlc"
)

//go:embed db/kashi.db
var embeddedDB []byte

type App struct {
	ctx        context.Context
	store      *db.Store
	conn       *sql.DB
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
		if err := a.store.UpsertSettings(ctx, db.UpsertSettingsParams{
			BranchName:     "Main",
			BranchCode:     "CN",
			ApiKey:         "",
			PrinterID:      "",
			PrinterSize:    "",
			ReceiptPrinter: "",
			ScreenPort:     "",
			ReceiptWidth:   80,
			ReceiptHeight:  297,
			Theme:          "light",
			TaxRate:        0.0,
		}); err != nil {
			log.Println("upsert settings:", err)
		}
	}
	a.seedCashboxAccounts()
	a.seedProducts()
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
	if len(existing) > 0 {
		return
	}
	for _, acc := range []struct{ name, currency string }{
		{"USD", "USD"},
		{"LBP", "LBP"},
		{"Visa", "USD"},
		{"Whish", "USD"},
	} {
		if _, err := a.store.CreateCashboxAccount(context.Background(), db.CreateCashboxAccountParams{
			Name:         acc.name,
			CurrencyCode: acc.currency,
		}); err != nil {
			log.Println("seed cashbox account:", err)
		}
	}
	log.Println("seeded cashbox accounts")
}

func (a *App) seedProducts() {
	existing, _ := a.store.ListProducts(context.Background())
	if len(existing) > 0 {
		return
	}

	categories := []string{
		"t-shirt", "sweatshirt", "pants", "shoes", "hat", "coat", "jacket",
		"blazer", "dress", "formal", "formal-woman", "underwear", "tie",
		"bow-tie", "glasses", "sunglasses", "necklace", "ring", "watch",
		"wallet", "purse", "bag", "suitcase", "umbrella", "mask",
		"boxing-gloves", "scarf", "belt", "shorts", "skirt", "leggings",
		"socks", "sandals", "heels",
	}
	names := []string{
		"Classic", "Premium", "Slim", "Relaxed", "Vintage", "Modern", "Sport",
		"Casual", "Formal", "Essential", "Deluxe", "Urban", "Retro", "Elite",
		"Basic", "Signature", "Artisan", "Luxe", "Core", "Pro",
	}
	items := []string{
		"Tee", "Hoodie", "Pants", "Sneakers", "Fedora", "Overcoat", "Jacket",
		"Blazer", "Dress", "Suit", "Gown", "Boxers", "Necktie", "Bow Tie",
		"Glasses", "Sunglasses", "Necklace", "Ring", "Watch", "Wallet",
		"Purse", "Tote", "Suitcase", "Umbrella", "Mask", "Gloves",
		"Scarf", "Belt", "Shorts", "Skirt", "Leggings", "Socks", "Sandals", "Heels",
	}
	discounts := []int64{0, 0, 0, 0, 5, 5, 10, 10, 15, 20}

	for i := 1; i <= 100; i++ {
		barcode := fmt.Sprintf("%d", i)
		code := fmt.Sprintf("PRD-%03d", i)
		name := fmt.Sprintf("%s %s", names[rand.Intn(len(names))], items[rand.Intn(len(items))])
		category := categories[rand.Intn(len(categories))]
		price := 5.0 + rand.Float64()*195.0
		discount := discounts[rand.Intn(len(discounts))]

		_, err := a.store.CreateProduct(context.Background(), db.CreateProductParams{
			Barcode:     barcode,
			Code:        code,
			Name:        name,
			Description: category,
			IsActive:    1,
			Price:       int64(price * 100),
			Discount:    discount,
		})
		if err != nil {
			log.Println("seed product:", code, err)
		}
	}
	log.Println("seeded 100 products")
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
	return map[string]interface{}{
		"branchName":       settings.BranchName,
		"branchCode":       settings.BranchCode,
		"apiKey":           settings.ApiKey,
		"printerID":        settings.PrinterID,
		"printerSize":      settings.PrinterSize,
		"receiptPrinter":   settings.ReceiptPrinter,
		"screenPort":       settings.ScreenPort,
		"receiptWidth":     settings.ReceiptWidth,
		"receiptHeight":    settings.ReceiptHeight,
		"taxRate":          settings.TaxRate,
		"roundingMode":     settings.RoundingMode,
		"receiptEnabled":   settings.ReceiptEnabled != 0,
		"salesInventoryId": settings.SalesInventoryID,
	}, nil
}

func (a *App) UpdateSettings(branchName, branchCode, apiKey, printerID, printerSize, receiptPrinter, screenPort string, receiptWidth, receiptHeight int64, taxRate float64, roundingMode string, receiptEnabled bool, salesInventoryID int64) error {
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
		ReceiptEnabled:   boolInt(receiptEnabled),
		SalesInventoryID: salesInventoryID,
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
		ReceiptEnabled:   current.ReceiptEnabled,
		SalesInventoryID: current.SalesInventoryID,
	})
}

func boolInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
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
