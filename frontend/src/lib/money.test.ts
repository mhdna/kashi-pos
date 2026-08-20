import { describe, expect, it } from "vitest";
import { isCashRoundedEquivalent, roundCurrencyUnitsFromCents, type CurrencyRate } from "./money";

const LBP: CurrencyRate = { code: "LBP", symbol: "L.L.", unitsPerUsd: 90_000, cashRoundingUnit: 10_000 };

describe("cash rounding", () => {
  it("accepts 10 USD plus 680,000 LBP for a 17.50 USD invoice", () => {
    const totalCents = 1_750;
    const payments = [1_000, 756]; // 680,000 LBP at the exact exchange rate

    expect(isCashRoundedEquivalent(1_756, totalCents, LBP, "half_up")).toBe(true);
    expect(payments).toEqual([1_000, 756]);
  });

  it("does not hide a genuine overpayment", () => {
    const payments = [1_000, 800];

    expect(isCashRoundedEquivalent(1_800, 1_750, LBP, "half_up")).toBe(false);
    expect(payments).toEqual([1_000, 800]);
  });

  it("preserves the rounded tender amount", () => {
    expect(isCashRoundedEquivalent(2_000, 1_998, LBP, "half_up")).toBe(true);
  });

  it("reproduces whole-dollar rounding for a currency with cashRoundingUnit=1", () => {
    const USD: CurrencyRate = { code: "USD", symbol: "$", unitsPerUsd: 1, cashRoundingUnit: 1 };
    expect(roundCurrencyUnitsFromCents(1050, USD, "half_up")).toBe(11);
    expect(roundCurrencyUnitsFromCents(1049, USD, "half_up")).toBe(10);
  });
});
