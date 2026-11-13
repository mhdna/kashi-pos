// What checkout records for the entries made in a cash-rounded currency
// (LBP: notes in 10,000s). Mirrors settleCashEntryCents in tx_invoice.go,
// step for step; cash_payment_netting.json (written by the Go test
// TestCashPaymentNettingGolden) holds the cases both sides must agree on.
//
// An account's entries are added up in LL and converted to cents once, each
// entry recording the step it makes in the running total. Adding each
// entry's own cents could be a cent out: LL 200,000 paid (222¢) less
// LL 60,000 back (67¢) is 155¢, but the LL 140,000 kept is 156¢.

import { roundCentsFromCurrencyUnits, roundCurrencyUnitsFromCents, type CurrencyRate, type RoundingMode } from "./money";

/** One entry: cents > 0 is a payment, < 0 change; units is its unsigned
 * amount in the currency's own units, 0 when unknown. */
export interface CashEntry {
  cents: number;
  units: number;
}

function roundToUnit(value: number, unit: number, mode: RoundingMode): number {
  const ratio = value / unit;
  if (mode === "down") return Math.floor(ratio + 1e-9) * unit;
  if (mode === "up") return Math.ceil(ratio - 1e-9) * unit;
  return Math.floor(ratio + 0.5) * unit;
}

function signedCents(units: number, currency: CurrencyRate, mode: RoundingMode): number {
  const cents = roundCentsFromCurrencyUnits(Math.abs(units), currency, mode);
  return units < 0 ? -cents : cents;
}

/** The cents checkout records for each of one account's entries. */
export function settleCashEntryCents(entries: CashEntry[], currency: CurrencyRate, mode: RoundingMode): number[] {
  const out = entries.map((entry) => entry.cents);
  const signedUnits = entries.map(() => 0);
  let netted = true;
  entries.forEach((entry, i) => {
    if (entry.cents > 0) {
      let units = roundCurrencyUnitsFromCents(entry.cents, currency, mode);
      if (entry.units > 0 && roundCentsFromCurrencyUnits(entry.units, currency, mode) === entry.cents) {
        units = roundToUnit(entry.units, currency.cashRoundingUnit, mode);
      }
      signedUnits[i] = units;
      out[i] = roundCentsFromCurrencyUnits(units, currency, mode);
    } else if (entry.cents < 0) {
      if (entry.units <= 0 || roundCentsFromCurrencyUnits(entry.units, currency, mode) !== -entry.cents) {
        netted = false;
        return;
      }
      signedUnits[i] = -entry.units;
    }
  });
  if (!netted) return out;
  let running = 0;
  let before = 0;
  entries.forEach((entry, i) => {
    if (entry.cents === 0) {
      out[i] = 0;
      return;
    }
    running += signedUnits[i] ?? 0;
    const after = signedCents(running, currency, mode);
    out[i] = after - before;
    before = after;
  });
  return out;
}

/** A calculator entry, with the currency of its payment method. */
export interface PaymentEntry {
  accountId: number;
  amountCents: number;
  /** Signed amount in the method's currency (negative for change). */
  amount: number;
  currency: CurrencyRate;
}

/** The unsigned LL of an entry in a cash-rounded currency, for checkout;
 * undefined for any other currency. */
export function entryUnits(entry: PaymentEntry): number | undefined {
  if (entry.currency.cashRoundingUnit <= 1) return undefined;
  const units = Math.round(Math.abs(entry.amount));
  return units > 0 ? units : undefined;
}

/** The cents checkout records for every entry, in order: entries in a
 * cash-rounded currency settled per account, the rest as they are. */
export function settledPaymentCents(entries: PaymentEntry[], mode: RoundingMode): number[] {
  const out = entries.map((entry) => entry.amountCents);
  const byAccount = new Map<number, number[]>();
  entries.forEach((entry, i) => {
    if (entry.amountCents === 0 || entry.currency.cashRoundingUnit <= 1) return;
    const indexes = byAccount.get(entry.accountId) ?? [];
    indexes.push(i);
    byAccount.set(entry.accountId, indexes);
  });
  for (const indexes of byAccount.values()) {
    const group = indexes.map((i) => entries[i]!);
    const cents = settleCashEntryCents(
      group.map((entry) => ({ cents: entry.amountCents, units: entryUnits(entry) ?? 0 })),
      group[0]!.currency,
      mode,
    );
    indexes.forEach((i, j) => {
      out[i] = cents[j] ?? 0;
    });
  }
  return out;
}
