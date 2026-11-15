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

describe("nothing is rounded", () => {
  it("rounded= gives square corners; only circle stays round", async () => {
    const { roundedStyle } = await import("./util");
    for (const value of [true, "", "sm", "lg", "xl", "pill", "4px"]) expect(roundedStyle(value as any)).toBe("0");
    expect(roundedStyle("circle")).toBe("50%");
  });
  it("the kit's buttons, fields, chips, alerts and rounded utilities are square", () => {
    const utilities = readFileSync(resolve(__dirname, "utilities.css"), "utf8");
    for (const sel of [".k-btn", ".k-chip", ".k-alert", ".k-field", ".k-snackbar__wrapper"]) {
      const rule = css.match(new RegExp(`^\\s*${sel.replace(/[.]/g, "\\.")} \\{[^}]*\\}`, "m"))![0];
      expect(rule).toMatch(/border-radius: 0;/);
    }
    expect(utilities).toMatch(/\.rounded-lg\{border-radius:0\}/);
    expect(utilities).toMatch(/\.rounded-pill\{border-radius:0\}/);
  });
  it("app styles have no pixel radii left (circles use 50%)", async () => {
    const { globSync } = (await import("node:fs")) as any;
    const files: string[] = globSync("src/{components,pages}/**/*.vue", { cwd: process.cwd() });
    expect(files.length).toBeGreaterThan(50);
    // The visitor counter in the nav keeps its rounded look on purpose.
    const keptRound = /FlipNumber\.vue$/;
    for (const f of files.filter((name) => !keptRound.test(name))) {
      const src = readFileSync(resolve(process.cwd(), f), "utf8").replace(/\.visitor-counter(-btn)? \{[^}]*\}/g, "");
      expect(src, f).not.toMatch(/border-radius: *(?!0[;\s!])\d+(\.\d+)?(px|rem|em)/);
    }
  });
});

describe("visitor counter", () => {
  it("keeps its rounded corners", () => {
    const nav = readFileSync(resolve(__dirname, "../components/NavDrawer.vue"), "utf8");
    const flip = readFileSync(resolve(__dirname, "../components/FlipNumber.vue"), "utf8");
    expect(nav).toMatch(/\.visitor-counter \{[^}]*border-radius: 12px;/);
    expect(nav).toMatch(/\.visitor-counter-btn \{[^}]*border-radius: 8px !important;/);
    expect(flip).toMatch(/border-radius: 8px;/);
  });
});
