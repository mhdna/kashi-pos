package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	_ "modernc.org/sqlite"

	db "kashi-pos/db/sqlc"
)

var supportedSocialPlatforms = []string{"facebook", "tiktok", "instagram", "threads", "youtube"}

var errDatabaseMissing = errors.New("database not found")

//go:embed db/kashi.db
var embeddedDB []byte

type App struct {
	ctx        context.Context
	store      *db.Store
	conn       *sql.DB
	dbPath     string
	admin      *adminServer
	startupErr error
	ready      chan struct{}

	pinMu             sync.Mutex
	failedPINAttempts int
	pinLockedUntil    time.Time
	pinLockoutLevel   int

	syncWorkerStop chan struct{}
	// syncWorkerKick lets a checkout wake the sync worker immediately after
	// committing a new outbox entry, instead of waiting up to
	// syncWorkerInterval for the next tick -- see kickSyncWorker in
	// sync_worker.go. Buffered so a kick from the checkout path never blocks
	// on the worker goroutine, and nil until startSyncWorker runs, which
	// kickSyncWorker's nil-safe select handles the same way an unconfigured
	// install already tolerates a no-op sync worker.
	syncWorkerKick chan struct{}
	wsClientStop   chan struct{}
	// backgroundWG lets shutdown wait for the sync worker and WS client
	// goroutines to actually exit, not just signal them to — otherwise a
	// goroutine can still be mid-tick (touching the SQLite file, e.g.
	// recreating -wal/-shm journal files) after shutdown returns, which
	// raced with test cleanup deleting the database's temp directory.
	backgroundWG sync.WaitGroup
}

const (
	maxPINAttempts    = 3
	pinLockoutBase    = 30 * time.Second
	pinLockoutMax     = 30 * time.Minute
	pinLockoutMaxTier = 10
)

// pinLockoutDuration doubles the wait every time the 3-attempt threshold is
// hit again, so restarting the app to dodge a single 30s wait just leads
// into a longer one next time. level is 1-based (first lockout = level 1).
func pinLockoutDuration(level int) time.Duration {
	if level < 1 {
		level = 1
	}
	if level > pinLockoutMaxTier {
		level = pinLockoutMaxTier
	}
	duration := pinLockoutBase * time.Duration(1<<uint(level-1))
	if duration > pinLockoutMax {
		duration = pinLockoutMax
	}
	return duration
}

func NewApp() *App {
	return &App{ready: make(chan struct{})}
}

func (a *App) startup(ctx context.Context) {
	defer close(a.ready)
	a.ctx = ctx

	dbPath, err := applicationDatabasePath()
	if err != nil {
		a.failStartup("resolve database path", err)
		return
	}
	log.Println("database path:", dbPath)
	a.dbPath = dbPath

	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			// An explicit KASHI_DB_PATH is an administrator-provided deployment
			// path, so retain the existing headless/bootstrap behavior for it.
			if os.Getenv("KASHI_DB_PATH") != "" {
				if err := a.openDatabase(dbPath, true); err != nil {
					a.failStartup("open database", err)
				}
				return
			}
			a.startupErr = fmt.Errorf("%w: %s", errDatabaseMissing, dbPath)
			return
		}
		a.failStartup("inspect database", err)
		return
	}
	if err := a.openDatabase(dbPath, false); err != nil {
		a.failStartup("open database", err)
	}
}

func applicationDatabasePath() (string, error) {
	if configured := os.Getenv("KASHI_DB_PATH"); configured != "" {
		return configured, nil
	}
	if path, err := storedDatabasePath(); err == nil && path != "" {
		return path, nil
	}
	dataDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "kashi-pos", "database"), nil
}

func storedDatabasePath() (string, error) {
	path, err := databasePathConfigPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func databasePathConfigPath() (string, error) {
	dataDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "kashi-pos", "database-path"), nil
}

func persistDatabasePath(path string) error {
	configPath, err := databasePathConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(configPath, []byte(path+"\n"), 0600)
}

