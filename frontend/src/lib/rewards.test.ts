import { describe, expect, it } from "vitest";
import { appliedDealNames, dealSize, dealTerms, dealTitle, dealWhen, rewardsActiveCount, type RewardDeal } from "./rewards";

const deal = (over: Partial<RewardDeal>): RewardDeal => ({ uid: "x", name: "X", kind: "percent", percent: 15, amount: 0, minTotal: 0, cashierPicks: false, ...over });

describe("dealTerms", () => {
  it("says what a deal takes off and when", () => {
    expect(dealTerms(deal({}))).toBe("15% off the whole invoice");
    expect(dealTerms(deal({ minTotal: 10000 }))).toBe("15% off invoices of $100 or more");
    expect(dealTerms(deal({ kind: "amount", amount: 500 }))).toBe("$5 off the whole invoice");
    expect(dealTerms(deal({ kind: "amount", amount: 250, minTotal: 4999 }))).toBe("$2.50 off invoices of $49.99 or more");
  });

  it("the reported case: Independence Day $50 off every $100", () => {
    const indep = deal({ name: "Independence Day $50", kind: "amount", amount: 5000, minTotal: 10000, repeats: true });
    expect(dealTerms(indep)).toBe("$50 off every $100 of the invoice");
    expect(dealTerms({ ...indep, replacesItemDiscounts: true })).toBe("$50 off every $100 of the invoice, instead of item discounts");
    expect(dealTerms(deal({ minTotal: 10000, replacesItemDiscounts: true }))).toBe("15% off invoices of $100 or more, instead of item discounts");
    expect(dealTerms(deal({ replacesItemDiscounts: true }))).toBe("15% off the whole invoice, instead of item discounts");
    expect(dealWhen(indep)).toBe("Every $100");
    expect(dealSize(indep)).toBe("$50");
    // Repeating needs an amount and a minimum; otherwise it reads as before.
    expect(dealTerms(deal({ kind: "amount", amount: 5000, repeats: true }))).toBe("$50 off the whole invoice");
    expect(dealWhen(deal({ minTotal: 10000, repeats: true }))).toBe("> $100");
    expect(dealTerms(deal({ kind: "amount", amount: 5000, minTotal: 10000, repeats: false }))).toBe("$50 off invoices of $100 or more");
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

describe("dealSize", () => {
  it("is the headline of the card", () => {
    expect(dealSize(deal({}))).toBe("15%");
    expect(dealSize(deal({ kind: "amount", amount: 500 }))).toBe("$5");
    expect(dealSize(deal({ kind: "amount", amount: 750 }))).toBe("$7.50");
  });
});

describe("dealWhen", () => {
  it("is the condition in a few words", () => {
    expect(dealWhen(deal({}))).toBe("Whole invoice");
    expect(dealWhen(deal({ minTotal: 5000 }))).toBe("> $50");
    expect(dealWhen(deal({ minTotal: 4999 }))).toBe("> $49.99");
  });
});

describe("dealTitle", () => {
  it("drops the size the card shows from the name", () => {
    expect(dealTitle(deal({ name: "Students 15%" }))).toBe("Students");
    expect(dealTitle(deal({ name: "15% Students" }))).toBe("Students");
    expect(dealTitle(deal({ name: "Staff", kind: "amount", amount: 500 }))).toBe("Staff");
    expect(dealTitle(deal({ name: "Staff $5", kind: "amount", amount: 500 }))).toBe("Staff");
    expect(dealTitle(deal({ name: "15%" }))).toBe("15%");
    expect(dealTitle(deal({ name: "VIP 20%", percent: 15 }))).toBe("VIP 20%");
  });
});

describe("deals added up", () => {
  it("the quote's joined name is each deal it took off", () => {
    expect(appliedDealNames("20% above $150 + Students 15% + 10% above $50")).toEqual(["20% above $150", "Students 15%", "10% above $50"]);
    expect(appliedDealNames("Students 15%")).toEqual(["Students 15%"]);
    expect(appliedDealNames("")).toEqual([]);
  });
  it("counts each applied deal once, picked or not", () => {
    expect(rewardsActiveCount("", ["s"], "20% above $150 + Students 15%", ["Students 15%"])).toBe(2);
    expect(rewardsActiveCount("K7M4QH", ["s"], "20% above $150 + Students 15% + 10% above $50", ["Students 15%"])).toBe(4);
  });
});
