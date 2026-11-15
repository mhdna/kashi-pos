// An unknown item's line is highlighted yellow, like a highlighter pen.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const src = readFileSync(resolve(__dirname, "InvoiceItems.vue"), "utf8");

describe("unknown item line", () => {
  it("has a yellow highlight", () => {
    expect(src).toMatch(/\.invoice-item-unknown \{\s*background: rgba\(255, 235, 59, 0\.45\);/);
  });
});
