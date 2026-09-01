package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// The values the touch login screen shipped with before credentials moved
// into the auth_credentials table (migration 000083). Still used as the
// fallback when there's no database yet (unit tests call NewApp() with no
// store) or the row is somehow missing.
const (
	defaultAdminPassword = "1963"
	defaultCashierPIN    = "1478"
)

// authCredentials reads the singleton auth_credentials row, falling back to
// the shipped defaults when the database or row is unavailable.
func (a *App) authCredentials() (adminHash, cashierPIN string) {
	adminHash = hashPassword(defaultAdminPassword)
	cashierPIN = defaultCashierPIN
	if a.conn == nil {
		return
	}
	var h, p string
	err := a.conn.QueryRow(`SELECT admin_password_hash, cashier_pin FROM auth_credentials WHERE id = 1`).Scan(&h, &p)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("read auth_credentials: %v", err)
		}
		return
	}
	if h != "" {
		adminHash = h
	}
	if p != "" {
		cashierPIN = p
	}
	return
}

func (a *App) adminPasswordHash() string {
	h, _ := a.authCredentials()
	return h
}

func (a *App) cashierPIN() string {
	_, p := a.authCredentials()
	return p
}

// GetCashierPIN returns the current shared cashier login PIN, for display on
// the admin-only Settings screen.
func (a *App) GetCashierPIN() (string, error) {
	if a.conn == nil {
		return "", fmt.Errorf("database not initialized")
	}
	return a.cashierPIN(), nil
}

// ChangeAdminPassword swaps the admin sign-in password after verifying the
// current one. Backs Settings -> Change admin password.
func (a *App) ChangeAdminPassword(currentPassword, newPassword string) error {
	if a.conn == nil {
		return fmt.Errorf("database not initialized")
	}
	if a.adminPasswordHash() != hashPassword(currentPassword) {
		return fmt.Errorf("current password is wrong")
	}
	newPassword = strings.TrimSpace(newPassword)
	if len(newPassword) < 4 {
		return fmt.Errorf("new password must be at least 4 characters")
	}
	_, err := a.conn.Exec(`UPDATE auth_credentials SET admin_password_hash = ? WHERE id = 1`, hashPassword(newPassword))
	return err
}

// ForceSetAdminPassword sets the admin sign-in password without knowing the
// current one -- the local execution path for the admin_password field of a
// kashi update_settings command, which arrives over the already
// authenticated command channel.
func (a *App) ForceSetAdminPassword(newPassword string) error {
	if a.conn == nil {
		return fmt.Errorf("database not initialized")
	}
	newPassword = strings.TrimSpace(newPassword)
	if len(newPassword) < 4 {
		return fmt.Errorf("new password must be at least 4 characters")
	}
	_, err := a.conn.Exec(`UPDATE auth_credentials SET admin_password_hash = ? WHERE id = 1`, hashPassword(newPassword))
	return err
}

// SetCashierPIN changes the shared cashier login PIN. Settings is admin-only
// (router requiresAdmin), so there's no further gate here.
func (a *App) SetCashierPIN(newPIN string) error {
	if a.conn == nil {
		return fmt.Errorf("database not initialized")
	}
	newPIN = strings.TrimSpace(newPIN)
	if !isFourDigitPIN(newPIN) {
		return fmt.Errorf("PIN must be exactly 4 digits")
	}
	_, err := a.conn.Exec(`UPDATE auth_credentials SET cashier_pin = ? WHERE id = 1`, newPIN)
	return err
}

func isFourDigitPIN(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