func (a *App) openDatabase(path string, createIfMissing bool) error {
	path = filepath.Clean(path)
	databaseExisted := true
	if _, err := os.Stat(path); err != nil {
		if !createIfMissing || !os.IsNotExist(err) {
			return err
		}
		databaseExisted = false
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("create database directory: %w", err)
		}
		if err := os.WriteFile(path, embeddedDB, 0600); err != nil {
			return fmt.Errorf("create database: %w", err)
		}
	}

	conn, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)")
	if err != nil {
		return err
	}
	if err := conn.PingContext(a.ctx); err != nil {
		_ = conn.Close()
		return fmt.Errorf("ping database: %w", err)
	}
	if err := applyMigrations(a.ctx, conn, path, databaseExisted); err != nil {
		_ = conn.Close()
		return fmt.Errorf("migrate database: %w", err)
	}

	a.conn = conn
	a.store = db.NewStore(conn)
	a.dbPath = path
	a.startupErr = nil
	a.loadPINLockoutState()
	a.seedDefaults()
	if err := a.applyEnvironmentDefaults(); err != nil {
		log.Println("apply environment setting defaults:", err)
	}
	if err := a.closeStaleOpenShifts(); err != nil {
		log.Println("close stale open shifts:", err)
	}
	a.startAdminServer()
	a.startSyncWorker()
	a.startWSClient()
	return nil
}

// GetDatabaseStatus reports the active path and whether startup needs a database.
func (a *App) GetDatabaseStatus() map[string]interface{} {
	exists := false
	if a.dbPath != "" {
		_, err := os.Stat(a.dbPath)
		exists = err == nil
	}
	return map[string]interface{}{
		"path":    a.dbPath,
		"exists":  exists,
		"missing": errors.Is(a.startupErr, errDatabaseMissing),
	}
}

// ChooseDatabase opens an existing SQLite database selected by the user.
func (a *App) ChooseDatabase() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Choose Kashi database",
		Filters: []runtime.FileFilter{{DisplayName: "SQLite database", Pattern: "*.db;database"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	if err := a.switchDatabase(path, false); err != nil {
		return "", err
	}
	return a.dbPath, nil
}

// CreateDatabase asks for a folder and creates a fresh database there.
func (a *App) CreateDatabase() (string, error) {
	directory, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:                "Choose folder for the new Kashi database",
		CanCreateDirectories: true,
	})
	if err != nil || directory == "" {
		return "", err
	}
	path := filepath.Join(directory, "database")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("a database already exists in %s", directory)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := a.switchDatabase(path, true); err != nil {
		return "", err
	}
	return a.dbPath, nil
}

func (a *App) switchDatabase(path string, createIfMissing bool) error {
	path, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return err
	}
	if a.conn != nil {
		a.stopAdminServer(context.Background())
		_ = a.conn.Close()
		a.conn = nil
		a.store = nil
	}
	if err := a.openDatabase(path, createIfMissing); err != nil {
		a.startupErr = err
		return err
	}
	if os.Getenv("KASHI_DB_PATH") == "" {
		if err := persistDatabasePath(path); err != nil {
			return fmt.Errorf("save database path: %w", err)
		}
	}
	return nil
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
	a.stopWSClient()
	a.stopSyncWorker()
	// Both signaled to stop above; wait for them to actually finish before
	// touching a.conn below — a goroutine still mid-tick when the
	// connection closes underneath it would either error noisily or (worse,
	// for tests) still be touching the SQLite file on disk after shutdown
	// has returned.
	a.backgroundWG.Wait()
	a.stopAdminServer(ctx)
	if a.conn != nil {
		a.conn.Close()
	}
}

