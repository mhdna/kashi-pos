import { describe, expect, it } from "vitest";
import { liveOverrideFor, liveOverrideTitle, type LiveCartLineOverride } from "./liveCart";

const overrides: LiveCartLineOverride[] = [
  { barcode: "bc-A", price_cents: 1200 },
  { barcode: "UNKNOWN", line_ref: "u1", discount: 10 },
  { barcode: "UNKNOWN", line_ref: "u2", unknown_deal: "none" },
  // Names no line: no ref on an unknown barcode, a ref on a product's.
  { barcode: "UNKNOWN", price_cents: 100 },
  { barcode: "bc-B", line_ref: "u1", price_cents: 100 },
];

describe("liveOverrideFor", () => {
  it("finds a product's override by its barcode", () => {
    expect(liveOverrideFor({ barcode: "bc-A", qty: 1 }, overrides, false)?.price_cents).toBe(1200);
  });

  it("finds each unknown item's own override by its ref", () => {
    expect(liveOverrideFor({ barcode: "UNKNOWN", qty: 1, unknownRef: "u1" }, overrides, true)?.discount).toBe(10);
    expect(liveOverrideFor({ barcode: "UNKNOWN", qty: 1, unknownRef: "u2" }, overrides, true)?.unknown_deal).toBe("none");
    expect(liveOverrideFor({ barcode: "UNKNOWN", qty: 1, unknownRef: "u3" }, overrides, true)).toBeUndefined();
  });

  it("never gives an unknown item a barcode-only override, nor a product a ref's", () => {
    expect(liveOverrideFor({ barcode: "UNKNOWN", qty: 1 }, overrides, true)).toBeUndefined();
    expect(liveOverrideFor({ barcode: "bc-B", qty: 1 }, overrides, false)).toBeUndefined();
  });

  it("skips returned, locked and barcode-less lines", () => {
    expect(liveOverrideFor({ barcode: "bc-A", qty: -1 }, overrides, false)).toBeUndefined();
    expect(liveOverrideFor({ barcode: "bc-A", qty: 1, locked: true }, overrides, false)).toBeUndefined();
    expect(liveOverrideFor({ barcode: "", qty: 1 }, overrides, false)).toBeUndefined();
  });
});

describe("liveOverrideTitle", () => {
  it("names everything central set", () => {
    expect(liveOverrideTitle({ priceCents: 1250, discount: 10, noDeal: true })).toBe(
      "Set in central: price $12.50, 10% off, no deal",
    );
    expect(liveOverrideTitle({ unknownDeal: "b2g1_30" })).toBe("Set in central: deal, else 30%");
    expect(liveOverrideTitle({ unknownDeal: "none" })).toBe("Set in central: no deal");
  });
});
