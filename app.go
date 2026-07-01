package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"fmt"
	"log"
	"os"

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

	if _, err := os.Stat("./database"); os.IsNotExist(err) {
		if err := os.WriteFile("./database", embeddedDB, 0644); err != nil {
			log.Fatal("failed to create database:", err)
		}
		log.Println("copied embedded database")
	}

	var err error
	a.conn, err = sql.Open("sqlite", "./database"+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		log.Fatal(err)
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

	cashierRole, err := a.queries.CreateRole(ctx, db.CreateRoleParams{
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
	_, err = a.queries.CreateUser(ctx, db.CreateUserParams{
		Username:     "user",
		PasswordHash: hashPassword("user"),
		RoleID:       cashierRole.ID,
		Active:       1,
	})
	if err != nil {
		log.Fatal("create default user:", err)
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

func (a *App) GetAppSettings() (map[string]interface{}, error) {
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