// loadPINLockoutState restores the PIN lockout counters from the database so
// a locked-out login can't be reset by restarting the app.
func (a *App) loadPINLockoutState() {
	a.pinMu.Lock()
	defer a.pinMu.Unlock()
	a.failedPINAttempts = 0
	a.pinLockedUntil = time.Time{}
	a.pinLockoutLevel = 0
	if a.conn == nil {
		return
	}
	var failedAttempts, lockoutLevel int
	var lockedUntil string
	err := a.conn.QueryRow(`SELECT failed_attempts, lockout_level, locked_until FROM pin_lockout WHERE id = 1`).Scan(&failedAttempts, &lockoutLevel, &lockedUntil)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("load pin lockout state: %v", err)
		}
		return
	}
	a.failedPINAttempts = failedAttempts
	a.pinLockoutLevel = lockoutLevel
	if lockedUntil != "" {
		if t, err := time.Parse(time.RFC3339, lockedUntil); err == nil {
			a.pinLockedUntil = t
		}
	}
}

// savePINLockoutState persists the current lockout counters. Caller must
// hold pinMu.
func (a *App) savePINLockoutState() {
	if a.conn == nil {
		return
	}
	lockedUntil := ""
	if !a.pinLockedUntil.IsZero() {
		lockedUntil = a.pinLockedUntil.UTC().Format(time.RFC3339)
	}
	_, err := a.conn.Exec(`INSERT INTO pin_lockout(id, failed_attempts, lockout_level, locked_until) VALUES(1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET failed_attempts=excluded.failed_attempts, lockout_level=excluded.lockout_level, locked_until=excluded.locked_until`,
		a.failedPINAttempts, a.pinLockoutLevel, lockedUntil)
	if err != nil {
		log.Printf("save pin lockout state: %v", err)
	}
}

// unlockPINLockout clears the lockout, for use by the remote admin API when
// a manager needs to unlock a till without waiting. Caller must hold pinMu.
func (a *App) unlockPINLockout() {
	a.failedPINAttempts = 0
	a.pinLockedUntil = time.Time{}
	a.pinLockoutLevel = 0
	a.savePINLockoutState()
}

// GetPINLockoutStatus reports whether the login PIN is currently locked out
// and for how many more seconds, so the login screen can show the countdown
// immediately on load instead of only after a rejected attempt - including
// right after an app restart, since the lockout is persisted.
func (a *App) GetPINLockoutStatus() map[string]interface{} {
	a.pinMu.Lock()
	defer a.pinMu.Unlock()
	remaining := time.Until(a.pinLockedUntil)
	if remaining <= 0 {
		return map[string]interface{}{"locked": false, "retryAfter": 0}
	}
	seconds := int((remaining + time.Second - 1) / time.Second)
	return map[string]interface{}{"locked": true, "retryAfter": seconds}
}

