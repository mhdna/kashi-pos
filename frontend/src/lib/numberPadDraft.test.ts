import { describe, expect, it } from "vitest";
import { appendDecimalToDraft, appendDigitToDraft, draftValue, toggleSign } from "./numberPadDraft";

const intOpts = { maxLength: 12, max: 999999999999, allowDecimal: false, decimalPlaces: 2 };
const decOpts = { maxLength: 12, max: 999999999.99, allowDecimal: true, decimalPlaces: 2 };

// Types keys into a fresh pad (replace mode on, as when it opens) and
// returns the draft.
function type(start: string, keys: string, opts = intOpts) {
  let draft = start;
  let replace = true;
  for (const key of keys) {
    let next: string | null;
    if (key === "±") next = toggleSign(draft);
    else if (key === ".") next = appendDecimalToDraft(draft, replace);
    else next = appendDigitToDraft(draft, key, { ...opts, replace });
    if (next === null) continue;
    draft = next;
    replace = false;
  }
  return draft;
}
const value = (draft: string, opts = intOpts) =>
  draftValue(draft, { min: -opts.max, max: opts.max, allowDecimal: opts.allowDecimal, decimalPlaces: opts.decimalPlaces });

describe("number pad sign", () => {
  it("typing then ± makes the number negative, like a calculator", () => {
    expect(type("0", "686000±")).toBe("-686000");
    expect(value(type("0", "686000±"))).toBe(-686000);
  });

  it("± first starts a negative number; the digits keep the sign", () => {
    expect(type("0", "±686000")).toBe("-686000");
    expect(type("", "±5")).toBe("-5");
  });

  it("± twice is back to positive", () => {
    expect(type("0", "25±±")).toBe("25");
    expect(value(type("0", "25±±"))).toBe(25);
  });

  it("± on the value the pad opened with flips it (an autofilled -50000)", () => {
    expect(type("-50000", "±")).toBe("50000");
    expect(type("50000", "±")).toBe("-50000");
  });

  it("typing over an opened negative value starts a new positive number", () => {
    expect(type("-50000", "3")).toBe("3");
  });

  it("a lone minus is 0, never -0", () => {
    expect(type("0", "±")).toBe("-");
    expect(Object.is(value("-"), 0)).toBe(true);
    expect(Object.is(value("-0"), 0)).toBe(true);
  });

  it("decimals work with the sign", () => {
    expect(type("0", "±.5", decOpts)).toBe("-0.5");
    expect(type("0", "12.50±", decOpts)).toBe("-12.50");
    expect(value(type("0", "12.505±", decOpts), decOpts)).toBe(-12.5); // third decimal refused
    expect(value(type("0", "±12.34", decOpts), decOpts)).toBe(-12.34);
  });

  it("the sign does not count toward the length or the max", () => {
    const opts = { ...intOpts, maxLength: 3, max: 999 };
    expect(type("0", "±9999", opts)).toBe("-999");
    expect(value("-5000", opts)).toBe(-999);
  });

  it("without negatives the clamp still floors at min", () => {
    expect(draftValue("-25", { min: 0, max: 999, allowDecimal: false, decimalPlaces: 2 })).toBe(0);
  });

  it("random key runs: the value is the typed digits with the typed sign", () => {
    let seed = 20261116;
    const rand = (n: number) => {
      seed = (seed * 1103515245 + 12345) % 2 ** 31;
      return seed % n;
    };
    for (let run = 0; run < 500; run++) {
      const keys = Array.from({ length: 1 + rand(10) }, () => (rand(5) === 0 ? "±" : String(rand(10)))).join("");
      const draft = type("0", keys);
      const signs = [...keys].filter((k) => k === "±").length;
      const digits = [...keys].filter((k) => k !== "±").join("").replace(/^0+(?=\d)/, "");
      const magnitude = Number(digits || 0);
      const expected = signs % 2 === 1 ? -magnitude : magnitude;
      expect(value(draft), `keys ${keys}`).toBe(expected === 0 ? 0 : expected);
    }
  });
});
