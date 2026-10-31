import { describe, expect, it } from "vitest";
import { placeBox } from "./position";

const viewport = { width: 1000, height: 800 };
const anchor = { left: 100, right: 200, top: 100, bottom: 140 };

describe("placeBox", () => {
  it("opens below, aligned to the start edge", () => {
    expect(placeBox(anchor, 150, 200, viewport, "bottom start")).toEqual({ left: 100, top: 140 });
  });
  it("aligns to the end edge", () => {
    expect(placeBox(anchor, 150, 200, viewport, "bottom end")).toEqual({ left: 50, top: 140 });
  });
  it("centres under the anchor with a gap", () => {
    expect(placeBox(anchor, 100, 100, viewport, "bottom center", 6)).toEqual({ left: 100, top: 146 });
  });
  it("flips above when there is no room below", () => {
    const low = { left: 100, right: 200, top: 700, bottom: 740 };
    expect(placeBox(low, 100, 200, viewport, "bottom start").top).toBe(500);
  });
  it("stays inside the viewport", () => {
    const edge = { left: 950, right: 990, top: 100, bottom: 140 };
    expect(placeBox(edge, 200, 100, viewport, "bottom start").left).toBe(792);
  });
  it("mirrors start/end in right-to-left", () => {
    expect(placeBox(anchor, 150, 100, viewport, "bottom start", 0, true).left).toBe(50);
  });
});
