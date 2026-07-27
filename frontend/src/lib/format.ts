import { roundLbpFromCents } from "@/lib/money";

export function formatNumber(value: number, maximumFractionDigits = 2): string {
  return value.toLocaleString(undefined, { maximumFractionDigits });
}

export function formatUsd(value: number): string {
  return `$${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

export function formatBalance(cents: number, currencyCode: string, exchangeRate: number): string {
  if (currencyCode === "LBP") {
    return `L.L. ${roundLbpFromCents(cents, exchangeRate).toLocaleString()}`;
  }
  return formatUsd(cents / 100);
}
