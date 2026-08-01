package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	db "kashi-pos/db/sqlc"
)

// loadDotEnv loads a local .env file without replacing explicitly exported
// environment variables. It intentionally supports the small dotenv subset
// needed for application settings: KEY=value, optional quotes, and comments.
func loadDotEnv() {
	paths := []string{".env"}
	if executable, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(executable), ".env"))
	}
	for _, path := range paths {
		if err := loadDotEnvFile(path); err == nil {
			log.Println("loaded environment defaults from", path)
			return
		} else if !os.IsNotExist(err) {
			log.Printf("could not load %s: %v", path, err)
			return
		}
	}
}

func loadDotEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("set %s: %w", key, err)
			}
		}
	}
	return scanner.Err()
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt64(key string, fallback int64) int64 {
	value, err := strconv.ParseInt(strings.TrimSpace(os.Getenv(key)), 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func envFloat64(key string, fallback float64) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv(key)), 64)
	if err != nil {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return fallback
	}
	return value
}

func envJSON(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var parsed interface{}
	if json.Unmarshal([]byte(value), &parsed) != nil {
		return fallback
	}
	return value
}

func envSocialPlatforms() string {
	value := strings.TrimSpace(os.Getenv("KASHI_SOCIAL_PLATFORMS"))
	if value == "" {
		return "[]"
	}
	var parsed []string
	if json.Unmarshal([]byte(value), &parsed) == nil {
		return value
	}
	platforms := make([]string, 0)
	for _, platform := range strings.Split(value, ",") {
		if platform = strings.TrimSpace(platform); platform != "" {
			platforms = append(platforms, platform)
		}
	}
	encoded, _ := json.Marshal(platforms)
	return string(encoded)
}

func defaultSettingsParams() db.UpsertSettingsParams {
	return db.UpsertSettingsParams{
		BranchName:          envString("KASHI_BRANCH_NAME", "Main"),
		BranchCode:          envString("KASHI_BRANCH_CODE", "CN"),
		ApiKey:              envString("KASHI_API_KEY", ""),
		PrinterID:           envString("KASHI_PRINTER_ID", ""),
		PrinterSize:         envString("KASHI_PRINTER_SIZE", ""),
		ReceiptPrinter:      envString("KASHI_RECEIPT_PRINTER", ""),
		ScreenPort:          envString("KASHI_SCREEN_PORT", ""),
		ReceiptWidth:        envInt64("KASHI_RECEIPT_WIDTH", 80),
		ReceiptHeight:       envInt64("KASHI_RECEIPT_HEIGHT", 297),
		Theme:               envString("KASHI_THEME", "light"),
		TaxRate:             envFloat64("KASHI_TAX_RATE", 0),
		RoundingMode:        envString("KASHI_ROUNDING_MODE", "half_up"),
		ReceiptEnabled:      boolInt(envBool("KASHI_RECEIPT_ENABLED", false)),
		SalesInventoryID:    envInt64("KASHI_SALES_INVENTORY_ID", 0),
		ExchangeRate:        envInt64("KASHI_EXCHANGE_RATE", 90000),
		MaxOverpaymentCents: envInt64("KASHI_MAX_OVERPAYMENT_CENTS", 0),
		MarketName:          envString("KASHI_MARKET_NAME", "Main"),
		MarketPhone:         envString("KASHI_MARKET_PHONE", ""),
		MarketDescription:   envString("KASHI_MARKET_DESCRIPTION", "Shop for all"),
		ReturnPolicy:        envString("KASHI_RETURN_POLICY", ""),
		Website:             envString("KASHI_WEBSITE", ""),
		Instagram:           envString("KASHI_INSTAGRAM", ""),
		SocialPlatforms:     envSocialPlatforms(),
		SocialHandles:       envJSON("KASHI_SOCIAL_HANDLES", "{}"),
		ReceiptFont:         envString("KASHI_RECEIPT_FONT", "Belleza"),
		ReceiptTitleSize:    envFloat64("KASHI_RECEIPT_TITLE_SIZE", 21),
		ReceiptBodySize:     envFloat64("KASHI_RECEIPT_BODY_SIZE", 11),
		ReceiptCutoff:       boolInt(envBool("KASHI_RECEIPT_CUTOFF", true)),
		ReceiptBodyFont:     envString("KASHI_RECEIPT_BODY_FONT", "Printer Font A"),
		BranchPhone:         envString("KASHI_BRANCH_PHONE", ""),
		CounterNumber:       envString("KASHI_REGISTER_NAME", envString("KASHI_COUNTER_NUMBER", "POS-01")),
	}
}

