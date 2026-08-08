import { describe, expect, it } from "vitest";
import { isCashRoundedEquivalent } from "./money";

const RATE = 90_000;

describe("cash rounding", () => {
  it("accepts 10 USD plus 680,000 LBP for a 17.50 USD invoice", () => {
    const totalCents = 1_750;
    const payments = [1_000, 756]; // 680,000 LBP at the exact exchange rate

    expect(isCashRoundedEquivalent(1_756, totalCents, RATE, "half_up")).toBe(true);
    expect(payments).toEqual([1_000, 756]);
  });

  it("does not hide a genuine overpayment", () => {
    const payments = [1_000, 800];

    expect(isCashRoundedEquivalent(1_800, 1_750, RATE, "half_up")).toBe(false);
    expect(payments).toEqual([1_000, 800]);
  });

  it("preserves the rounded tender amount", () => {
    expect(isCashRoundedEquivalent(2_000, 1_998, RATE, "half_up")).toBe(true);
  });
});
