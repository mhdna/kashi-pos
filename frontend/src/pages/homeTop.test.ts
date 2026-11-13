// Vitest doesn't mount the whole sale page for this: the top section's tint is
// a wrapper in the template and a rule in the style, read from the source.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const source = readFileSync(resolve(process.cwd(), "src/pages/home.vue"), "utf8");

describe("sale page top section", () => {
  it("has a tint behind it", () => {
    expect(source).toMatch(/\.home-left-pane \{[^}]*--top-tint: rgba\(var\(--k-theme-on-surface\), 0\.0\d+\)/);
    expect(source).toMatch(/\.home-top \{[^}]*background: var\(--top-tint\)/);
  });

  it("the cart's column header takes the same tint, with no gap between it and the top section", () => {
    const items = readFileSync(resolve(process.cwd(), "src/components/InvoiceItems.vue"), "utf8");
    expect(items).toMatch(/\.invoice-items-header \{[^}]*linear-gradient\(var\(--top-tint, transparent\)/);
    // The cart's own top margin would show the page colour between the two.
    expect(items).not.toMatch(/class="invoice-items mt-\d/);
  });

  it("the tinted wrapper holds the header and the barcode row, and stops before the cart", () => {
    const start = source.indexOf('<div class="home-top">');
    const cart = source.indexOf("<InvoiceItems", start);
    expect(start).toBeGreaterThan(0);
    const inside = source.slice(start, cart);
    // The title row with the assignees and buttons, and the barcode field.
    expect(inside).toContain("required-assignees");
    expect(inside).toContain("barcodeInput");
    // The cart itself is not inside it, and the divs balance.
    expect(inside).not.toContain("<InvoiceItems");
    const opens = (inside.match(/<div\b/g) ?? []).length;
    const closes = (inside.match(/<\/div>/g) ?? []).length;
    expect(closes).toBe(opens);
  });
});
