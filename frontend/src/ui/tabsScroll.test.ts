import { describe, expect, it } from "vitest";
import { arrowState, pageTarget, revealTarget, wheelToX } from "./tabsScroll";

describe("arrowState", () => {
  it("the reported case: Settings tabs wider than the screen show the next arrow at the start", () => {
    expect(arrowState(0, 1760, 2300)).toEqual({ prev: false, next: true });
    expect(arrowState(540, 1760, 2300)).toEqual({ prev: true, next: false });
    expect(arrowState(200, 1760, 2300)).toEqual({ prev: true, next: true });
  });
  it("tabs that fit show no arrows", () => {
    expect(arrowState(0, 800, 800)).toEqual({ prev: false, next: false });
    expect(arrowState(0, 800, 600)).toEqual({ prev: false, next: false });
    expect(arrowState(0.5, 800, 800.6)).toEqual({ prev: false, next: false });
  });
});

describe("pageTarget", () => {
  it("moves most of a width and stops at the ends", () => {
    expect(pageTarget(0, 1000, 3000, 1)).toBe(800);
    expect(pageTarget(1900, 1000, 3000, 1)).toBe(2000);
    expect(pageTarget(300, 1000, 3000, -1)).toBe(0);
    expect(pageTarget(0, 50, 3000, 1)).toBe(80);
  });
  it("randomized: always inside [0, max]", () => {
    for (let n = 0; n < 500; n++) {
      const cw = 50 + ((n * 37) % 1500), sw = cw + ((n * 91) % 2000), sl = (n * 53) % (sw - cw + 1);
      for (const dir of [-1, 1] as const) {
        const t = pageTarget(sl, cw, sw, dir);
        expect(t).toBeGreaterThanOrEqual(0);
        expect(t).toBeLessThanOrEqual(sw - cw);
      }
    }
  });
});

describe("wheelToX", () => {
  it("uses the bigger of the two", () => {
    expect(wheelToX(0, 120)).toBe(120);
    expect(wheelToX(-40, 10)).toBe(-40);
    expect(wheelToX(0, -100)).toBe(-100);
  });
});

describe("revealTarget", () => {
  it("scrolls just enough to show a hidden tab, with a margin", () => {
    expect(revealTarget(0, 1000, 3000, 1500, 200)).toBe(740);
    expect(revealTarget(800, 1000, 3000, 100, 200)).toBe(60);
    expect(revealTarget(500, 1000, 3000, 700, 200)).toBe(500);
    expect(revealTarget(0, 1000, 3000, 2900, 100)).toBe(2000);
  });
});
