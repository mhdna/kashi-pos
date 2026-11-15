// Cards, dialogs and menus are square across the app, whatever rounded= a
// page asks for.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const css = readFileSync(resolve(__dirname, "ui.css"), "utf8");
const card = readFileSync(resolve(__dirname, "KCard.vue"), "utf8");

describe("square corners", () => {
  it("cards are square, even with rounded= or a rounded-* class", () => {
    expect(css).toMatch(/\.k-card \{[^}]*border-radius: 0 !important;/);
    expect(card).not.toMatch(/roundedStyle/);
  });
  it("dialogs and menus are square", () => {
    expect(css).toMatch(/\.k-dialog > \.k-overlay__content \{[^}]*border-radius: 0;/);
    expect(css).toMatch(/\.k-menu > \.k-overlay__content \{[^}]*border-radius: 0;/);
  });
  it("product search tiles are square", () => {
    const src = readFileSync(resolve(__dirname, "../components/ProductSearchResults.vue"), "utf8");
    expect(src).toMatch(/\.product-cell \{[^}]*border-radius: 0;/);
  });
});
