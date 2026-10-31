import { describe, expect, it } from "vitest";
import { nextSort, sortItems, visibleRange } from "./virtual";

describe("visibleRange", () => {
  it("renders the top rows plus overscan at scroll 0", () => {
    expect(visibleRange(0, 520, 52, 1000, 4)).toEqual({ start: 0, end: 14, padTop: 0, padBottom: (1000 - 14) * 52 });
  });
  it("moves the window and keeps the total height constant", () => {
    const r = visibleRange(5200, 520, 52, 1000, 4);
    expect(r.start).toBe(96);
    expect(r.end).toBe(114);
    expect(r.padTop + (r.end - r.start) * 52 + r.padBottom).toBe(1000 * 52);
  });
  it("never goes past the end", () => {
    const r = visibleRange(99999, 520, 52, 100, 4);
    expect(r.end).toBe(100);
    expect(r.padBottom).toBe(0);
  });
  it("renders nothing for an empty list and a screenful before measuring", () => {
    expect(visibleRange(0, 520, 52, 0)).toEqual({ start: 0, end: 0, padTop: 0, padBottom: 0 });
    expect(visibleRange(0, 0, 10, 500, 0).end).toBe(30);
  });
});

describe("sorting", () => {
  it("cycles asc, desc, off", () => {
    const a = nextSort(null, "x");
    expect(a).toEqual({ key: "x", direction: "asc" });
    const b = nextSort(a, "x");
    expect(b).toEqual({ key: "x", direction: "desc" });
    expect(nextSort(b, "x")).toBeNull();
    expect(nextSort(b, "y")).toEqual({ key: "y", direction: "asc" });
  });
  it("sorts numbers and strings, empties last", () => {
    const rows = [{ n: 10 }, { n: 2 }, { n: undefined }, { n: 5 }];
    expect(sortItems(rows, { key: "n", direction: "asc" }).map((r) => r.n)).toEqual([2, 5, 10, undefined]);
    expect(sortItems(rows, { key: "n", direction: "desc" }).map((r) => r.n)).toEqual([10, 5, 2, undefined]);
    const names = [{ s: "b2" }, { s: "b10" }, { s: "A" }];
    expect(sortItems(names, { key: "s", direction: "asc" }).map((r) => r.s)).toEqual(["A", "b2", "b10"]);
  });
  it("keeps order when unsorted", () => {
    const rows = [{ n: 2 }, { n: 1 }];
    expect(sortItems(rows, null)).toBe(rows);
  });
});
