package main

import (
	"fmt"
	"log"
	"strings"

	db "kashi-pos/db/sqlc"
)

// autoCoverNegativeLBPFlag: at close shift, a negative Cash LL balance (more
// LBP paid out as change than taken in) is swapped into Cash $$, so LL ends
// at 0 and the shortfall comes off the USD cash. Off by default; kashi can
// push it per branch.
const autoCoverNegativeLBPFlag = "auto_cover_negative_lbp"

func (a *App) autoCoverNegativeLBP() bool {
	v, err := a.store.GetFeatureFlag(a.ctx, autoCoverNegativeLBPFlag)
	return err == nil && v != 0
}

// SetAutoCoverNegativeLBPEnabled toggles autoCoverNegativeLBPFlag.
func (a *App) SetAutoCoverNegativeLBPEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.SetFeatureFlag(a.ctx, db.SetFeatureFlagParams{
		Key: autoCoverNegativeLBPFlag, Enabled: boolInt(enabled),
	})
}

// PrepareShiftClose runs what has to happen before a shift's count is
// shown: the close-shift screen calls it before loading balances, and
// CloseShiftByID again in case a sale came in meanwhile. It reports whether
// it changed any balance.
func (a *App) PrepareShiftClose(shiftID int64) (bool, error) {
	if a.store == nil {
		return false, fmt.Errorf("database not initialized")
	}
	if !a.autoCoverNegativeLBP() {
		return false, nil
	}
	return a.coverNegativeLBP(shiftID)
}

// coverNegativeLBP swaps a negative Cash LL balance into Cash $$, through
// the same validated path as the Swap balance dialog: a swap of a negative
// source moves the whole negative amount, leaving LL at 0.
func (a *App) coverNegativeLBP(shiftID int64) (bool, error) {
	accounts, err := a.store.ListCashboxAccounts(a.ctx)
	if err != nil {
		return false, fmt.Errorf("cash accounts: %w", err)
	}
	var lbpID, usdID int64
	for _, account := range accounts {
		if !isCashAccount(account.Name, account.CurrencyCode) {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(account.CurrencyCode)) {
		case "LBP":
			if lbpID == 0 {
				lbpID = account.ID
			}
		case "USD":
			if usdID == 0 {
				usdID = account.ID
			}
		}
	}
	if lbpID == 0 || usdID == 0 {
		return false, nil
	}

	balance, err := a.shiftCashBalanceCents(shiftID, lbpID, "LBP")
	if err != nil || balance >= 0 {
		return false, err
	}
	if err := a.replaceShiftBalance(shiftID, lbpID, usdID, -balance, false); err != nil {
		return false, fmt.Errorf("cover negative LBP: %w", err)
	}
	log.Printf("shift %d: moved negative Cash LL balance (%d cents) into Cash $$", shiftID, balance)
	return true, nil
}

// shiftCashBalanceCents is a cash account's balance for the shift in USD
// cents, opening float included -- what the balance list shows and what
// checkShiftBalanceReplacement lets a swap move.
func (a *App) shiftCashBalanceCents(shiftID, accountID int64, currencyCode string) (int64, error) {
	totals, err := a.incomeAccountTotals(shiftID)
	if err != nil {
		return 0, err
	}
	var balance int64
	for _, row := range totals {
		if row.ID == accountID {
			balance = databaseInt64(row.Total)
			break
		}
	}
	shift, err := a.store.GetShiftByID(a.ctx, shiftID)
	if err != nil {
		return 0, err
	}
	usdFloat, lbpFloat, err := a.openingFloatCentsForShift(shift)
	if err != nil {
		return 0, err
	}
	if strings.EqualFold(currencyCode, "LBP") {
		balance += lbpFloat
	} else {
		balance += usdFloat
	}
	return balance, nil
}
