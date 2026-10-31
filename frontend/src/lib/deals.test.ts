import { describe, expect, it } from "vitest";
import { dealLineKey, dealsByLine, type DealLine } from "./deals";

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
