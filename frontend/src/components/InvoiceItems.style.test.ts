import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const items = readFileSync(resolve(__dirname, "InvoiceItems.vue"), "utf8");
const status = readFileSync(resolve(__dirname, "calculator/CalculatorStatus.vue"), "utf8");

describe("discount colours", () => {
  it("a line's -30% chip is the same red as the Discount total: the theme's error colour", () => {
    const chips = items.match(/:color="item\.discountOff \? 'grey' : '(\w+)'"/g) ?? [];
    expect(chips).toHaveLength(2);
    for (const chip of chips) expect(chip).toContain("'error'");
    // Red text only: no tinted background behind it.
    expect(items.match(/:variant="item\.discountOff \? 'outlined' : 'text'"/g) ?? []).toHaveLength(2);
    expect(items).not.toMatch(/item\.discountOff \? 'outlined' : 'tonal'/);
    expect(status).toMatch(/\.calculator-amounts-discount \{[^}]*color: rgb\(var\(--k-theme-error\)\)/);
  });
});
