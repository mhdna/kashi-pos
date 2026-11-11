import { describe, expect, it } from "vitest";
import { searchResultsInset } from "./searchInset";

describe("searchResultsInset", () => {
  it("the keyboard at the bottom of the pane: its own height", () => {
    expect(searchResultsInset(900, 820)).toBe(80);
  });
  it("a pane that ends above the screen bottom: only the overlap, not the keyboard's height", () => {
    // The keyboard starts at 920 but the pane ends at 960: only 40px overlap it.
    expect(searchResultsInset(960, 920)).toBe(40);
  });
  it("a keyboard clear of the pane takes nothing off", () => {
    expect(searchResultsInset(800, 820)).toBe(0);
    expect(searchResultsInset(800, 800)).toBe(0);
  });
  it("rounds to whole pixels and survives bad numbers", () => {
    expect(searchResultsInset(900.4, 819.7)).toBe(81);
    expect(searchResultsInset(NaN, 800)).toBe(0);
    expect(searchResultsInset(900, Infinity)).toBe(0);
  });
  it("randomized: the results' bottom edge never ends below the keyboard's top", () => {
    let seed = 7;
    const rnd = () => (seed = (seed * 1664525 + 1013904223) % 4294967296) / 4294967296;
    for (let i = 0; i < 500; i++) {
      const paneBottom = 300 + rnd() * 900;
      const keyboardTop = 200 + rnd() * 1000;
      const inset = searchResultsInset(paneBottom, keyboardTop);
      expect(inset).toBeGreaterThanOrEqual(0);
      // The results end at paneBottom - inset: at or above the keyboard (to rounding).
      if (paneBottom > keyboardTop) expect(paneBottom - inset).toBeLessThanOrEqual(keyboardTop + 0.5);
    }
  });
});
