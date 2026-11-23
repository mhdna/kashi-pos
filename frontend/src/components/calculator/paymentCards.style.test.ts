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

// Every card is the same height, whatever is beside it: a card that comes or
// goes never resizes the others.
describe("calculator cards keep one height", () => {
  it("payment method cards are fixed at 62px", () => {
    const src = readFileSync(resolve(__dirname, "CalculatorPaymentMethods.vue"), "utf8");
    expect(src).toMatch(/\.calculator-type-card \{[^}]*height: 62px;[^}]*min-height: 62px;[^}]*max-height: 62px;/);
  });
  it("On Account and Coupon are the same height; On Account is two lines", () => {
    const src = readFileSync(resolve(__dirname, "../Calculator.vue"), "utf8");
    expect(src).toMatch(/\.calculator-action-cards \{[^}]*grid-auto-rows: 62px;/);
    expect(src).toMatch(/\.calculator-action-cards > \.calculator-type-card \{[^}]*height: 62px;[^}]*max-height: 62px;/);
    expect(src).toMatch(/\.calculator-action-cards \.payment-method-name \{[^}]*white-space: normal;/);
    expect(src).toContain('<br>{{ t("Account") }}</span>');
  });
});
