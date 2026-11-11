import { describe, expect, it } from "vitest";
import { currencyIcon } from "./currencyIcon";
import { iconPathFor } from "./icons";

describe("currencyIcon", () => {
  it.each([
    ["USD", "mdi-currency-usd"],
    ["usd", "mdi-currency-usd"],
    [" EUR ", "mdi-currency-eur"],
    ["GBP", "mdi-currency-gbp"],
    ["LBP", "mdi-cash"],
    ["", "mdi-cash"],
    [undefined, "mdi-cash"],
    [null, "mdi-cash"],
  ])("%s → %s", (code, icon) => {
    expect(currencyIcon(code)).toBe(icon);
  });

  it("only returns registered icons", () => {
    for (const code of ["USD", "EUR", "GBP", "LBP"]) expect(iconPathFor(currencyIcon(code))).toMatch(/^M/);
  });
});
