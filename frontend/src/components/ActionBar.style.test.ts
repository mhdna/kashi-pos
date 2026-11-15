// Check Out is green text and icon with no border (orange for an exchange),
// and the hold and cancel Confirm is the primary button.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const src = readFileSync(resolve(__dirname, "ActionBar.vue"), "utf8");
const checkout = src.slice(src.indexOf('class="checkout-button'), src.indexOf("</k-btn>", src.indexOf('class="checkout-button')));

describe("action bar buttons", () => {
  it("Check Out is green text and icon with no border", () => {
    expect(checkout).toMatch(/variant="text"/);
    expect(checkout).not.toMatch(/variant="outlined"|variant="flat"|:style=/);
    expect(src).not.toMatch(/border-color: #2e7d32/);
    expect(src).toMatch(/"green-darken-3"/);
  });

  it("Check Out sits on a raised card: surface background and a shadow", () => {
    const css = src
      .slice(src.indexOf(".checkout-button {"), src.indexOf("}", src.indexOf(".checkout-button {")))
      .replace(/\/\*[\s\S]*?\*\//g, "");
    expect(css).toMatch(/background-color: rgb\(var\(--k-theme-surface\)\)/);
    expect(css).toMatch(/box-shadow: 0 2px 4px/);
    expect(css).not.toMatch(/border/);
  });

  it("the hold and cancel Confirm is the primary button", () => {
    const confirm = src.slice(src.indexOf("<k-card-actions>"), src.indexOf("</k-card-actions>"));
    expect(confirm).toMatch(/color="primary"/);
    expect(confirm).not.toMatch(/brown|blue-grey/);
  });
});
