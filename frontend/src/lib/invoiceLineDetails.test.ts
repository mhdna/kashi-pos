import { describe, expect, it } from "vitest";
import golden from "./testdata/invoice_line_details.json";
import { invoiceLineDetails, type InvoiceLineProduct } from "./invoiceLineDetails";

const pairs = (line: InvoiceLineProduct | null | undefined) => invoiceLineDetails(line).map((d) => `${d.label}: ${d.value}`);

describe("invoiceLineDetails", () => {
  it("tells apart the two Lady Leather Skirts from the real backend (golden)", () => {
    // Written by TestInvoiceDetailsCarryEveryProductDetail from a real checkout.
    const [full, bare] = golden as InvoiceLineProduct[];
    expect(pairs(full)).toEqual([
      "Code: TNV551219-16",
      "Barcode: 8000000113603",
      "Color: Black",
      "Size: 16",
      "Category: Skirts",
      "Brand: Tannav",
      "Kind: Leather",
      "Family: Bottoms",
      "Season: Winter",
      "Year: 2026",
    ]);
    // A bare product shows only what it has, not empty labels or "Year: 0".
    expect(pairs(bare)).toEqual(["Code: TNV551219", "Barcode: 8000000114129"]);
    expect(pairs(full)).not.toEqual(pairs(bare));
  });

  it("skips blanks, '-', whitespace and a zero or missing year", () => {
    expect(
      pairs({ code: "  ", barcode: "-", color: " - ", size: "", category: null, brand: undefined, productYear: 0, season: " " }),
    ).toEqual([]);
    expect(pairs({ productYear: "0" })).toEqual([]);
    expect(pairs({ productYear: "abc" })).toEqual([]);
    expect(pairs({ productYear: -1 })).toEqual([]);
    expect(pairs({ productYear: "2025" })).toEqual(["Year: 2025"]);
    expect(pairs(null)).toEqual([]);
    expect(pairs(undefined)).toEqual([]);
  });

  it("trims values and keeps a code that is only digits", () => {
    expect(pairs({ code: " 0042 ", barcode: " 8000000000777 " })).toEqual(["Code: 0042", "Barcode: 8000000000777"]);
  });

  it("always lists fields in a fixed order, whatever the input order (randomized)", () => {
    const order = ["code", "barcode", "color", "size", "category", "brand", "kind", "family", "season", "productYear"] as const;
    let seed = 7;
    const rand = () => ((seed = (seed * 1103515245 + 12345) % 2 ** 31) / 2 ** 31);
    for (let run = 0; run < 500; run++) {
      const line: Record<string, unknown> = {};
      const expected: string[] = [];
      const keys = [...order].sort(() => rand() - 0.5);
      for (const key of keys) {
        const roll = rand();
        if (roll < 0.25) line[key] = "";
        else if (roll < 0.35) line[key] = "-";
        else if (roll < 0.45) continue;
        else line[key] = key === "productYear" ? 2000 + Math.floor(rand() * 30) : `${key}-${run}`;
      }
      for (const key of order) {
        const v = line[key];
        if (v !== undefined && v !== "" && v !== "-") expected.push(String(v));
      }
      expect(invoiceLineDetails(line as InvoiceLineProduct).map((d) => d.value)).toEqual(expected);
    }
  });
});
