// Total, Rounded and the discount on the calculator card are coloured text
// with no pill (background or rounded box) behind them.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const css = readFileSync(resolve(__dirname, "CalculatorStatus.vue"), "utf8");
const rule = (sel: string) => css.slice(css.indexOf(`${sel} {`), css.indexOf("}", css.indexOf(`${sel} {`)));

describe("calculator card amounts", () => {
  for (const sel of [".calculator-amounts-total", ".calculator-amounts-rounded", ".calculator-amounts-discount"]) {
    it(`${sel} has no pill behind it`, () => {
      expect(rule(sel)).not.toMatch(/background|border-radius/);
    });
  }

  it("the discount keeps its red text", () => {
    expect(rule(".calculator-amounts-discount")).toMatch(/color: rgb\(var\(--k-theme-error\)\)/);
  });
});
