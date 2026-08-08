export type RoundingMode = "half_up" | "down" | "up";

export const LBP_ROUNDING_UNIT = 10_000;

export function normalizeRoundingMode(mode?: string): RoundingMode {
  return mode === "down" || mode === "up" || mode === "half_up" ? mode : "half_up";
}

/** Matches the backend's integer LBP cash-rounding rule. */
export function roundLbpFromCents(cents: number, exchangeRate: number, mode?: string): number {
  if (!Number.isFinite(cents) || !Number.isFinite(exchangeRate) || exchangeRate <= 0)
    return 0;
  const sign = cents < 0 ? -1 : 1;
  const exactLbp = (Math.abs(cents) * exchangeRate) / 100;
  const units = exactLbp / LBP_ROUNDING_UNIT;
  const roundingMode = normalizeRoundingMode(mode);
  const roundedUnits =
    roundingMode === "down"
      ? Math.floor(units + 1e-9)
      : roundingMode === "up"
        ? Math.ceil(units - 1e-9)
        : Math.floor(units + 0.5);
  return roundedUnits === 0 ? 0 : sign * roundedUnits * LBP_ROUNDING_UNIT;
}

export function roundCents(value: number, mode?: string): number {
  const roundingMode = normalizeRoundingMode(mode);
  if (roundingMode === "down") return Math.floor(value + 1e-9);
  if (roundingMode === "up") return Math.ceil(value - 1e-9);
  return Math.floor(value + 0.5);
}

export function roundCentsByCurrency(cents: number, currency: string, exchangeRate: number, mode?: string): number {
  const sign = cents < 0 ? -1 : 1;
  const amount = Math.abs(cents);
  if (currency.toUpperCase() === "USD") return sign * roundCents(amount / 100, mode) * 100;
  return sign * roundCentsFromLbp(roundLbpFromCents(amount, exchangeRate, mode), exchangeRate);
}

/** Convert a physical LBP amount to accounting cents using the app rate. */
export function roundCentsFromLbp(amount: number, exchangeRate: number): number {
  if (!Number.isFinite(amount) || !Number.isFinite(exchangeRate) || amount < 0 || exchangeRate <= 0) {
    return 0;
  }
  return Math.floor((amount * 100) / exchangeRate + 0.5);
}

export function isCashRoundedEquivalent(
  paidCents: number,
  totalCents: number,
  exchangeRate: number,
  mode?: string,
): boolean {
  return (
    paidCents !== totalCents &&
    roundLbpFromCents(paidCents, exchangeRate, mode) ===
      roundLbpFromCents(totalCents, exchangeRate, mode)
  );
}
