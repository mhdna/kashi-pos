import { describe, expect, it } from "vitest";
import { unknownScanBarcode, unknownUnitsOf, validGTIN } from "./unknownScan";

// The user's label: 8000000092403.
describe("unknownScanBarcode", () => {
  it("a real barcode the catalog doesn't know is carried over as scanned", () => {
    expect(unknownScanBarcode("8000000092403")).toBe("8000000092403");
    expect(unknownScanBarcode(" 8000000092403 ")).toBe("8000000092403");
    expect(unknownScanBarcode("036000291452")).toBe("036000291452"); // UPC-A, leading zero kept
    expect(unknownScanBarcode("96385074")).toBe("96385074"); // EAN-8
    expect(unknownScanBarcode("10614141000415")).toBe("10614141000415"); // GTIN-14
  });

  it("codes, words, short numbers and bad check digits are not", () => {
    for (const v of ["", "CS3850-", "cup", "123", "8000000092404", "800000009240", "8000 000092403", "١٢٣٤٥٦٧٨"]) {
      expect(unknownScanBarcode(v)).toBe("");
    }
  });

  it("any single digit misread anywhere is caught", () => {
    const good = "8000000092403";
    for (let i = 0; i < good.length; i++) {
      for (let d = 0; d <= 9; d++) {
        if (String(d) === good[i]) continue;
        expect(validGTIN(good.slice(0, i) + d + good.slice(i + 1))).toBe(false);
      }
    }
  });

  it("random bodies have exactly one check digit that fits", () => {
    let seed = 7;
    const rand = () => ((seed = (seed * 1103515245 + 12345) % 2 ** 31) / 2 ** 31);
    for (let n = 0; n < 500; n++) {
      const len = [7, 11, 12, 13][Math.floor(rand() * 4)] ?? 13;
      let body = "";
      for (let i = 0; i < len; i++) body += Math.floor(rand() * 10);
      let fits = 0;
      for (let d = 0; d <= 9; d++) if (validGTIN(body + d)) fits++;
      expect(fits).toBe(1);
    }
  });
});

describe("unknownUnitsOf", () => {
  const isUnknown = (i: any) => i.barcode === "UNKNOWN";
  it("one unit per piece, one line ref per line, the scanned barcode on each unit", () => {
    const units = unknownUnitsOf(
      [
        { barcode: "UNKNOWN", qty: 2, price: 12, unknownRef: "a", unknownBarcode: "8000000092403" },
        { barcode: "123", qty: 1, price: 5 },
        { barcode: "UNKNOWN", qty: 1, price: 3.5, unknownRef: "b" },
      ],
      isUnknown,
    );
    expect(units).toEqual([
      { label: "Unknown item $12.00", unitPriceCents: 1200, discount: 0, lineRef: "a", slotRef: "a:1", barcode: "8000000092403" },
      { label: "Unknown item $12.00", unitPriceCents: 1200, discount: 0, lineRef: "a", slotRef: "a:2", barcode: "8000000092403" },
      { label: "Unknown item $3.50", unitPriceCents: 350, discount: 0, lineRef: "b", slotRef: "b:1" },
    ]);
  });

  it("per-unit prices and discounts, returns skipped, old lines go by position", () => {
    const units = unknownUnitsOf(
      [
        { barcode: "UNKNOWN", qty: -1, price: 9 },
        { barcode: "UNKNOWN", qty: 2, price: 10, unitPrices: [10, 9.99], unitDiscounts: [0, 15] },
      ],
      isUnknown,
    );
    expect(units.map((u) => [u.unitPriceCents, u.discount, u.lineRef, u.slotRef])).toEqual([
      [1000, 0, "1", "1:1"],
      [999, 15, "1", "1:2"],
    ]);
    expect(units.every((u) => !("barcode" in u))).toBe(true);
  });
});