// VerifyPIN authenticates the temporary role PINs used by the touch login screen.
// Keeping the PIN check here prevents the credentials from being shipped in the
// frontend bundle. Replace this with per-user PIN storage when user PIN management
// is introduced.
func (a *App) VerifyPIN(pin string) (map[string]interface{}, error) {
	a.pinMu.Lock()
	defer a.pinMu.Unlock()

	now := time.Now()
	if now.Before(a.pinLockedUntil) {
		remaining := a.pinLockedUntil.Sub(now)
		seconds := int((remaining + time.Second - 1) / time.Second)
		return nil, fmt.Errorf("locked:%d", seconds)
	}

	role := ""
	switch pin {
	case "0000":
		role = "admin"
	case "1234":
		role = "cashier"
	default:
		a.failedPINAttempts++
		if a.failedPINAttempts >= maxPINAttempts {
			a.failedPINAttempts = 0
			a.pinLockoutLevel++
			duration := pinLockoutDuration(a.pinLockoutLevel)
			a.pinLockedUntil = now.Add(duration)
			a.savePINLockoutState()
			return nil, fmt.Errorf("locked:%d", int(duration/time.Second))
		}
		a.savePINLockoutState()
		return nil, fmt.Errorf("invalid PIN")
	}

	a.failedPINAttempts = 0
	a.pinLockedUntil = time.Time{}
	a.pinLockoutLevel = 0
	a.savePINLockoutState()
	if a.store != nil {
		if err := a.closeStaleOpenShifts(); err != nil {
			log.Printf("close stale open shifts after PIN login: %v", err)
		}
	}

	return map[string]interface{}{
		"id":       int64(0),
		"username": role,
		"role":     role,
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
	for _, acc := range []struct{ name, currency, color string }{
		{"Cash $$", "USD", "#2E7D32"},
		{"Cash LBP", "LBP", "#1565C0"},
		{"Visa", "USD", "#7B1FA2"},
		{"Whish", "USD", "#D32F2F"},
		{"On Account", "USD", ""},
		{"On Coupon", "USD", ""},
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
		created, err := a.store.CreateCashboxAccount(context.Background(), db.CreateCashboxAccountParams{
			Name:         acc.name,
			CurrencyCode: acc.currency,
		})
		if err != nil {
			log.Println("seed cashbox account:", err)
			continue
		}
		if acc.color != "" {
			if _, err := a.store.UpdateCashboxAccount(context.Background(), db.UpdateCashboxAccountParams{
				ID: created.ID, Name: created.Name, CurrencyCode: created.CurrencyCode, Color: acc.color,
			}); err != nil {
				log.Println("seed cashbox account color:", err)
			}
		}
	}
	log.Println("seeded cashbox accounts")
}

func hashPassword(pw string) string {
	h := sha256.Sum256([]byte(pw))
	return fmt.Sprintf("%x", h)
}

// WaitUntilReady blocks until backend startup (database open, migrations,
// default seeding) has finished, then returns the startup error (if any) and
// the saved theme in a single round trip. The frontend must use this instead
// of calling GetStartupError/GetTheme directly on mount - those read state
// that startup() populates concurrently, so calling them before startup()
// finishes can race and silently report "ready" with defaults.
func (a *App) WaitUntilReady() map[string]interface{} {
	<-a.ready
	result := map[string]interface{}{
		"error":           a.GetStartupError(),
		"theme":           "light",
		"databaseMissing": errors.Is(a.startupErr, errDatabaseMissing),
	}
	if theme, err := a.GetTheme(); err == nil {
		result["theme"] = theme
	}
	return result
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
		"branchName":             settings.BranchName,
		"branchCode":             settings.BranchCode,
		"apiKey":                 settings.ApiKey,
		"centralApiUrl":          settings.CentralApiUrl,
		"printerID":              settings.PrinterID,
		"printerSize":            settings.PrinterSize,
		"receiptPrinter":         settings.ReceiptPrinter,
		"screenPort":             settings.ScreenPort,
		"receiptWidth":           settings.ReceiptWidth,
		"receiptHeight":          settings.ReceiptHeight,
		"taxRate":                settings.TaxRate,
		"roundingMode":           settings.RoundingMode,
		"roundingCurrency":       a.effectiveRoundingCurrencyCode(settings.RoundingCurrency),
		"receiptEnabled":         settings.ReceiptEnabled != 0,
		"salesInventoryId":       settings.SalesInventoryID,
		"exchangeRate":           settings.ExchangeRate,
		"maxOverpaymentCents":    settings.MaxOverpaymentCents,
		"exchangeWindowHours":    settings.ExchangeWindowHours,
		"marketName":             settings.MarketName,
		"marketPhone":            settings.MarketPhone,
		"branchPhone":            settings.BranchPhone,
		"counterNumber":          settings.CounterNumber,
		"registerName":           settings.CounterNumber,
		"marketDescription":      settings.MarketDescription,
		"returnPolicy":           settings.ReturnPolicy,
		"website":                settings.Website,
		"instagram":              settings.Instagram,
		"socialPlatforms":        decodeSocialPlatforms(settings.SocialPlatforms),
		"socialHandles":          decodeSocialHandles(settings.SocialHandles),
		"receiptFont":            settings.ReceiptFont,
		"receiptTitleSize":       settings.ReceiptTitleSize,
		"receiptBodySize":        settings.ReceiptBodySize,
		"receiptBodyFont":        settings.ReceiptBodyFont,
		"receiptCutoff":          settings.ReceiptCutoff != 0,
		"defaultOpeningFloatUsd": settings.DefaultOpeningFloatUsd,
		"defaultOpeningFloatLbp": settings.DefaultOpeningFloatLbp,
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
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, priceChangeManualOverrideFlag); flagErr == nil {
		result["priceChangeManualOverrideEnabled"] = enabled != 0
	} else {
		result["priceChangeManualOverrideEnabled"] = false
	}
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, invoiceKeyboardModeFlag); flagErr == nil {
		result["invoiceKeyboardModeEnabled"] = enabled != 0
	} else {
		result["invoiceKeyboardModeEnabled"] = false
	}
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, clientRequiredFlag); flagErr == nil {
		result["clientRequired"] = enabled != 0
	} else {
		result["clientRequired"] = true
	}
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, hidePaymentMethodCurrencyFlag); flagErr == nil {
		result["hidePaymentMethodCurrency"] = enabled != 0
	} else {
		result["hidePaymentMethodCurrency"] = false
	}
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, autoAppendLbpZerosFlag); flagErr == nil {
		result["autoAppendLbpZeros"] = enabled != 0
	} else {
		result["autoAppendLbpZeros"] = false
	}
	if enabled, flagErr := a.store.GetFeatureFlag(a.ctx, overchangeConfirmationFlag); flagErr == nil {
		result["overchangeConfirmationEnabled"] = enabled != 0
	} else {
		result["overchangeConfirmationEnabled"] = true
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

// SetPriceChangeManualOverrideEnabled toggles how manual item price/discount
// edits behave. Disabled (the default) is "discount mode": editing a line's
// price or discount recomputes the other from the item's original price, so
// a price change is always expressed as a discount. Enabled is "manual
// override mode": price and discount are edited independently, as they were
// before this setting existed.
func (a *App) SetPriceChangeManualOverrideEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.SetFeatureFlag(a.ctx, db.SetFeatureFlagParams{
		Key: priceChangeManualOverrideFlag, Enabled: boolInt(enabled),
	})
}

