import { describe, expect, it } from "vitest";
import { collectedTotalCents, netIncomeCents, onAccountTotalCents } from "./shiftIncome";

describe("shift income totals", () => {
  it("the reported case: Cash $243.00 is an income total of $243.00, not $0.00", () => {
    expect(collectedTotalCents([{ total: 24300 }])).toBe(24300);
  });

  it("is zero for nothing, and ignores junk amounts", () => {
    expect(collectedTotalCents([])).toBe(0);
    expect(collectedTotalCents([{ total: NaN }, { total: undefined as unknown as number }])).toBe(0);
  });

  it("keeps sales on account out of what was collected, and counts them on their own", () => {
    const rows = [{ total: 10000 }, { total: 5000 }, { total: 7000, isOnAccount: true }];
    expect(collectedTotalCents(rows)).toBe(15000);
    expect(onAccountTotalCents(rows)).toBe(7000);
  });

  it("net income is collected minus what went out", () => {
    expect(netIncomeCents(24300, 3000)).toBe(21300);
    expect(netIncomeCents(0, 500)).toBe(-500);
  });

  // The identity the screen promises: the total is exactly the sum of its
  // collected lines, and collected + on account is everything.
  it("randomised: total equals its lines", () => {
    let seed = 7;
    const rnd = () => ((seed = (seed * 1103515245 + 12345) & 0x7fffffff) / 0x7fffffff);
    for (let i = 0; i < 500; i++) {
      const rows = Array.from({ length: Math.floor(rnd() * 8) }, () => ({
        total: Math.floor(rnd() * 100000),
        isOnAccount: rnd() < 0.25,
      }));
      const collected = rows.filter((r) => !r.isOnAccount).reduce((s, r) => s + r.total, 0);
      const owed = rows.filter((r) => r.isOnAccount).reduce((s, r) => s + r.total, 0);
      expect(collectedTotalCents(rows)).toBe(collected);
      expect(onAccountTotalCents(rows)).toBe(owed);
      expect(collectedTotalCents(rows) + onAccountTotalCents(rows)).toBe(rows.reduce((s, r) => s + r.total, 0));
    }
  });
});
