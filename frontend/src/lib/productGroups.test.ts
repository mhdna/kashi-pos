import { describe, expect, it } from "vitest";
import { colorAvailable, groupProducts, matching, priceLabel, sizeAvailable, sortSizes, type VariantLike } from "./productGroups";

let nextId = 1;
const v = (code: string, color = "", size = "", price = 29): VariantLike & { name: string } => ({ id: nextId++, code, color, size, price, name: code });

// The till's example: 4 colours x 2 sizes on one code, 4 colours on another.
function shoes() {
  nextId = 13406;
  const rows = [];
  for (const size of ["20-25", "25-30"]) for (const color of ["Grey", "Beige", "White", "Black"]) rows.push(v("C6KSHE29-LC", color, size, 29));
  for (const color of ["Grey", "Beige", "White", "Black"]) rows.push(v("C6KSHE35-LC", color, "31-36", 35));
  return rows;
}

describe("groupProducts", () => {
  it("shows the 12 shoe rows as two products", () => {
    const groups = groupProducts(shoes());
    expect(groups.map((g) => g.key)).toEqual(["C6KSHE29-LC", "C6KSHE35-LC"]);
    expect(groups[0]!.variants).toHaveLength(8);
    expect(groups[0]!.colors).toEqual(["Grey", "Beige", "White", "Black"]);
    expect(groups[0]!.sizes).toEqual(["20-25", "25-30"]);
    expect(groups[1]!.sizes).toEqual(["31-36"]);
    expect([groups[0]!.minPrice, groups[0]!.maxPrice]).toEqual([29, 29]);
  });

  it("keeps the order in which codes first appear, even when variants interleave", () => {
    nextId = 1;
    const groups = groupProducts([v("B", "Red"), v("A", "Red"), v("B", "Blue"), v("C"), v("A", "Blue")]);
    expect(groups.map((g) => g.key)).toEqual(["B", "A", "C"]);
    expect(groups[0]!.variants.map((x) => x.color)).toEqual(["Red", "Blue"]);
  });

  it("gives each product with no code its own group", () => {
    nextId = 1;
    const groups = groupProducts([v(""), v(""), v("X")]);
    expect(groups.map((g) => g.key)).toEqual(["id:1", "id:2", "X"]);
  });

  it("treats codes that differ only in case as different (the search pages by exact code)", () => {
    nextId = 1;
    expect(groupProducts([v("ab1"), v("AB1")])).toHaveLength(2);
  });

  it("merges a code's variants that arrive on a later page", () => {
    const all = shoes();
    const first = groupProducts(all.slice(0, 5));
    expect(first[0]!.variants).toHaveLength(5);
    const later = groupProducts(all);
    expect(later[0]!.variants).toHaveLength(8);
    expect(later[0]!.first).toBe(first[0]!.first);
  });

  it("lists each colour and size once, whatever the case or spaces", () => {
    nextId = 1;
    const [g] = groupProducts([v("A", "Black", "M"), v("A", "black ", "m"), v("A", "", "")]);
    expect(g!.colors).toEqual(["Black"]);
    expect(g!.sizes).toEqual(["M"]);
  });

  it("lists sizes in wearing order, not the order the variants came", () => {
    nextId = 1;
    const [g] = groupProducts([v("T", "Red", "XL"), v("T", "Red", "S"), v("T", "Red", "M")]);
    expect(g!.sizes).toEqual(["S", "M", "XL"]);
  });

  it("spans the prices when variants differ", () => {
    nextId = 1;
    const [g] = groupProducts([v("A", "Red", "S", 20), v("A", "Red", "XL", 25.5)]);
    expect([g!.minPrice, g!.maxPrice]).toEqual([20, 25.5]);
  });

  it("holds every product exactly once (randomised)", () => {
    let seed = 42;
    const rnd = (n: number) => ((seed = (seed * 1103515245 + 12345) % 2147483648), seed % n);
    for (let round = 0; round < 300; round++) {
      nextId = 1;
      const codes = ["A", "B", "C", "", "d", "D"];
      const colors = ["", "Red", "red", "Blue", "Black/White"];
      const sizes = ["", "S", "M", "20-25", "38"];
      const items = Array.from({ length: rnd(40) }, () => v(codes[rnd(6)]!, colors[rnd(5)], sizes[rnd(5)], rnd(100)));
      const groups = groupProducts(items);
      const flat = groups.flatMap((g) => g.variants);
      expect(flat).toHaveLength(items.length);
      expect(new Set(flat.map((x) => x.id)).size).toBe(items.length);
      for (const g of groups) {
        for (const x of g.variants) expect(x.code ? x.code : `id:${x.id}`).toBe(g.key);
        expect(g.first).toBe(g.variants[0]);
        const prices = g.variants.map((x) => x.price!);
        expect(g.minPrice).toBe(Math.min(...prices));
        expect(g.maxPrice).toBe(Math.max(...prices));
        const wantColors = new Set(g.variants.map((x) => (x.color ?? "").trim().toLowerCase()).filter(Boolean));
        expect(new Set(g.colors.map((c) => c.toLowerCase()))).toEqual(wantColors);
        // Every offered colour/size pair that's available really has a variant, and vice versa.
        for (const c of g.colors)
          for (const s of g.sizes) {
            const has = g.variants.some((x) => x.color?.trim().toLowerCase() === c.toLowerCase() && x.size?.trim().toLowerCase() === s.toLowerCase());
            expect(colorAvailable(g.variants, c, s)).toBe(has);
            expect(sizeAvailable(g.variants, s, c)).toBe(has);
          }
      }
    }
  });
});

