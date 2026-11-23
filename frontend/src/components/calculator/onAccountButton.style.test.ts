// The calculator's On Account button is shown again, next to Coupon, and opens
// the client picker (emit 'account') once the cart has items.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const src = readFileSync(resolve(__dirname, "../Calculator.vue"), "utf8");
const live = src.replace(/<!--[\s\S]*?-->/g, "");

describe("calculator On Account button", () => {
  it("is in the rendered template and asks for a client", () => {
    expect(live).toMatch(/t\("On"\) \}\}<br>\{\{ t\("Account"\) \}\}/);
    expect(live).toMatch(/emit\('account'\)/);
  });
  it("is not left in a comment", () => {
    expect(src).not.toMatch(/On Account button, off for now/);
  });
  it("keeps the Coupon button", () => {
    expect(live).toMatch(/t\("Coupon"\) \}\}<\/span>/);
  });
});
