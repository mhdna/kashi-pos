// Deal badges (B1G1, 1 free) are text only: no gift icon on the sale screen,
// the return menu or the invoice details, and no middle dot in their text.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

describe("deal badges", () => {
  for (const file of ["InvoiceItems.vue", "ReturnInvoiceMenu.vue", "../pages/invoices-list.vue"]) {
    it(`${file} has no gift icon`, () => {
      const src = readFileSync(resolve(__dirname, file), "utf8");
      expect(src).not.toMatch(/mdi-gift-outline/);
      expect(src).not.toMatch(/FREE ·/);
    });
  }
});
