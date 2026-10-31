import { describe, expect, it } from "vitest";
import { dealLineKey, dealsByLine, unknownDealsByLine, type DealLine } from "./deals";

describe("dealsByLine", () => {
  // Four $15 jeans on "Buy 2 get 1, otherwise 30%", one sold as Summer and
  // three as Winter: two cart lines, each with its own share of the deal.
  const lines: DealLine[] = [
    { code: "CA23KGJE15", barcode: "bc", saleOption: "Summer", qty: 1, deal: "B2G1", dealDiscountCents: 1500, free: true },
    { code: "CA23KGJE15", barcode: "bc", saleOption: "Winter", qty: 2, deal: "B2G1", dealDiscountCents: 0 },
    { code: "CA23KGJE15", barcode: "bc", saleOption: "Winter", qty: 1, deal: "B2G1", dealDiscountCents: 450 },
  ];

  it("keeps a product's sale options apart", () => {
    const deals = dealsByLine(lines);
    const summer = deals.get(dealLineKey({ code: "CA23KGJE15", barcode: "bc", saleOption: "Summer" }));
    const winter = deals.get(dealLineKey({ code: "CA23KGJE15", barcode: "bc", saleOption: "Winter" }));
    expect(summer).toMatchObject({ discountCents: 1500, freeQty: 1 });
    expect(winter).toMatchObject({ discountCents: 450, freeQty: 0 });
  });

  it("treats no option the same on both sides", () => {
    const deals = dealsByLine([{ code: "A", barcode: "bc-a", qty: 3, deal: "B2G1", dealDiscountCents: 100 }]);
    expect(deals.get(dealLineKey({ code: "A", barcode: "bc-a", saleOption: "" }))?.discountCents).toBe(100);
  });
});

describe("unknown item lines", () => {
  // Every unknown item is the same product: its ref, not its barcode, is
  // what ties a quote line to its cart line.
  it("keys unknown item lines by their ref", () => {
    const lines: DealLine[] = [
      { code: "UNKNOWN", barcode: "UNKNOWN", lineRef: "a", qty: 1, deal: "Buy 2 get 1", dealLabel: "B2G1", dealDiscountCents: 3000, free: true, unknownDeal: "b2g1_30" },
      { code: "UNKNOWN", barcode: "UNKNOWN", lineRef: "b", qty: 1, unknownDeal: "none" },
    ];
    const deals = dealsByLine(lines);
    expect(deals.get(dealLineKey({ barcode: "UNKNOWN", unknownRef: "a" }))?.freeQty).toBe(1);
    expect(deals.get(dealLineKey({ barcode: "UNKNOWN", unknownRef: "b" }))).toBeUndefined();
    const choices = unknownDealsByLine(lines);
    expect(choices.get(dealLineKey({ barcode: "UNKNOWN", unknownRef: "a" }))).toBe("b2g1_30");
    expect(choices.get(dealLineKey({ barcode: "UNKNOWN", unknownRef: "b" }))).toBe("none");
  });
});
