// The calculator's payment, On Account and Coupon cards have square corners.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

describe("calculator payment cards", () => {
  for (const file of ["CalculatorPaymentMethods.vue", "../Calculator.vue"]) {
    it(`${file} cards are square`, () => {
      const src = readFileSync(resolve(__dirname, file), "utf8");
      expect(src).toMatch(/\.calculator-type-card \{[^}]*border-radius: 0;/);
    });
  }
});
