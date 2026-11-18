import { describe, expect, it } from "vitest";
import { dealTerms, rewardsActiveCount, type RewardDeal } from "./rewards";

const deal = (over: Partial<RewardDeal>): RewardDeal => ({ uid: "x", name: "X", kind: "percent", percent: 15, amount: 0, minTotal: 0, cashierPicks: false, ...over });

describe("dealTerms", () => {
  it("says what a deal takes off and when", () => {
    expect(dealTerms(deal({}))).toBe("15% off the whole invoice");
    expect(dealTerms(deal({ minTotal: 10000 }))).toBe("15% off invoices of $100 or more");
    expect(dealTerms(deal({ kind: "amount", amount: 500 }))).toBe("$5 off the whole invoice");
    expect(dealTerms(deal({ kind: "amount", amount: 250, minTotal: 4999 }))).toBe("$2.50 off invoices of $49.99 or more");
  });
});

describe("rewardsActiveCount", () => {
  it("counts the code, each pick, and an automatic deal once", () => {
    expect(rewardsActiveCount("", [], "")).toBe(0);
    expect(rewardsActiveCount("K7M4QH", [], "")).toBe(1);
    expect(rewardsActiveCount("", ["a", "b"], "")).toBe(2);
    expect(rewardsActiveCount("", [], "15% off $100")).toBe(1);
    expect(rewardsActiveCount("", ["students"], "Students 15%", ["Students 15%"])).toBe(1);
    expect(rewardsActiveCount("K", ["students"], "15% off $100", ["Students 15%"])).toBe(3);
  });
});
