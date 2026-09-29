package main

import (
	"context"
	"fmt"

	db "kashi-pos/db/sqlc"
)

// Payment methods and currencies are kashi's, not this till's. kashi sends
// its whole set for both entities rather than a "changed since" delta,
// precisely so absence carries meaning: a row the till has and kashi did not
// send is one kashi does not have, and it stops being offered.
//
// Retiring is always a deactivation, never a delete. A cashbox account is the
// FK target of invoice payments, expenses and shift balances, so deleting one
// that has ever been used would orphan the records that explain how a shift
// balanced. Deactivating removes it from checkout and leaves history intact.
//
// Two things are deliberately out of kashi's reach, and the queries enforce
// it rather than these functions:
//
//   - "On Account" and "On Coupon" are internal bookkeeping placeholders that
//     the coupon and client-account settlement paths resolve by exact name.
//     kashi has no concept of them.
//   - the default currency is the accounting anchor every amount in this
//     database is stored in. It is structural, not a currency kashi gets an
//     opinion about.

// emptyMeansEmpty guards the NOT IN queries below, which match nothing when
// handed an empty slice -- SQL's `x NOT IN ()` is not false, it is a syntax
// error in SQLite and a no-op in others. An empty payload is a legitimate
// state ("kashi has no payment methods, so neither does this till"), so it is
// spelled out as its own branch instead of being left to the driver.
const emptyMeansEmpty = "\x00-no-such-name-\x00"

func reconcileCurrencies(ctx context.Context, q *db.Queries, sent []syncedCurrency) error {
	codes := make([]string, 0, len(sent))
	for _, c := range sent {
		codes = append(codes, c.Code)
	}
	if len(codes) == 0 {
		codes = []string{emptyMeansEmpty}
	}
	return q.DeactivateCurrenciesNotIn(ctx, codes)
}

func reconcileCashboxAccounts(ctx context.Context, q *db.Queries, sent []syncedCashboxAccount) error {
	names := make([]string, 0, len(sent))
	for _, a := range sent {
		names = append(names, a.Name)
	}
	if len(names) == 0 {
		names = []string{emptyMeansEmpty}
	}
	if err := q.DeactivateCashboxAccountsNotIn(ctx, names); err != nil {
		return err
	}

	// Anything kashi did send is active by definition -- it is in kashi's
	// current set. This is what brings a method back after kashi re-adds one
	// it had previously dropped.
	for _, a := range sent {
		existing, err := q.GetCashboxAccountByName(ctx, a.Name)
		if err != nil {
			return fmt.Errorf("payment method %q: %w", a.Name, err)
		}
		if existing.IsActive != 1 {
			if err := q.SetCashboxAccountActive(ctx, db.SetCashboxAccountActiveParams{
				IsActive: 1,
				ID:       existing.ID,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
