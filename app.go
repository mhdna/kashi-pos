package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	db "kashi-pos/db/sqlc"
)

//go:embed db/kashi.db
var embeddedDB []byte

type App struct {
	ctx     context.Context
	queries *db.Queries
	conn    *sql.DB
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	dbPath := "database"
	if exe, err := os.Executable(); err == nil {
		dbPath = filepath.Join(filepath.Dir(exe), "database")
	}

	log.Println("database path:", dbPath)

	if err := os.WriteFile(dbPath, embeddedDB, 0644); err != nil {
		log.Println("failed to create database:", err)
		return
	}

	var err error
	a.conn, err = sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		log.Println("failed to open database:", err)
		return
	}

	a.queries = db.New(a.conn)
	a.seedDefaults()
}

func (a *App) shutdown(ctx context.Context) {
	if a.conn != nil {
		a.conn.Close()
	}
}

func (a *App) VerifyLogin(username, password string) (map[string]interface{}, error) {
	if a.queries == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	user, err := a.queries.GetUserByUsername(a.ctx, username)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	if user.PasswordHash != hashPassword(password) {
		return nil, fmt.Errorf("wrong password")
	}
	if user.Active == 0 {
		return nil, fmt.Errorf("account disabled")
	}

	role, err := a.queries.GetRoleByID(a.ctx, user.RoleID)
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
	if a.queries == nil {
		return fmt.Errorf("database not initialized")
	}
	user, err := a.queries.GetUserByUsername(a.ctx, username)
	if err != nil {
		return fmt.Errorf("user not found")
	}
	if user.PasswordHash != hashPassword(oldPassword) {
		return fmt.Errorf("wrong password")
	}
	_, err = a.queries.UpdateUser(a.ctx, db.UpdateUserParams{
		Username:     user.Username,
		PasswordHash: hashPassword(newPassword),
		RoleID:       user.RoleID,
		Active:       user.Active,
		ID:           user.ID,
	})
	return err
}

func (a *App) seedDefaults() {
	if a.queries == nil {
		return
	}

	ctx := context.Background()

	roles, err := a.queries.ListRoles(ctx)
	if err != nil || len(roles) > 0 {
		return
	}

	adminRole, err := a.queries.CreateRole(ctx, db.CreateRoleParams{
		Name:        "admin",
		Description: "Administrator",
	})
	if err != nil {
		log.Println("create admin role:", err)
		return
	}

	cashierRole, err := a.queries.CreateRole(ctx, db.CreateRoleParams{
		Name:        "cashier",
		Description: "Cashier",
	})
	if err != nil {
		log.Println("create cashier role:", err)
		return
	}

	_, err = a.queries.CreateUser(ctx, db.CreateUserParams{
		Username:     "admin",
		PasswordHash: hashPassword("admin"),
		RoleID:       adminRole.ID,
		Active:       1,
	})
	if err != nil {
		log.Println("create default admin:", err)
		return
	}
	_, err = a.queries.CreateUser(ctx, db.CreateUserParams{
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
	_, err = a.queries.GetSettings(ctx)
	if err != nil {
		if err := a.queries.UpsertSettings(ctx, db.UpsertSettingsParams{
			BranchName:     "",
			ApiKey:         "",
			PrinterID:      "",
			PrinterSize:    "",
			ReceiptPrinter: "",
			ScreenPort:     "",
			ReceiptWidth:   80,
			ReceiptHeight:  297,
			Theme:          "light",
		}); err != nil {
			log.Println("upsert settings:", err)
		}
	}
	a.seedCashboxAccounts()
	a.seedProducts()
	a.seedSalespersons()
}

func (a *App) seedSalespersons() {
	existing, _ := a.queries.ListSalespersons(context.Background())
	if len(existing) > 0 {
		return
	}
	for _, name := range []string{"Sarah M", "Fatima G", "Ali A", "Zahraa M", "Alex A"} {
		if _, err := a.queries.CreateSalesperson(context.Background(), name); err != nil {
			log.Println("seed salesperson:", err)
		}
	}
	log.Println("seeded salespersons")
}

func (a *App) seedCashboxAccounts() {
	existing, _ := a.queries.ListCashboxAccounts(context.Background())
	if len(existing) > 0 {
		return
	}
	for _, acc := range []struct{ name, currency string }{
		{"USD", "USD"},
		{"LBP", "LBP"},
		{"Visa", "USD"},
		{"Whish", "USD"},
	} {
		if _, err := a.queries.CreateCashboxAccount(context.Background(), db.CreateCashboxAccountParams{
			Name:         acc.name,
			CurrencyCode: acc.currency,
		}); err != nil {
			log.Println("seed cashbox account:", err)
		}
	}
	log.Println("seeded cashbox accounts")
}

func (a *App) seedProducts() {
	existing, _ := a.queries.ListProducts(context.Background())
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

		_, err := a.queries.CreateProduct(context.Background(), db.CreateProductParams{
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
	if a.queries == nil {
		return "light", nil
	}
	settings, err := a.queries.GetSettings(a.ctx)
	if err != nil {
		return "light", nil
	}
	return settings.Theme, nil
}

func (a *App) SetTheme(theme string) error {
	if a.queries == nil {
		return fmt.Errorf("database not initialized")
	}
	err := a.queries.UpdateTheme(a.ctx, theme)
	if err != nil {
		return err
	}
	return nil
}

func (a *App) GetAppSettings() (map[string]interface{}, error) {
	if a.queries == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	settings, err := a.queries.GetSettings(a.ctx)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"branchName":     settings.BranchName,
		"apiKey":         settings.ApiKey,
		"printerID":      settings.PrinterID,
		"printerSize":    settings.PrinterSize,
		"receiptPrinter": settings.ReceiptPrinter,
		"screenPort":     settings.ScreenPort,
		"receiptWidth":   settings.ReceiptWidth,
		"receiptHeight":  settings.ReceiptHeight,
	}, nil
}

func (a *App) UpdateSettings(branchName, apiKey, printerID, printerSize, receiptPrinter, screenPort string, receiptWidth, receiptHeight int64) error {
	if a.queries == nil {
		return fmt.Errorf("database not initialized")
	}
	current, err := a.queries.GetSettings(a.ctx)
	if err != nil {
		return err
	}
	return a.queries.UpdateSettings(a.ctx, db.UpdateSettingsParams{
		BranchName:     branchName,
		ApiKey:         apiKey,
		PrinterID:      printerID,
		PrinterSize:    printerSize,
		ReceiptPrinter: receiptPrinter,
		ScreenPort:     screenPort,
		ReceiptWidth:   receiptWidth,
		ReceiptHeight:  receiptHeight,
		Theme:          current.Theme,
	})
}
