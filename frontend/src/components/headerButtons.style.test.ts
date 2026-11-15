// The sale screen's Salesperson and Client buttons are square grey tonal
// buttons: no colour of their own, no rounded corners.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

for (const file of ["SalespersonsDialog.vue", "ClientButton.vue"]) {
  const src = readFileSync(resolve(__dirname, file), "utf8");
  const btn = src.slice(src.indexOf("<k-btn"), src.indexOf('class="header-fixed-width-btn"'));
  describe(file, () => {
    it("is a square tonal button with no colour", () => {
      expect(btn).toMatch(/variant="tonal"/);
      expect(btn).toMatch(/rounded="0"/);
      expect(btn).not.toMatch(/\scolor="/);
    });
  });
}
