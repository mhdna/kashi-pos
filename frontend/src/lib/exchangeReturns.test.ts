import { describe, expect, it } from "vitest";
import { effectiveExchangePrice, type ExchangeItem } from "./exchange";
import {
  asReturnedLine,
  cartReturnKey,
  returnedCartLine,
  returnQtyProblem,
  returnRows,
  saleItemKey,
  selectionItem,
  setReturnedQty,
  totalReturned,
} from "./exchangeReturns";

// The sale from the shop floor: a Buy 1 Get 1 of kids' clothes.
const pants: ExchangeItem = {
  code: "BOC5KBJOP9",
  barcode: "bc-pants",
  name: "Kids Boy Jogging Pants",
  price: 9,
  discount: 0,
  available: 1,
  deal: "Buy 1 Get 1",
  freeQty: 1,
};
const hoodie: ExchangeItem = {
  code: "BOC5KBH15",
  barcode: "bc-hoodie",
  name: "Kids Boy Hoodie",
  price: 15,
  discount: 0,
  available: 2,
};
const jacket: ExchangeItem = {
  code: "CA24KGJK29",
  barcode: "bc-jacket",
  name: "Kids Girl Jacket",
  price: 29,
  discount: 10,
  available: 1,
};
const sale = [pants, hoodie, jacket];
const newShirt = {
  code: "SHIRT",
  barcode: "bc-shirt",
  name: "Shirt",
  price: 20,
  qty: 1,
  unitPrices: [20],
  unitDiscounts: [0],
};

// What the cart held before this module built returned lines (InvoiceItems
// addExchangeItems over useExchangeDialog's selection), kept here to prove
// the shared builder gives exactly the same line.
function oldReturnedLine(item: ExchangeItem, roundingMode = "half_up") {
  const selected = {
    ...item,
    soldUnitPrice: Number(item.price),
    soldUnitDiscount: Number(item.discount),
    price: effectiveExchangePrice(item, roundingMode),
    discount: 0,
    qty: 1,
  };
  return {
    ...selected,
    category: (selected as any).category ?? "",
    qty: -Number(selected.qty || 1),
    price: Number(selected.price),
    discount: 0,
    soldUnitPrice: Number(selected.soldUnitPrice ?? selected.price),
    soldUnitDiscount: Number(selected.soldUnitDiscount ?? selected.discount),
    unitPrices: [],
    unitDiscounts: [],
    locked: true,
    originalPrice: Number((selected as any).originalPrice ?? selected.price),
  };
}

describe("the returned line", () => {
  it.each(sale.map((i) => [i.name, i] as const))(
    "for %s is what the exchange always put in the cart",
    (_, item) => {
      expect(returnedCartLine(item, 1)).toEqual(oldReturnedLine(item));
      expect(asReturnedLine(selectionItem(item, 1))).toEqual(oldReturnedLine(item));
    },
  );

  it("is negative, locked and priced at what one unit was paid", () => {
    const line = returnedCartLine(jacket, 1);
    expect(line).toMatchObject({
      qty: -1,
      locked: true,
      price: 26.1,
      discount: 0,
      soldUnitPrice: 29,
      soldUnitDiscount: 10,
    });
    expect(returnedCartLine(hoodie, 2).qty).toBe(-2);
  });

  it("matches its sold item", () => {
    for (const item of sale)
      expect(cartReturnKey(returnedCartLine(item, 1))).toBe(saleItemKey(item));
  });
});

describe("returnRows", () => {
  it("counts what the cart returns of each sold item", () => {
    const cart = [returnedCartLine(pants, 1), returnedCartLine(jacket, 1), newShirt];
    expect(returnRows(sale, cart).map((r) => [r.item.code, r.returned, r.available])).toEqual([
      ["BOC5KBJOP9", 1, 1],
      ["BOC5KBH15", 0, 2],
      ["CA24KGJK29", 1, 1],
    ]);
  });

  it("adds up an item split over two cart lines", () => {
    const cart = [returnedCartLine(hoodie, 1), returnedCartLine(hoodie, 1)];
    expect(returnRows(sale, cart)[1]!.returned).toBe(2);
  });

  it("goes by code (any case), barcode and what it was sold at", () => {
    const cheap = { ...hoodie, price: 12, available: 1 };
    const rows = returnRows(
      [hoodie, cheap],
      [{ ...returnedCartLine(hoodie, 1), code: " boc5kbh15 " }],
    );
    expect(rows.map((r) => r.returned)).toEqual([1, 0]);
    // Same product, another variant (barcode): not this row.
    expect(
      returnRows([hoodie], [{ ...returnedCartLine(hoodie, 1), barcode: "bc-other" }])[0]!.returned,
    ).toBe(0);
  });

  it("never counts a sold line", () => {
    expect(
      totalReturned(
        returnRows(sale, [{ ...newShirt, code: hoodie.code, barcode: hoodie.barcode, price: 15 }]),
      ),
    ).toBe(0);
  });
});

