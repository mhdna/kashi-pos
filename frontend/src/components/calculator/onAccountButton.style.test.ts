// The calculator's On Account button is switched off for now (commented out in
// the template); the Coupon card beside it stays.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const src = readFileSync(resolve(__dirname, "../Calculator.vue"), "utf8");
const live = src.replace(/<!--[\s\S]*?-->/g, "");

describe("calculator On Account button", () => {
  it("is not in the rendered template", () => {
    expect(live).not.toMatch(/>On Account</);
    expect(live).not.toMatch(/emit\('account'\)/);
  });
  it("is kept in a comment so it can come back", () => {
    expect(src).toMatch(/<!--[^]*On Account[^]*-->/);
  });
  it("leaves the Coupon button", () => {
    expect(live).toMatch(/>Coupon</);
  });
});
