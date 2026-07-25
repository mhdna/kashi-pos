export type RoundingMode = "half_up" | "down" | "up";

export const LBP_ROUNDING_UNIT = 10_000;

export function normalizeRoundingMode(mode?: string): RoundingMode {
  return mode === "down" || mode === "up" || mode === "half_up" ? mode : "half_up";
}

/** Matches the backend's integer LBP cash-rounding rule. */
export function roundLbpFromCents(cents: number, exchangeRate: number, mode?: string): number {
  if (!Number.isFinite(cents) || cents < 0 || !Number.isFinite(exchangeRate) || exchangeRate <= 0)
    return 0;
  const exactLbp = (cents * exchangeRate) / 100;
  const units = exactLbp / LBP_ROUNDING_UNIT;
  const roundingMode = normalizeRoundingMode(mode);
  const roundedUnits =
    roundingMode === "down"
      ? Math.floor(units + 1e-9)
      : roundingMode === "up"
        ? Math.ceil(units - 1e-9)
        : Math.floor(units + 0.5);
  return roundedUnits * LBP_ROUNDING_UNIT;
}

export function roundCents(value: number, mode?: string): number {
  const roundingMode = normalizeRoundingMode(mode);
  if (roundingMode === "down") return Math.floor(value + 1e-9);
  if (roundingMode === "up") return Math.ceil(value - 1e-9);
  return Math.floor(value + 0.5);
}
