package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "modernc.org/sqlite"

	db "kashi-pos/db/sqlc"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
)

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

	dir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	dbDir := filepath.Join(dir, "kashi-pos")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatal(err)
	}
	dbPath := filepath.Join(dbDir, "kashi.db")

	a.conn, err = sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		log.Fatal(err)
	}

	if err := runMigrations(dbPath); err != nil {
		log.Fatal("migrations failed:", err)
	}
	a.queries = db.New(a.conn)
	a.seedDefaults()
}

func (a *App) shutdown(ctx context.Context) {
	if a.conn != nil {
		a.conn.Close()
	}
}

func runMigrations(dbPath string) error {
	migrationsPath := resolveMigrationsPath()
	migDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("migrate db: %w", err)
	}
	defer migDB.Close()

	driver, err := sqlite.WithInstance(migDB, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("migrate driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsPath,
		"sqlite",
		driver,
	)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func resolveMigrationsPath() string {
	// Try relative to executable first
	exe, err := os.Executable()
	if err == nil {
		p := filepath.Join(filepath.Dir(exe), "db", "migrations")
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}
	// Fallback to CWD
	p := filepath.Join(".", "db", "migrations")
	if info, err := os.Stat(p); err == nil && info.IsDir() {
		abs, _ := filepath.Abs(p)
		return abs
	}
	// Fallback to project root (dev mode)
	_, b, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(b), "db", "migrations")
}

func (a *App) VerifyLogin(username, password string) (map[string]interface{}, error) {
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
		log.Fatal("create admin role:", err)
	}

	_, err = a.queries.CreateRole(ctx, db.CreateRoleParams{
		Name:        "cashier",
		Description: "Cashier",
	})
	if err != nil {
		log.Fatal("create cashier role:", err)
	}

	_, err = a.queries.CreateUser(ctx, db.CreateUserParams{
		Username:     "admin",
		PasswordHash: hashPassword("admin"),
		RoleID:       adminRole.ID,
		Active:       1,
	})
	if err != nil {
		log.Fatal("create default admin:", err)
	}

	log.Println("seeded default roles and admin user")
	_, err = a.queries.GetSettings(ctx)
	if err != nil {
		a.queries.UpsertSettings(ctx, db.UpsertSettingsParams{
			BranchName:  "",
			ApiKey:      "",
			PrinterID:   "",
			PrinterSize: "",
			Theme:       "system",
		})
	}
}

func hashPassword(pw string) string {
	h := sha256.Sum256([]byte(pw))
	return fmt.Sprintf("%x", h)
}

func (a *App) GetTheme() (string, error) {
	settings, err := a.queries.GetSettings(a.ctx)
	if err != nil {
		return "", err
	}
	return settings.Theme, nil
}

func (a *App) SetTheme(theme string) error {
	err := a.queries.UpdateTheme(a.ctx, theme)
	if err != nil {
		return err
	}
	return nil
}
