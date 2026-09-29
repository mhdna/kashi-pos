package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	db "kashi-pos/db/sqlc"
)

// branchEntry is one locally set-up branch: its own isolated database, plus
// the display name/code shown in the picker and admin switcher.
type branchEntry struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	DBPath string `json:"dbPath"`
}

// branchRegistry is the local (per-machine) record of every branch this
// install has been set up to run, replacing .env-based branch configuration.
// It lives outside any single branch's database because it has to be
// readable before any database is chosen.
type branchRegistry struct {
	Branches []branchEntry `json:"branches"`
	// Default is which branch opens automatically at the next app launch.
	Default string `json:"default"`
	// Active is the branch currently open; kept alongside Default so a
	// runtime switch (SwitchBranch) doesn't have to also mean "and make this
	// the new default forever" -- that's a separate, explicit admin action
	// (SetDefaultBranch).
	Active string `json:"active"`
}

func branchRegistryDir() (string, error) {
	dataDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "kashi-pos"), nil
}

func branchRegistryPath() (string, error) {
	dir, err := branchRegistryDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "branches.json"), nil
}

func loadBranchRegistry() (branchRegistry, error) {
	path, err := branchRegistryPath()
	if err != nil {
		return branchRegistry{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return branchRegistry{}, nil
		}
		return branchRegistry{}, err
	}
	var reg branchRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return branchRegistry{}, err
	}
	return reg, nil
}

func saveBranchRegistry(reg branchRegistry) error {
	path, err := branchRegistryPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (r branchRegistry) find(code string) (branchEntry, bool) {
	for _, b := range r.Branches {
		if b.Code == code {
			return b, true
		}
	}
	return branchEntry{}, false
}

// normalizeRegistryBranchCode mirrors normalizeBranchCode's rules (letters
// and numbers only) but lives here since it gates registry membership, which
// exists independently of any open database.
func normalizeRegistryBranchCode(code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return "", fmt.Errorf("branch code is required")
	}
	for _, r := range code {
		if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return "", fmt.Errorf("branch code must contain only letters and numbers")
		}
	}
	return code, nil
}

func branchDatabasePath(dir, code string) string {
	return filepath.Join(dir, "database-"+strings.ToLower(code))
}

// resolveStartupDatabasePath decides which database to open at launch, in
// order: an explicit KASHI_DB_PATH (the headless/deployment escape hatch),
// then the registry's active branch, falling back to its default branch,
// falling back to whichever branch is registered first. If nothing is
// registered yet, it tries to adopt a pre-registry install (see
// migrateLegacyInstallIntoRegistry) before finally reporting "" -- meaning
// the frontend's first-run setup screen should run.
func applicationDatabasePath() (string, error) {
	if configured := os.Getenv("KASHI_DB_PATH"); configured != "" {
		return configured, nil
	}
	reg, err := loadBranchRegistry()
	if err != nil {
		return "", err
	}
	if len(reg.Branches) == 0 {
		if migrated, ok := migrateLegacyInstallIntoRegistry(); ok {
			reg = migrated
		}
	}
	if entry, ok := reg.find(reg.Active); ok {
		return entry.DBPath, nil
	}
	if entry, ok := reg.find(reg.Default); ok {
		return entry.DBPath, nil
	}
	if len(reg.Branches) > 0 {
		return reg.Branches[0].DBPath, nil
	}
	return "", nil
}

// migrateLegacyInstallIntoRegistry adopts database files left over from
// before the branch registry existed (a single unsuffixed "database", or the
// short-lived "database-<code>" env-suffixed scheme) so upgrading this app
// doesn't look like data loss. Each candidate file is opened just long
// enough to read its own branch_code/branch_name, which is how it gets a
// name in the registry without the registry having had to know it already.
func migrateLegacyInstallIntoRegistry() (branchRegistry, bool) {
	dir, err := branchRegistryDir()
	if err != nil {
		return branchRegistry{}, false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return branchRegistry{}, false
	}
	var reg branchRegistry
	seen := map[string]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name != "database" && !strings.HasPrefix(name, "database-") {
			continue
		}
		if strings.HasPrefix(name, "database-path") {
			continue
		}
		if strings.HasSuffix(name, "-wal") || strings.HasSuffix(name, "-shm") || strings.HasSuffix(name, "-journal") {
			continue
		}
		path := filepath.Join(dir, name)
		code, branchName, ok := readLegacyBranchIdentity(path)
		if !ok {
			continue
		}
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		reg.Branches = append(reg.Branches, branchEntry{Code: code, Name: branchName, DBPath: path})
	}
	if len(reg.Branches) == 0 {
		return branchRegistry{}, false
	}
	reg.Default = reg.Branches[0].Code
	reg.Active = reg.Branches[0].Code
	if err := saveBranchRegistry(reg); err != nil {
		log.Printf("save migrated branch registry: %v", err)
	}
	log.Printf("migrated %d legacy database(s) into the branch registry", len(reg.Branches))
	return reg, true
}

