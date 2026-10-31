import { describe, expect, it } from "vitest";
import { caretIndexAt, replaceRange } from "./caret";

// Every character 10px wide.
const mono = (n: number) => n * 10;

describe("caretIndexAt", () => {
  it("snaps to the nearest character boundary", () => {
    expect(caretIndexAt("N245", 0, mono)).toBe(0);
    expect(caretIndexAt("N245", 4, mono)).toBe(0);
    expect(caretIndexAt("N245", 6, mono)).toBe(1);
    expect(caretIndexAt("N245", 25, mono)).toBe(2);
    expect(caretIndexAt("N245", 34, mono)).toBe(3);
  });

  it("stays inside the text", () => {
    expect(caretIndexAt("N245", -20, mono)).toBe(0);
    expect(caretIndexAt("N245", 500, mono)).toBe(4);
    expect(caretIndexAt("", 30, mono)).toBe(0);
  });

  it("works with uneven widths", () => {
    const widths = [0, 30, 35, 70];
    const uneven = (n: number) => widths[n] ?? 0;
    expect(caretIndexAt("Wil", 14, uneven)).toBe(0);
    expect(caretIndexAt("Wil", 16, uneven)).toBe(1);
    expect(caretIndexAt("Wil", 33, uneven)).toBe(2);
    expect(caretIndexAt("Wil", 60, uneven)).toBe(3);
  });
});

describe("replaceRange", () => {
  it("inserts at the caret", () => {
    expect(replaceRange("N25", 2, 2, "4")).toEqual({ value: "N245", caret: 3 });
  });

  it("replaces a selection, whichever way round", () => {
    expect(replaceRange("N245", 1, 3, "x")).toEqual({ value: "Nx5", caret: 2 });
    expect(replaceRange("N245", 3, 1, "")).toEqual({ value: "N5", caret: 1 });
  });

  it("clamps to the value", () => {
    expect(replaceRange("N2", 5, 9, "4")).toEqual({ value: "N24", caret: 3 });
  });
});
