import { describe, expect, it } from "vitest";
import { firstRuleError, normaliseMessages } from "./fieldRules";

describe("firstRuleError", () => {
  it("returns the first failing message", () => {
    const rules = [(v: unknown) => !!v || "Required", (v: unknown) => String(v).length > 2 || "Too short"];
    expect(firstRuleError(rules, "")).toBe("Required");
    expect(firstRuleError(rules, "ab")).toBe("Too short");
    expect(firstRuleError(rules, "abc")).toBe("");
  });
  it("treats a bare false as a failure with no text", () => {
    expect(firstRuleError([() => false], 1)).toBe(" ");
  });
});

describe("normaliseMessages", () => {
  it("wraps strings and drops empties", () => {
    expect(normaliseMessages("a")).toEqual(["a"]);
    expect(normaliseMessages(["a", ""])).toEqual(["a"]);
    expect(normaliseMessages(undefined)).toEqual([]);
  });
});
