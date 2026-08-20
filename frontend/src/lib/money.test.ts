import { describe, expect, it } from "vitest";
import { isCashRoundedEquivalent, roundCurrencyUnitsFromCents, roundDiscountCents, type CurrencyRate } from "./money";

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

describe("roundDiscountCents (shared by InvoiceItems.vue and exchange.ts)", () => {
  it("matches half_up/down/up for a 10% discount on a fractional gross", () => {
    // 15.33 -> 1533 cents gross, 10% discount
    expect(roundDiscountCents(1533, 10, "half_up")).toBe(153);
    expect(roundDiscountCents(1533, 10, "down")).toBe(153);
    expect(roundDiscountCents(1533, 10, "up")).toBe(154);
  });

  it("truncates a fractional discount percent like the backend's roundRatio does", () => {
    expect(roundDiscountCents(10000, 33.9, "half_up")).toBe(roundDiscountCents(10000, 33, "half_up"));
  });

  it("never discounts a non-positive gross", () => {
    expect(roundDiscountCents(0, 50, "half_up")).toBe(0);
  });
});