func readLegacyBranchIdentity(path string) (code, name string, ok bool) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return "", "", false
	}
	defer conn.Close()
	if err := conn.QueryRow(`SELECT branch_code, branch_name FROM settings LIMIT 1`).Scan(&code, &name); err != nil {
		return "", "", false
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return "", "", false
	}
	if strings.TrimSpace(name) == "" {
		name = code
	}
	return code, name, true
}

// ListBranches returns every branch registered on this machine, plus which
// one is currently active and which one is the default for next launch.
func (a *App) ListBranches() (map[string]interface{}, error) {
	reg, err := loadBranchRegistry()
	if err != nil {
		return nil, err
	}
	branches := make([]map[string]interface{}, 0, len(reg.Branches))
	for _, b := range reg.Branches {
		branches = append(branches, map[string]interface{}{
			"code": b.Code,
			"name": b.Name,
		})
	}
	return map[string]interface{}{
		"branches": branches,
		"default":  reg.Default,
		"active":   reg.Active,
	}, nil
}

// CreateBranch sets up a brand new local branch -- its own database, seeded
// with the given identity -- and switches to it immediately. Used by the
// first-run setup screen (no branch registered yet) and by an admin adding
// another branch from Settings.
func (a *App) CreateBranch(name, code, apiKey, centralApiUrl string) (map[string]interface{}, error) {
	reg, err := loadBranchRegistry()
	if err != nil {
		return nil, err
	}
	// a.store == nil means no branch is open this session yet (first boot, or
	// the active branch's database went missing) -- nobody could have logged
	// in to prove they're an admin, so physical access to the till at that
	// point is treated the same way "Create database" always was: trusted.
	// Once a database is open, adding another branch is an admin action.
	if a.store != nil && !a.currentUserIsAdmin() {
		return nil, fmt.Errorf("only an admin can add another branch")
	}

	code, err = normalizeRegistryBranchCode(code)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("branch name is required")
	}
	if _, exists := reg.find(code); exists {
		return nil, fmt.Errorf("branch %q is already set up on this machine", code)
	}

	dir, err := branchRegistryDir()
	if err != nil {
		return nil, err
	}
	dbPath := branchDatabasePath(dir, code)
	if _, err := os.Stat(dbPath); err == nil {
		return nil, fmt.Errorf("a database already exists for branch %q", code)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if err := a.switchDatabase(dbPath, true); err != nil {
		return nil, err
	}

	current, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return nil, err
	}
	params := db.UpdateSettingsParams{
		BranchName: name, BranchCode: code, ApiKey: apiKey, PrinterID: current.PrinterID,
		PrinterSize: current.PrinterSize, ReceiptPrinter: current.ReceiptPrinter, ScreenPort: current.ScreenPort,
		ReceiptWidth: current.ReceiptWidth, ReceiptHeight: current.ReceiptHeight, Theme: current.Theme, TaxRate: current.TaxRate,
		RoundingMode: current.RoundingMode, RoundingCurrency: current.RoundingCurrency, ReceiptEnabled: current.ReceiptEnabled, SalesInventoryID: current.SalesInventoryID,
		ExchangeRate: current.ExchangeRate, MaxOverpaymentCents: current.MaxOverpaymentCents, ExchangeWindowHours: current.ExchangeWindowHours,
		MarketName: current.MarketName, MarketPhone: current.MarketPhone, MarketDescription: current.MarketDescription,
		ReturnPolicy: current.ReturnPolicy, Website: current.Website, Instagram: current.Instagram, SocialPlatforms: current.SocialPlatforms, SocialHandles: current.SocialHandles,
		ReceiptFont: current.ReceiptFont, ReceiptTitleSize: current.ReceiptTitleSize, ReceiptBodySize: current.ReceiptBodySize, ReceiptCutoff: current.ReceiptCutoff,
		ReceiptBodyFont: current.ReceiptBodyFont, BranchPhone: current.BranchPhone, CounterNumber: current.CounterNumber,
	}
	if err := a.store.UpdateSettings(a.ctx, params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(centralApiUrl) != "" {
		if err := a.store.UpdateCentralAPIURL(a.ctx, normalizeCentralAPIURL(centralApiUrl)); err != nil {
			return nil, err
		}
	}

	reg.Branches = append(reg.Branches, branchEntry{Code: code, Name: name, DBPath: dbPath})
	if reg.Default == "" {
		reg.Default = code
	}
	reg.Active = code
	if err := saveBranchRegistry(reg); err != nil {
		return nil, err
	}

	// This branch's local database is brand new (empty). If it's already
	// wired up to a central server, immediately backfill master data
	// (products/clients/price lists/payment methods) rather than leaving the
	// till empty until the periodic sync tick trickles it in. Safe to run
	// unconditionally: harmless (and near-instant) for a branch kashi has
	// never seen either, since there's nothing local yet to push and nothing
	// central yet to pull.
	//
	// This runs synchronously (the frontend shows a spinner for the whole
	// CreateBranch call) rather than in a background goroutine deliberately:
	// ResyncAll reads/writes through a.store, and a.store is swapped out from
	// under any in-flight goroutine the instant an admin switches branches
	// (see switchDatabase). A detached backfill still running when that
	// happens would keep writing kashi's data for *this* branch into
	// whichever branch's database is active by the time it gets there.
	if strings.TrimSpace(centralApiUrl) != "" && strings.TrimSpace(apiKey) != "" {
		if _, err := a.ResyncAll(ResyncOptions{}); err != nil {
			log.Printf("initial backfill for new branch %s: %v", code, err)
		}
	}

	return map[string]interface{}{"code": code, "name": name}, nil
}

