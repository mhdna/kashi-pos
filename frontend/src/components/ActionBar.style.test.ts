// Check Out keeps its original green outline (orange for an exchange), and the
// hold and cancel Confirm is the primary button.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const src = readFileSync(resolve(__dirname, "ActionBar.vue"), "utf8");
const checkout = src.slice(src.indexOf('class="checkout-button'), src.indexOf("</k-btn>", src.indexOf('class="checkout-button')));

describe("action bar buttons", () => {
  it("Check Out keeps its original look: green outline on a see-through background", () => {
    expect(checkout).toMatch(/variant="outlined"/);
    expect(src).toMatch(/border-color: #2e7d32 !important; border-width: 2px/);
    expect(src).toMatch(/"green-darken-3"/);
  });

  it("the hold and cancel Confirm is the primary button", () => {
    const confirm = src.slice(src.indexOf("<k-card-actions>"), src.indexOf("</k-card-actions>"));
    expect(confirm).toMatch(/color="primary"/);
    expect(confirm).not.toMatch(/brown|blue-grey/);
  });
});

describe("Check Out corners", () => {
  it("are square", () => {
    const src = readFileSync(resolve(__dirname, "ActionBar.vue"), "utf8");
    const block = src.slice(src.indexOf('class="checkout-button'), src.indexOf('@click="$emit(\'checkout\')"'));
    expect(block).toContain('rounded="0"');
  });
});
