package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"fmt"
	"log"
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

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		if err := os.WriteFile(dbPath, embeddedDB, 0644); err != nil {
			log.Println("failed to create database:", err)
			return
		}
		log.Println("copied embedded database")
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
		a.queries.UpsertSettings(ctx, db.UpsertSettingsParams{
			BranchName:  "",
			ApiKey:      "",
			PrinterID:   "",
			PrinterSize: "",
			Theme:       "light",
		})
	}
	a.seedCashboxAccounts()
	a.seedProducts()
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
		a.queries.CreateCashboxAccount(context.Background(), db.CreateCashboxAccountParams{
			Name:         acc.name,
			CurrencyCode: acc.currency,
		})
	}
	log.Println("seeded cashbox accounts")
}

func (a *App) seedProducts() {
	existing, _ := a.queries.ListProducts(context.Background())
	if len(existing) > 0 {
		return
	}

	type product struct {
		barcode, code, name, category string
		price                          float64
		discount                       int64
	}
	products := []product{
		{"8000000012345", "TS-001", "Graphic Tee", "t-shirt", 19.99, 15},
		{"8000000023456", "SW-001", "Classic Hoodie", "sweatshirt", 49.99, 10},
		{"8000000034567", "PT-001", "Slim Chinos", "pants", 39.99, 0},
		{"8000000045678", "SH-001", "Running Sneakers", "shoes", 89.99, 0},
		{"8000000056789", "HT-001", "Classic Fedora", "hat", 29.99, 0},
		{"8000000067890", "CT-001", "Wool Overcoat", "coat", 189.99, 0},
		{"8000000078901", "JK-001", "Denim Jacket", "jacket", 79.99, 0},
		{"8000000089012", "BZ-001", "Navy Blazer", "blazer", 149.99, 0},
		{"8000000090123", "DR-001", "Floral Summer Dress", "dress", 69.99, 20},
		{"8000000101234", "FM-001", "Business Suit", "formal", 249.99, 5},
		{"8000000112345", "FW-001", "Evening Gown", "formal-woman", 199.99, 0},
		{"8000000123456", "UW-001", "Silk Boxers", "underwear", 24.99, 0},
		{"8000000134567", "TN-001", "Silk Necktie", "tie", 34.99, 0},
		{"8000000145678", "BT-001", "Wool Bow Tie", "bow-tie", 29.99, 0},
		{"8000000156789", "GL-001", "Reading Glasses", "glasses", 59.99, 0},
		{"8000000167890", "SG-001", "Aviator Sunglasses", "sunglasses", 79.99, 10},
		{"8000000178901", "NK-001", "Gold Chain Necklace", "necklace", 89.99, 0},
		{"8000000189012", "RG-001", "Silver Band Ring", "ring", 49.99, 0},
		{"8000000190123", "WK-001", "Chronograph Watch", "watch", 199.99, 10},
		{"8000000201234", "WL-001", "Leather Bifold Wallet", "wallet", 39.99, 0},
		{"8000000212345", "PR-001", "Clutch Purse", "purse", 59.99, 15},
		{"8000000223456", "BG-001", "Canvas Tote Bag", "bag", 29.99, 0},
		{"8000000234567", "SC-001", "Carry-On Suitcase", "suitcase", 129.99, 0},
		{"8000000245678", "UB-001", "Compact Umbrella", "umbrella", 24.99, 0},
		{"8000000256789", "MK-001", "Surgical Face Mask 50-Pack", "mask", 14.99, 20},
		{"8000000267890", "BG-002", "Training Gloves", "boxing-gloves", 49.99, 0},
		{"8000000278901", "SC-002", "Cashmere Scarf", "scarf", 44.99, 0},
		{"8000000289012", "BL-001", "Leather Belt", "belt", 34.99, 0},
		{"8000000290123", "SH-004", "Denim Shorts", "shorts", 29.99, 10},
		{"8000000301234", "SK-001", "Pleated Skirt", "skirt", 44.99, 0},
		{"8000000312345", "LG-001", "Cotton Leggings", "leggings", 24.99, 0},
		{"8000000323456", "SK-002", "Merino Wool Socks 3-Pack", "socks", 14.99, 0},
		{"8000000334567", "SD-001", "Leather Sandals", "sandals", 59.99, 15},
		{"8000000345678", "HL-001", "Stiletto Heels", "heels", 89.99, 0},
	}

	for _, p := range products {
		_, err := a.queries.CreateProduct(context.Background(), db.CreateProductParams{
			Barcode:     p.barcode,
			Code:        p.code,
			Name:        p.name,
			Description: p.category,
			IsActive:    1,
			Price:       int64(p.price * 100),
			Discount:    p.discount,
		})
		if err != nil {
			log.Println("seed product:", p.code, err)
		}
	}
	log.Println("seeded", len(products), "products")
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
		"branchName":  settings.BranchName,
		"apiKey":      settings.ApiKey,
		"printerID":   settings.PrinterID,
		"printerSize": settings.PrinterSize,
	}, nil
}

func (a *App) UpdateSettings(branchName, apiKey, printerID, printerSize string) error {
	if a.queries == nil {
		return fmt.Errorf("database not initialized")
	}
	current, err := a.queries.GetSettings(a.ctx)
	if err != nil {
		return err
	}
	return a.queries.UpdateSettings(a.ctx, db.UpdateSettingsParams{
		BranchName:  branchName,
		ApiKey:      apiKey,
		PrinterID:   printerID,
		PrinterSize: printerSize,
		Theme:       current.Theme,
	})
}