describe("setReturnedQty", () => {
  const start = () => [returnedCartLine(pants, 1), returnedCartLine(jacket, 1), newShirt];

  it("returns one more of an item already returned", () => {
    const cart = [returnedCartLine(hoodie, 1), newShirt];
    const out = setReturnedQty(cart, sale, saleItemKey(hoodie), 2);
    expect(out[0].qty).toBe(-2);
    expect(out[1]).toBe(newShirt);
  });

  it("returns an item that wasn't, after the other returned lines", () => {
    const cart = start();
    const out = setReturnedQty(cart, sale, saleItemKey(hoodie), 1);
    expect(out.map((l) => [l.code, l.qty])).toEqual([
      ["BOC5KBJOP9", -1],
      ["CA24KGJK29", -1],
      ["BOC5KBH15", -1],
      ["SHIRT", 1],
    ]);
    expect(out[2]).toEqual(returnedCartLine(hoodie, 1));
  });

  it("puts an item back on the sale (not returned) at 0", () => {
    const out = setReturnedQty(start(), sale, saleItemKey(pants), 0);
    expect(out.map((l) => l.code)).toEqual(["CA24KGJK29", "SHIRT"]);
  });

  it("leaves every other line as it is, the same object", () => {
    const cart = start();
    const out = setReturnedQty(cart, sale, saleItemKey(hoodie), 2);
    expect(out[0]).toBe(cart[0]);
    expect(out[1]).toBe(cart[1]);
    expect(out[3]).toBe(cart[2]);
    expect(cart).toHaveLength(3); // the old cart untouched
  });

  it("keeps the line's salesperson and note when its count changes", () => {
    const cart = [{ ...returnedCartLine(hoodie, 1), salespersonId: 4, note: "torn" }];
    expect(setReturnedQty(cart, sale, saleItemKey(hoodie), 2)[0]).toMatchObject({
      qty: -2,
      salespersonId: 4,
      note: "torn",
    });
  });

  it("folds an item split over two lines into one", () => {
    const cart = [returnedCartLine(hoodie, 1), newShirt, returnedCartLine(hoodie, 1)];
    const out = setReturnedQty(cart, sale, saleItemKey(hoodie), 1);
    expect(out.map((l) => [l.code, l.qty])).toEqual([
      ["BOC5KBH15", -1],
      ["SHIRT", 1],
    ]);
  });

  it.each([
    [
      "more than the sale has left",
      saleItemKey(pants),
      2,
      "The sale has only 1 of these left to return.",
    ],
    [
      "more than the sale has left (several)",
      saleItemKey(hoodie),
      3,
      "The sale has only 2 of these left to return.",
    ],
    ["a negative count", saleItemKey(pants), -1, "Enter a whole number of units."],
    ["a part of a unit", saleItemKey(hoodie), 1.5, "Enter a whole number of units."],
    ["an item not on the sale", "nope", 1, "This item isn't on the sale."],
  ])("refuses %s", (_, key, qty, message) => {
    const cart = start();
    expect(returnQtyProblem(returnRows(sale, cart), key, qty)).toBe(message);
    expect(() => setReturnedQty(cart, sale, key, qty)).toThrow(message);
  });

  it("never lets an exchange return nothing", () => {
    const cart = [returnedCartLine(pants, 1), newShirt];
    const message =
      "An exchange returns at least one item. To return nothing, cancel the exchange.";
    expect(() => setReturnedQty(cart, sale, saleItemKey(pants), 0)).toThrow(message);
    // With another item returned, it can go.
    const two = setReturnedQty(cart, sale, saleItemKey(hoodie), 1);
    expect(() => setReturnedQty(two, sale, saleItemKey(pants), 0)).not.toThrow();
  });
});

// Any sequence of changes a cashier can make keeps the cart sound: every
// count within what the sale has, at least one unit returned, the new items
// untouched and in order, and every returned line a proper returned line.
describe("every sequence of changes keeps the cart sound", () => {
  it("over 3,000 random sequences", () => {
    let seed = 99;
    const rand = () => (seed = (seed * 1103515245 + 12345) & 0x7fffffff) / 0x7fffffff;
    const int = (n: number) => Math.floor(rand() * n);
    for (let run = 0; run < 3000; run++) {
      const items: ExchangeItem[] = Array.from({ length: 1 + int(5) }, (_, i) => ({
        code: `P${i}`,
        barcode: `b${i}`,
        name: `Item ${i}`,
        price: 1 + int(50),
        discount: [0, 0, 10, 25][int(4)]!,
        available: 1 + int(3),
      }));
      const sold = [
        { ...newShirt, code: "N1" },
        { ...newShirt, code: "N2" },
      ].slice(0, int(3));
      let cart: any[] = [returnedCartLine(items[0]!, 1), ...sold];
      for (let step = 0; step < 12; step++) {
        const item = items[int(items.length)]!;
        const rows = returnRows(items, cart);
        const current = rows.find((r) => r.key === saleItemKey(item))!.returned;
        const qty = current + int(5) - 2;
        const problem = returnQtyProblem(rows, saleItemKey(item), qty);
        if (problem) {
          expect(() => setReturnedQty(cart, items, saleItemKey(item), qty)).toThrow();
          continue;
        }
        cart = setReturnedQty(cart, items, saleItemKey(item), qty);
        const after = returnRows(items, cart);
        expect(after.find((r) => r.key === saleItemKey(item))!.returned).toBe(qty);
      }
      const rows = returnRows(items, cart);
      expect(totalReturned(rows)).toBeGreaterThanOrEqual(1);
      for (const r of rows) expect(r.returned >= 0 && r.returned <= r.available).toBe(true);
      expect(cart.filter((l) => l.qty > 0)).toEqual(sold);
      for (const l of cart.filter((l) => l.qty < 0)) {
        expect(Number.isInteger(l.qty)).toBe(true);
        expect(l.locked).toBe(true);
        const item = items.find((i) => saleItemKey(i) === cartReturnKey(l))!;
        expect(l.price).toBe(effectiveExchangePrice(item));
      }
      // One line per returned item, never two.
      const keys = cart.filter((l) => l.qty < 0).map(cartReturnKey);
      expect(new Set(keys).size).toBe(keys.length);
    }
  });
});
