import { describe, expect, it } from "vitest";
import { perVisitor } from "./perVisitor";

describe("perVisitor", () => {
  it("is the takings divided by the people, rounded", () => {
    expect(perVisitor(288300, 11)).toBe(26209); // 262.09 in cents
    expect(perVisitor(1000, 3)).toBe(333);
    expect(perVisitor(1000, 1)).toBe(1000);
  });

  it("is nothing without visitors", () => {
    for (const v of [0, -1, null, undefined]) expect(perVisitor(5000, v)).toBeNull();
  });

  it("is zero for a day with visitors and no takings", () => {
    expect(perVisitor(0, 40)).toBe(0);
  });

  it("is nothing for a total that is not a number", () => {
    expect(perVisitor(NaN, 5)).toBeNull();
  });
});