// SetInvoiceKeyboardModeEnabled toggles keyboard mode for the current
// invoice's item list. Enabled, clicking a line's qty/discount/price no
// longer opens the number-pad popup - it just focuses the value so digits
// can be typed directly, and arrow keys move focus between fields.
func (a *App) SetInvoiceKeyboardModeEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.SetFeatureFlag(a.ctx, db.SetFeatureFlagParams{
		Key: invoiceKeyboardModeFlag, Enabled: boolInt(enabled),
	})
}

// SetClientRequired toggles whether a client must be explicitly selected
// before checkout. Required (the default) is today's behavior - the
// cashier must use "Prefer not to say" or enter a phone/name. Not required
// lets invoices default to Anonymous automatically, though a phone and
// name can still be entered at any time.
func (a *App) SetClientRequired(required bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.SetFeatureFlag(a.ctx, db.SetFeatureFlagParams{
		Key: clientRequiredFlag, Enabled: boolInt(required),
	})
}

// SetHidePaymentMethodCurrency toggles whether each payment method's
// currency code is shown as a second line under its name on the checkout
// calculator's payment buttons.
func (a *App) SetHidePaymentMethodCurrency(hide bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.SetFeatureFlag(a.ctx, db.SetFeatureFlagParams{
		Key: hidePaymentMethodCurrencyFlag, Enabled: boolInt(hide),
	})
}

// SetAutoAppendLbpZerosEnabled toggles whether the checkout calculator
// automatically multiplies a typed amount by 1,000 when it's confirmed
// against an LBP (cash-rounded) payment method, so cashiers can type "50"
// instead of "50000".
func (a *App) SetAutoAppendLbpZerosEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.SetFeatureFlag(a.ctx, db.SetFeatureFlagParams{
		Key: autoAppendLbpZerosFlag, Enabled: boolInt(enabled),
	})
}

