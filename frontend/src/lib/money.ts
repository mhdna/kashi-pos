// Display/preview-only helpers. These are NOT the source of truth for
// checkout math - the backend (money.go, QuoteInvoiceTotal/QuotePaymentStatus)
// computes and validates every amount that actually gets persisted. Use
// these only for things like live keypad formatting where round-tripping to
// the backend on every keystroke would be wasteful.
//
// Every function takes `mode` as a required parameter on purpose: the old
// version let call sites omit it and silently fell back to half-up
// regardless of the shop's configured rounding mode, which was a real bug.

export type RoundingMode = "half_up" | "down" | "up";

/** A currency's rate/rounding config, as returned by ListCurrencies(). */
export interface CurrencyRate {
  code: string;
  symbol: string;
  /** Major units of this currency equal to one USD (e.g. 90000 for LBP). */
  unitsPerUsd: number;
  /** Physical cash in this currency rounds to the nearest multiple of this,
   * in the currency's own major units (e.g. 10000 for LBP, 1 for USD). */
  cashRoundingUnit: number;
}

export function normalizeRoundingMode(mode: string): RoundingMode {
  return mode === "down" || mode === "up" || mode === "half_up" ? mode : "half_up";
}

function roundHalfEvenSafe(units: number, mode: RoundingMode): number {
  if (mode === "down") return Math.floor(units + 1e-9);
  if (mode === "up") return Math.ceil(units - 1e-9);
  return Math.floor(units + 0.5);
}

/** Rounds a line-item discount (gross cents x discount percent / 100) the
 * same way the backend's roundRatio does. Previously reimplemented
 * separately (and identically) in InvoiceItems.vue and exchange.ts. */
export function roundDiscountCents(grossCents: number, discountPercent: number, mode: RoundingMode): number {
  const numerator = grossCents * Math.trunc(discountPercent);
  return roundHalfEvenSafe(numerator / 100, mode);
}

/**
 * Converts an accounting-cents amount into a currency's major units,
 * rounded to that currency's configured cash-rounding unit. Mirrors the
 * backend's roundCurrencyUnitsFromCents (money.go) - plugging in a
 * currency with cashRoundingUnit=1 reproduces "round to the nearest whole
 * unit" for any currency, not just USD/LBP.
 */
export function roundCurrencyUnitsFromCents(cents: number, currency: CurrencyRate, mode: RoundingMode): number {
  if (!Number.isFinite(cents) || currency.unitsPerUsd <= 0 || currency.cashRoundingUnit <= 0) return 0;
  const exactUnits = (Math.abs(cents) * currency.unitsPerUsd) / 100 / currency.cashRoundingUnit;
  const roundedUnits = roundHalfEvenSafe(exactUnits, mode);
  const sign = cents < 0 ? -1 : 1;
  return roundedUnits === 0 ? 0 : sign * roundedUnits * currency.cashRoundingUnit;
}

/** Inverse of roundCurrencyUnitsFromCents: a physically counted amount in a
 * currency's major units back to accounting cents. */
export function roundCentsFromCurrencyUnits(units: number, currency: CurrencyRate, mode: RoundingMode): number {
  if (!Number.isFinite(units) || units < 0 || currency.unitsPerUsd <= 0) return 0;
  return roundHalfEvenSafe((units * 100) / currency.unitsPerUsd, mode);
}

/** True when displaying `cents` in `currency` actually changes the value -
 * i.e. cash-rounding to the currency's cashRoundingUnit lost precision. USD
 * (cashRoundingUnit=1) never rounds; LBP (cashRoundingUnit=10000) almost
 * always does unless the amount happens to be an exact multiple. */
export function isRoundedToUnit(cents: number, currency: CurrencyRate, mode: RoundingMode): boolean {
  if (!Number.isFinite(cents) || currency.cashRoundingUnit <= 1) return false;
  const absCents = Math.round(Math.abs(cents));
  const roundedUnits = Math.abs(roundCurrencyUnitsFromCents(absCents, currency, mode));
  const roundedCents = roundCentsFromCurrencyUnits(roundedUnits, currency, mode);
  return roundedCents !== absCents;
}

export function isCashRoundedEquivalent(
  paidCents: number,
  totalCents: number,
  currency: CurrencyRate,
  mode: RoundingMode,
): boolean {
  return (
    paidCents !== totalCents &&
    roundCurrencyUnitsFromCents(paidCents, currency, mode) === roundCurrencyUnitsFromCents(totalCents, currency, mode)
  );
}
