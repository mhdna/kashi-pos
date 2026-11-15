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

describe("salesperson picker corners", () => {
  it("tiles and the menu card are square", () => {
    const grid = readFileSync(resolve(__dirname, "SalespersonGrid.vue"), "utf8");
    const dialog = readFileSync(resolve(__dirname, "SalespersonsDialog.vue"), "utf8");
    expect(grid).toMatch(/\.sp-tile \{[^}]*border-radius: 0;/);
    expect(dialog).toContain('<k-card class="salespersons-grid" rounded="0">');
  });
});
