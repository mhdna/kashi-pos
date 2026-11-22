import { describe, expect, it } from "vitest";
import { dealLineKey, replacedDiscountsByLine, type DealLine } from "./deals";
// Written by the backend's TestReplacedDiscountQuoteGolden: a $100 jacket
// and $20 socks at 20% with "Independence Day $50" (instead of item
// discounts) picked, and two socks ($40) that don't reach its $100.
import golden from "./testdata/replaced_discount_quotes.json";

const line = (over: Partial<DealLine>): DealLine => ({ code: "J", barcode: "bc-J", qty: 1, ...over });

describe("replacedDiscountsByLine", () => {
  it("marks every line of the backend's sale that went without its 20%", () => {
    const got = replacedDiscountsByLine(golden.replaced.lines as DealLine[]);
    expect(got.get(dealLineKey({ barcode: "bc-JACKET" }))).toBe(20);
    expect(got.get(dealLineKey({ barcode: "bc-SOCKS" }))).toBe(20);
    expect(got.size).toBe(2);
    // The lines then add up at full price, and the deal takes the rest.
    const lines = golden.replaced.lines.reduce((s, l) => s + l.lineTotalCents, 0);
    expect(lines).toBe(12000);
    expect(lines - golden.replaced.invoiceDiscountCents).toBe(golden.replaced.discountedSubtotalCents);
  });

  it("marks nothing where the 20% stayed", () => {
    expect(replacedDiscountsByLine(golden.kept.lines as DealLine[]).size).toBe(0);
    expect(golden.kept.discountedSubtotalCents).toBe(3200);
  });

  it("keeps the biggest of a line split over several quote lines, apart by sale option and ref", () => {
    const got = replacedDiscountsByLine([
      line({ replacedDiscount: 20 }),
      line({ free: true, replacedDiscount: 30 }),
      line({ saleOption: "Winter", replacedDiscount: 10 }),
      line({ barcode: "unk", lineRef: "u1", replacedDiscount: 15 }),
      line({ barcode: "unk", lineRef: "u2" }),
      line({ barcode: "bc-Z", replacedDiscount: 0 }),
    ]);
    expect(got.get(dealLineKey({ barcode: "bc-J" }))).toBe(30);
    expect(got.get(dealLineKey({ barcode: "bc-J", saleOption: "Winter" }))).toBe(10);
    expect(got.get(dealLineKey({ lineRef: "u1" }))).toBe(15);
    expect(got.has(dealLineKey({ lineRef: "u2" }))).toBe(false);
    expect(got.has(dealLineKey({ barcode: "bc-Z" }))).toBe(false);
  });

  it("is empty for no lines", () => {
    expect(replacedDiscountsByLine([]).size).toBe(0);
  });
});
