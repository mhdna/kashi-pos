import { describe, expect, it } from "vitest";
import { lineSalespersonChanges } from "./lineSalespersons";

const lines = [
  { lineId: 10, salespersonId: 0 },
  { lineId: 11, salespersonId: 3 },
  { lineId: 12, salespersonId: 5 },
];

describe("lineSalespersonChanges", () => {
  it("lists nothing when nothing was touched", () => {
    expect(lineSalespersonChanges(lines, {})).toEqual([]);
  });

  it("lists only the lines that really change, in invoice order", () => {
    expect(lineSalespersonChanges(lines, { 12: 3, 10: 5 })).toEqual([
      { lineId: 10, salespersonId: 5 },
      { lineId: 12, salespersonId: 3 },
    ]);
  });

  it("ignores a pick that puts the same person back", () => {
    expect(lineSalespersonChanges(lines, { 11: 3, 10: 0 })).toEqual([]);
  });

  it("treats 0 as handing the line back to the invoice", () => {
    expect(lineSalespersonChanges(lines, { 11: 0 })).toEqual([{ lineId: 11, salespersonId: 0 }]);
  });

  it("ignores picks for lines that are not on the invoice", () => {
    expect(lineSalespersonChanges(lines, { 99: 3 })).toEqual([]);
  });

  it("every combination of picks over a few lines agrees with a plain compare", () => {
    const ids = [0, 3, 5];
    for (const a of ids) for (const b of ids) for (const c of ids) {
      const picks = { 10: a, 11: b, 12: c };
      const expected = lines
        .map((l, i) => ({ lineId: l.lineId, salespersonId: [a, b, c][i]! }))
        .filter((x, i) => x.salespersonId !== lines[i]!.salespersonId);
      expect(lineSalespersonChanges(lines, picks)).toEqual(expected);
    }
  });
});