// lockablePages are the nav pages an admin can choose to unlock for
// non-admin users. Every page here is locked for non-admins by default
// (missing feature flag = locked); admins always see every page regardless
// of these flags. Pages not in this list (home, invoices-list, shifts,
// settings, etc.) are never gated by this mechanism.
var lockablePages = []string{"clients", "inventory", "transfers", "attendance", "salespersons"}

func pageUnlockFlag(page string) string {
	return "page_unlock_" + page
}

// ListPageUnlocks reports, for each lockable page, whether it has been
// unlocked for non-admin users. Missing/unset means locked.
func (a *App) ListPageUnlocks() (map[string]bool, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	result := make(map[string]bool, len(lockablePages))
	for _, page := range lockablePages {
		enabled, err := a.store.GetFeatureFlag(a.ctx, pageUnlockFlag(page))
		result[page] = err == nil && enabled != 0
	}
	return result, nil
}

// SetPageUnlocked unlocks or re-locks one page for non-admin users.
func (a *App) SetPageUnlocked(page string, unlocked bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	found := false
	for _, p := range lockablePages {
		if p == page {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("unknown page %q", page)
	}
	return a.store.SetFeatureFlag(a.ctx, db.SetFeatureFlagParams{
		Key: pageUnlockFlag(page), Enabled: boolInt(unlocked),
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
		ExchangeRate: current.ExchangeRate, MaxOverpaymentCents: current.MaxOverpaymentCents, ExchangeWindowHours: current.ExchangeWindowHours, MarketName: marketName, MarketPhone: marketPhone, BranchPhone: branchPhone, MarketDescription: marketDescription,
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
	roundingCurrencyRow, err := resolveCurrency(a.ctx, a.store.Queries, roundingCurrency)
	if err != nil || !roundingCurrencyRow.IsActive {
		return fmt.Errorf("rounding currency %q is not a known, active currency", roundingCurrency)
	}
	roundingCurrency = roundingCurrencyRow.Code
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
		ExchangeRate:     exchangeRate, MaxOverpaymentCents: maxOverpaymentCents, ExchangeWindowHours: current.ExchangeWindowHours,
		MarketName: current.MarketName, MarketPhone: current.MarketPhone, MarketDescription: current.MarketDescription,
		ReturnPolicy: current.ReturnPolicy, Website: current.Website, Instagram: current.Instagram, SocialPlatforms: current.SocialPlatforms, SocialHandles: current.SocialHandles, ReceiptFont: current.ReceiptFont,
		ReceiptTitleSize: current.ReceiptTitleSize, ReceiptBodySize: current.ReceiptBodySize, ReceiptCutoff: current.ReceiptCutoff,
		ReceiptBodyFont: current.ReceiptBodyFont, BranchPhone: current.BranchPhone, CounterNumber: current.CounterNumber,
	})
}

// UpdateCentralAPIURL sets the base URL of the kashi server this branch
// syncs to (e.g. "http://localhost:8080"). Empty disables sync entirely —
// the outbox worker and WebSocket client both check for this before
// running (see sync_client.go's centralClient.enabled()).
func (a *App) UpdateCentralAPIURL(centralApiUrl string) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.UpdateCentralAPIURL(a.ctx, centralApiUrl)
}

func (a *App) SetExchangeWindowHours(hours int64) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if hours < 0 {
		return fmt.Errorf("exchange window cannot be negative")
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
		TaxRate:          current.TaxRate,
		RoundingMode:     current.RoundingMode,
		RoundingCurrency: current.RoundingCurrency,
		ReceiptEnabled:   current.ReceiptEnabled,
		SalesInventoryID: current.SalesInventoryID,
		ExchangeRate:     current.ExchangeRate, MaxOverpaymentCents: current.MaxOverpaymentCents, ExchangeWindowHours: hours,
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
		if _, err := a.store.CreateShift(ctx, db.CreateShiftParams{}); err != nil {
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
