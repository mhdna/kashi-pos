// Vitest doesn't mount the whole sale page for this: the rules are read from
// the source.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const source = readFileSync(resolve(process.cwd(), "src/pages/home.vue"), "utf8");

describe("sale page barcode field", () => {
  it("a scan that worked flashes the barcode field green, one that did not red; never yellow", () => {
    const success = source.match(/\.barcode-flash-success \.k-field \{[^}]*background-color: rgba\((\d+), (\d+), (\d+)/);
    const error = source.match(/\.barcode-flash-error \.k-field \{[^}]*background-color: rgba\((\d+), (\d+), (\d+)/);
    expect(success).toBeTruthy();
    expect(error).toBeTruthy();
    const [, r, g, b] = success!.map(Number) as [number, number, number, number];
    expect(g).toBeGreaterThan(r); // green channel leads: not the old yellow (255, 214, 0)
    expect(g).toBeGreaterThan(b);
    const [, er, eg] = error!.map(Number) as [number, number, number];
    expect(er).toBeGreaterThan(eg); // red stays red
  });

  it("the top of the page has no tinted background behind it", () => {
    const items = readFileSync(resolve(process.cwd(), "src/components/InvoiceItems.vue"), "utf8");
    expect(source).not.toMatch(/--top-tint|\.home-top/);
    expect(items).not.toMatch(/--top-tint/);
    // And the cart keeps its own spacing above it.
    expect(items).toMatch(/class="invoice-items mt-2 mb-2 me-0"/);
  });
});
