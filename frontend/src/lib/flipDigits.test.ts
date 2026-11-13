import { describe, expect, it } from "vitest";
import { changedRun } from "./flipDigits";

describe("changedRun", () => {
  it("is nothing when the number did not change", () => {
    expect(changedRun(477, 477)).toBeNull();
  });

  it("is just the last digit for a plain +1 or -1", () => {
    expect(changedRun(477, 478)).toEqual({ from: 2, count: 1 });
    expect(changedRun(478, 477)).toEqual({ from: 2, count: 1 });
    expect(changedRun(0, 1)).toEqual({ from: 0, count: 1 });
  });

  it("covers every digit a carry or a borrow touches", () => {
    expect(changedRun(479, 480)).toEqual({ from: 1, count: 2 });
    expect(changedRun(480, 479)).toEqual({ from: 1, count: 2 });
    expect(changedRun(499, 500)).toEqual({ from: 0, count: 3 });
    expect(changedRun(1099, 1100)).toEqual({ from: 1, count: 3 });
  });

  it("counts a new card when the number grows, and the cards left when it shrinks", () => {
    expect(changedRun(9, 10)).toEqual({ from: 0, count: 2 });
    expect(changedRun(99, 100)).toEqual({ from: 0, count: 3 });
    expect(changedRun(100, 99)).toEqual({ from: 0, count: 2 });
    expect(changedRun(10, 9)).toEqual({ from: 0, count: 1 });
  });

  it("every step of a long count is one run ending at the last digit, and matches a plain compare", () => {
    for (let n = 0; n < 1300; n++) {
      for (const next of [n + 1, Math.max(0, n - 1)]) {
        if (next === n) continue;
        const run = changedRun(n, next)!;
        const b = String(next);
        expect(run.from + run.count).toBe(b.length); // ends at the units digit
        // Every digit before the run is the same in both numbers...
        const a = String(n).padStart(b.length, " ");
        for (let i = 0; i < run.from; i++) expect(a[i]).toBe(b[i]);
        // ...and the first digit of the run really is different.
        expect(a[run.from]).not.toBe(b[run.from]);
      }
    }
  });
});
