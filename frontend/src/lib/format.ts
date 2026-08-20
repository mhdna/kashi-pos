import { roundCurrencyUnitsFromCents, type CurrencyRate, type RoundingMode } from "@/lib/money";

export function formatNumber(value: number, maximumFractionDigits = 2): string {
  return value.toLocaleString(undefined, { maximumFractionDigits });
}

export function formatUsd(value: number): string {
  return `$${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

/**
 * Formats an accounting-cents amount in whatever currency `currencyCode`
 * names, looking it up in `currencies` (from ListCurrencies()) rather than
 * hardcoding "LBP" - any currency a shop has added displays correctly here.
 * `mode` is required so this can never silently default to half-up
 * regardless of the shop's configured rounding mode.
 */
export function formatBalance(
  cents: number,
  currencyCode: string,
  currencies: CurrencyRate[],
  mode: RoundingMode,
): string {
  const currency = currencies.find((c) => c.code === currencyCode);
  if (!currency || currency.cashRoundingUnit <= 1) return formatUsd(cents / 100);
  return `${currency.symbol} ${roundCurrencyUnitsFromCents(cents, currency, mode).toLocaleString()}`;
}
