import { describe, expect, it } from "vitest";
import { matchParts } from "./highlight";

describe("matchParts", () => {
  it("marks every match, case-insensitively", () => {
    expect(matchParts("N24-n245", "n24")).toEqual([
      { text: "N24", hit: true },
      { text: "-", hit: false },
      { text: "n24", hit: true },
      { text: "5", hit: false },
    ]);
  });

  it("keeps the text whole without a term or a match", () => {
    expect(matchParts("N24", "")).toEqual([{ text: "N24", hit: false }]);
    expect(matchParts("N24", "  ")).toEqual([{ text: "N24", hit: false }]);
    expect(matchParts("N24", "x")).toEqual([{ text: "N24", hit: false }]);
    expect(matchParts("", "n")).toEqual([]);
  });

  it("matches the term as typed, trimmed", () => {
    expect(matchParts("AB AB", " ab ")).toEqual([
      { text: "AB", hit: true },
      { text: " ", hit: false },
      { text: "AB", hit: true },
    ]);
  });

  it("doesn't overlap matches", () => {
    expect(matchParts("aaa", "aa")).toEqual([
      { text: "aa", hit: true },
      { text: "a", hit: false },
    ]);
  });
});
