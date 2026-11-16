// Expenses and Loans: no icon before the page title, chips are text only
// (no tinted background), and the dialog's buttons are solid, not tinted.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

for (const page of ["expenses", "loans"]) {
  const src = readFileSync(resolve(__dirname, `${page}.vue`), "utf8");
  const template = src.slice(0, src.indexOf("<script"));
  describe(`${page} page`, () => {
    it("has no icon before its title", () => {
      const title = template.slice(template.indexOf('<div class="records-title">'), template.indexOf("</h1>"));
      expect(title).not.toContain("k-icon");
    });
    it("has no chip or button with a tinted background", () => {
      expect(template).not.toContain('variant="tonal"');
      for (const chip of template.match(/<k-chip[^>]*>/g) ?? []) expect(chip).toContain('variant="text"');
    });
  });
}