describe("sortSizes", () => {
  it("puts letter sizes in wearing order", () => {
    expect(sortSizes(["XL", "S", "M", "XS", "L", "XXL"])).toEqual(["XS", "S", "M", "L", "XL", "XXL"]);
    expect(sortSizes(["3XL", "2XL", "XL"])).toEqual(["XL", "2XL", "3XL"]);
  });
  it("orders numbers and ranges by their first number", () => {
    expect(sortSizes(["40", "38", "42", "36.5"])).toEqual(["36.5", "38", "40", "42"]);
    expect(sortSizes(["31-36", "20-25", "25-30"])).toEqual(["20-25", "25-30", "31-36"]);
    expect(sortSizes(["10Y", "2Y", "6Y"])).toEqual(["2Y", "6Y", "10Y"]);
  });
  it("keeps letters before numbers before the rest, the rest as they came", () => {
    expect(sortSizes(["One size", "M", "40", "Free", "S"])).toEqual(["S", "M", "40", "One size", "Free"]);
  });
});

describe("matching", () => {
  it("narrows by colour, size, both or neither", () => {
    const rows = shoes().slice(0, 8);
    expect(matching(rows, null, null)).toHaveLength(8);
    expect(matching(rows, "Grey", null)).toHaveLength(2);
    expect(matching(rows, null, "25-30")).toHaveLength(4);
    const one = matching(rows, "grey", "25-30");
    expect(one).toHaveLength(1);
    expect(one[0]!.id).toBe(13410);
    expect(matching(rows, "Pink", null)).toEqual([]);
  });
  it("reports which choices are left", () => {
    nextId = 1;
    const rows = [v("A", "Navy", "31-36"), v("A", "Pink", "31-36"), v("A", "Black", "20-25")];
    expect(colorAvailable(rows, "Navy", "20-25")).toBe(false);
    expect(colorAvailable(rows, "Black", "20-25")).toBe(true);
    expect(sizeAvailable(rows, "20-25", "Pink")).toBe(false);
    expect(sizeAvailable(rows, "31-36", null)).toBe(true);
  });
});

describe("priceLabel", () => {
  it("shows one price or a range", () => {
    expect(priceLabel(29, 29)).toBe("$29.00");
    expect(priceLabel(29, 35)).toBe("$29.00 – $35.00");
    expect(priceLabel(0, 0)).toBe("$0.00");
  });
});