func (a *App) applyEnvironmentDefaults() error {
	settings, err := a.store.GetSettings(a.ctx)
	if err != nil {
		return err
	}
	params := db.UpdateSettingsParams{
		BranchName: settings.BranchName, BranchCode: settings.BranchCode, ApiKey: settings.ApiKey, PrinterID: settings.PrinterID,
		PrinterSize: settings.PrinterSize, ReceiptPrinter: settings.ReceiptPrinter, ScreenPort: settings.ScreenPort,
		ReceiptWidth: settings.ReceiptWidth, ReceiptHeight: settings.ReceiptHeight, Theme: settings.Theme, TaxRate: settings.TaxRate,
		RoundingMode: settings.RoundingMode, ReceiptEnabled: settings.ReceiptEnabled, SalesInventoryID: settings.SalesInventoryID,
		ExchangeRate: settings.ExchangeRate, MaxOverpaymentCents: settings.MaxOverpaymentCents, MarketName: settings.MarketName, MarketPhone: settings.MarketPhone,
		MarketDescription: settings.MarketDescription, ReturnPolicy: settings.ReturnPolicy, Website: settings.Website,
		Instagram: settings.Instagram, SocialPlatforms: settings.SocialPlatforms, SocialHandles: settings.SocialHandles,
		ReceiptFont: settings.ReceiptFont, ReceiptTitleSize: settings.ReceiptTitleSize, ReceiptBodySize: settings.ReceiptBodySize,
		ReceiptCutoff: settings.ReceiptCutoff, ReceiptBodyFont: settings.ReceiptBodyFont, BranchPhone: settings.BranchPhone,
		CounterNumber: settings.CounterNumber,
	}
	changed := false
	setString := func(target *string, value string) {
		if *target == "" && value != "" {
			*target, changed = value, true
		}
	}
	setInt := func(target *int64, value int64) {
		if *target == 0 && value != 0 {
			*target, changed = value, true
		}
	}
	setFloat := func(target *float64, value float64) {
		if *target == 0 && value != 0 {
			*target, changed = value, true
		}
	}
	setString(&params.BranchName, envOnlyString("KASHI_BRANCH_NAME"))
	setString(&params.BranchCode, envOnlyString("KASHI_BRANCH_CODE"))
	setString(&params.ApiKey, envOnlyString("KASHI_API_KEY"))
	setString(&params.PrinterID, envOnlyString("KASHI_PRINTER_ID"))
	setString(&params.PrinterSize, envOnlyString("KASHI_PRINTER_SIZE"))
	setString(&params.ReceiptPrinter, envOnlyString("KASHI_RECEIPT_PRINTER"))
	setString(&params.ScreenPort, envOnlyString("KASHI_SCREEN_PORT"))
	setInt(&params.ReceiptWidth, envOnlyInt("KASHI_RECEIPT_WIDTH"))
	setInt(&params.ReceiptHeight, envOnlyInt("KASHI_RECEIPT_HEIGHT"))
	setString(&params.Theme, envOnlyString("KASHI_THEME"))
	setFloat(&params.TaxRate, envOnlyFloat("KASHI_TAX_RATE"))
	setString(&params.RoundingMode, envOnlyString("KASHI_ROUNDING_MODE"))
	setInt(&params.ReceiptEnabled, envOnlyBoolInt("KASHI_RECEIPT_ENABLED"))
	setInt(&params.SalesInventoryID, envOnlyInt("KASHI_SALES_INVENTORY_ID"))
	setInt(&params.ExchangeRate, envOnlyInt("KASHI_EXCHANGE_RATE"))
	setInt(&params.MaxOverpaymentCents, envOnlyInt("KASHI_MAX_OVERPAYMENT_CENTS"))
	setString(&params.MarketName, envOnlyString("KASHI_MARKET_NAME"))
	setString(&params.MarketPhone, envOnlyString("KASHI_MARKET_PHONE"))
	setString(&params.MarketDescription, envOnlyString("KASHI_MARKET_DESCRIPTION"))
	setString(&params.ReturnPolicy, envOnlyString("KASHI_RETURN_POLICY"))
	setString(&params.Website, envOnlyString("KASHI_WEBSITE"))
	setString(&params.Instagram, envOnlyString("KASHI_INSTAGRAM"))
	setString(&params.SocialPlatforms, envOnlySocialPlatforms())
	setString(&params.SocialHandles, envOnlyJSON("KASHI_SOCIAL_HANDLES"))
	setString(&params.ReceiptFont, envOnlyString("KASHI_RECEIPT_FONT"))
	setFloat(&params.ReceiptTitleSize, envOnlyFloat("KASHI_RECEIPT_TITLE_SIZE"))
	setFloat(&params.ReceiptBodySize, envOnlyFloat("KASHI_RECEIPT_BODY_SIZE"))
	setInt(&params.ReceiptCutoff, envOnlyBoolInt("KASHI_RECEIPT_CUTOFF"))
	setString(&params.ReceiptBodyFont, envOnlyString("KASHI_RECEIPT_BODY_FONT"))
	setString(&params.BranchPhone, envOnlyString("KASHI_BRANCH_PHONE"))
	registerName := envOnlyString("KASHI_REGISTER_NAME")
	if registerName == "" {
		registerName = envOnlyString("KASHI_COUNTER_NUMBER")
	}
	setString(&params.CounterNumber, registerName)
	if !changed {
		return nil
	}
	return a.store.UpdateSettings(a.ctx, params)
}

func envOnlyString(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func envOnlyInt(key string) int64 {
	if value, err := strconv.ParseInt(envOnlyString(key), 10, 64); err == nil {
		return value
	}
	return 0
}

func envOnlyFloat(key string) float64 {
	if value, err := strconv.ParseFloat(envOnlyString(key), 64); err == nil {
		return value
	}
	return 0
}

func envOnlyBoolInt(key string) int64 {
	if value, err := strconv.ParseBool(envOnlyString(key)); err == nil {
		return boolInt(value)
	}
	return 0
}

func envOnlyJSON(key string) string {
	value := envOnlyString(key)
	if value == "" {
		return ""
	}
	var parsed interface{}
	if json.Unmarshal([]byte(value), &parsed) == nil {
		return value
	}
	return ""
}

func envOnlySocialPlatforms() string {
	value := strings.TrimSpace(os.Getenv("KASHI_SOCIAL_PLATFORMS"))
	if value == "" {
		return ""
	}
	var parsed []string
	if json.Unmarshal([]byte(value), &parsed) == nil {
		return value
	}
	platforms := make([]string, 0)
	for _, platform := range strings.Split(value, ",") {
		if platform = strings.TrimSpace(platform); platform != "" {
			platforms = append(platforms, platform)
		}
	}
	encoded, _ := json.Marshal(platforms)
	return string(encoded)
}