// SwitchBranch opens a different already-registered branch's database in
// place, without restarting the process. The frontend must follow a
// successful call with runtime.WindowReloadApp so the UI re-reads the newly
// active branch's state.
func (a *App) SwitchBranch(code string) error {
	// See CreateBranch's comment: a.store == nil means nobody could have
	// logged in yet (no database open this session), so this is the initial
	// branch-picker screen, not a live admin-triggered switch.
	if a.store != nil && !a.currentUserIsAdmin() {
		return fmt.Errorf("only an admin can switch branches")
	}
	code, err := normalizeRegistryBranchCode(code)
	if err != nil {
		return err
	}
	reg, err := loadBranchRegistry()
	if err != nil {
		return err
	}
	entry, ok := reg.find(code)
	if !ok {
		return fmt.Errorf("branch %q is not set up on this machine", code)
	}
	if err := a.switchDatabase(entry.DBPath, false); err != nil {
		return err
	}
	reg.Active = code
	return saveBranchRegistry(reg)
}

// syncBranchRegistryEntry keeps the local branch registry in step with this
// branch's own name/code whenever they change via Settings. It's keyed by
// the open database's path rather than the code, since the code itself may
// be what just changed.
func (a *App) syncBranchRegistryEntry(name, code string) {
	if a.dbPath == "" || code == "" {
		return
	}
	reg, err := loadBranchRegistry()
	if err != nil {
		log.Printf("load branch registry: %v", err)
		return
	}
	changed := false
	found := false
	for i := range reg.Branches {
		if reg.Branches[i].DBPath != a.dbPath {
			continue
		}
		found = true
		if reg.Branches[i].Code != code || reg.Branches[i].Name != name {
			oldCode := reg.Branches[i].Code
			reg.Branches[i].Code = code
			reg.Branches[i].Name = name
			if reg.Default == oldCode {
				reg.Default = code
			}
			if reg.Active == oldCode {
				reg.Active = code
			}
			changed = true
		}
		break
	}
	if !found {
		reg.Branches = append(reg.Branches, branchEntry{Code: code, Name: name, DBPath: a.dbPath})
		if reg.Default == "" {
			reg.Default = code
		}
		reg.Active = code
		changed = true
	}
	if changed {
		if err := saveBranchRegistry(reg); err != nil {
			log.Printf("sync branch registry: %v", err)
		}
	}
}

// SetDefaultBranch sets which registered branch opens automatically the next
// time the app launches, independent of whichever branch is active right now.
func (a *App) SetDefaultBranch(code string) error {
	if !a.currentUserIsAdmin() {
		return fmt.Errorf("only an admin can change the default branch")
	}
	code, err := normalizeRegistryBranchCode(code)
	if err != nil {
		return err
	}
	reg, err := loadBranchRegistry()
	if err != nil {
		return err
	}
	if _, ok := reg.find(code); !ok {
		return fmt.Errorf("branch %q is not set up on this machine", code)
	}
	reg.Default = code
	return saveBranchRegistry(reg)
}
