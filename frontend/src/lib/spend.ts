import type { CurrencyRate } from "@/lib/money";

// Expense and loan amounts are stored in their own currency, x100, as
// entered (200,000 LL is 20,000,000). Income and balances elsewhere are USD
// cents, so a net figure converts with the currency's rate.

export type CurrencyTotal = { currencyCode: string; total: number };

export function spendToUsdCents(amount: number, currencyCode: string, currencies: CurrencyRate[]): number {
  const rate = currencies.find((c) => c.code === currencyCode)?.unitsPerUsd || 1;
  return Math.round(Number(amount || 0) / rate);
}

export function sumToUsdCents(rows: { amount?: number; total?: number; currencyCode: string }[], currencies: CurrencyRate[]): number {
  return rows.reduce((sum, r) => sum + spendToUsdCents(Number(r.amount ?? r.total ?? 0), r.currencyCode, currencies), 0);
}

export function formatSpendAmount(amount: number, currencyCode: string, currencies: CurrencyRate[]): string {
  const value = Number(amount || 0) / 100;
  if (currencyCode === "USD" || !currencyCode) {
    return `$${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
  }
  const symbol = currencies.find((c) => c.code === currencyCode)?.symbol || currencyCode;
  return `${Math.round(value).toLocaleString()} ${symbol}`;
}

// "$120.00 + 200,000 LL", or "$0.00" when there's nothing.
export function formatCurrencyTotals(totals: CurrencyTotal[], currencies: CurrencyRate[]): string {
  const nonZero = (totals ?? []).filter((t) => Number(t.total) !== 0);
  if (!nonZero.length) return "$0.00";
  return nonZero.map((t) => formatSpendAmount(t.total, t.currencyCode, currencies)).join(" + ");
}
