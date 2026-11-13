import { describe, expect, it } from "vitest";
import { buildHoldSnapshot, heldCart, heldSaleLine, holdKind, holdRestore, holdTitle } from "./holdSnapshot";
import heldSale from "./testdata/held_sale.json";

// A returned line as the exchange puts it in the cart (InvoiceItems
// addExchangeItems): negative, locked, at its sold price.
const returned = {
  code: "CA24LWVZ12",
  barcode: "bc-vizon",
  name: "Lady Winter Vizon",
  price: 12,
  discount: 0,
  qty: -1,
  soldUnitPrice: 12,
  soldUnitDiscount: 0,
  unitPrices: [],
  unitDiscounts: [],
  locked: true,
  originalPrice: 12,
};
const unknown = {
  code: "UNKNOWN",
  barcode: "UNKNOWN",
  name: "Unknown item",
  price: 30,
  qty: 1,
  discount: 0,
  unitPrices: [30],
  unitDiscounts: [0],
  originalPrice: 30,
  originalDiscount: 0,
  note: "",
  unknownRef: "u1",
};
// A line as the cart builds it (InvoiceItems addItem).
const socks = {
  code: "L6KS01.5",
  barcode: "bc-socks",
  name: "Kids Socks",
  price: 1.5,
  discount: 0,
  qty: 2,
  unitPrices: [1.5, 1.5],
  unitDiscounts: [0, 0],
  originalPrice: 1.5,
  originalDiscount: 0,
  note: "",
};

// Holding goes through JSON (HoldInvoice stores it as sent, ListHoldInvoices
// hands it back with an id), so the tests do too.
function heldAndListed(snapshot: unknown, id = 7) {
  return { ...JSON.parse(JSON.stringify(snapshot)), id, createdAt: "2026-11-07 10:00:00" };
}

describe("holding an exchange", () => {
  const exchange = { originalInvoiceId: 412, originalInvoiceCode: "SA/CT/2026/02900" };

  it("comes back as the same exchange, line for line", () => {
    const items = [unknown, returned];
    const hold = heldAndListed(
      buildHoldSnapshot({ items, client: { id: 3 }, exchange, notes: "size" }),
    );
    const restore = holdRestore(hold);
    expect(restore.mode).toBe("exchange");
    if (restore.mode !== "exchange") return;
    expect(restore.exchange).toEqual(exchange);
    expect(restore.items).toEqual(items);
    // The returned line is still a return: negative, locked, at its sold price.
    const back = restore.items.find((i) => i.code === "CA24LWVZ12");
    expect(back).toMatchObject({ qty: -1, locked: true, soldUnitPrice: 12 });
  });

  it("keeps what was held when the cart changes after holding", () => {
    const items = [{ ...returned }, { ...unknown }];
    const snapshot = buildHoldSnapshot({ items, exchange });
    items[0]!.qty = -5;
    expect(snapshot.items[0].qty).toBe(-1);
  });

  it("is titled as an exchange of its sale", () => {
    const hold = heldAndListed(
      buildHoldSnapshot({ items: [returned, unknown, socks], exchange }),
      12,
    );
    expect(holdTitle(hold)).toBe("#12 — Exchange of SA/CT/2026/02900 · 1 returned, 2 new");
  });

  it("works without the sale's code (an exchange started before it was kept)", () => {
    const hold = heldAndListed(
      buildHoldSnapshot({ items: [returned], exchange: { originalInvoiceId: 5 } }),
    );
    expect(holdRestore(hold)).toMatchObject({
      mode: "exchange",
      exchange: { originalInvoiceId: 5 },
    });
    expect(holdTitle(hold)).toBe("#7 — Exchange · 1 returned, 0 new");
  });

  it.each([
    ["no sale", null],
    ["a zero sale id", { originalInvoiceId: 0 }],
    ["a negative sale id", { originalInvoiceId: -3 }],
  ])("isn't kept as an exchange with %s", (_, ex) => {
    const snapshot = buildHoldSnapshot({ items: [socks], exchange: ex as any });
    expect(snapshot.exchange).toBeUndefined();
  });
});

describe("holding a sale", () => {
  it("comes back as a sale", () => {
    const hold = heldAndListed(buildHoldSnapshot({ items: [socks, unknown] }));
    expect(holdRestore(hold)).toEqual({ mode: "sale", items: [socks, unknown] });
    expect(holdTitle(hold)).toBe("#7 — 2 items");
  });

  it("has no exchange in it", () => {
    expect(buildHoldSnapshot({ items: [socks] })).not.toHaveProperty("exchange");
  });

  it("says one item", () => {
    expect(holdTitle(heldAndListed(buildHoldSnapshot({ items: [socks] })))).toBe("#7 — 1 item");
  });

  it("reads a hold from before snapshots (items listed from hold_invoice_items)", () => {
    const old = {
      id: 3,
      itemCount: 2,
      items: [
        { code: "A", qty: 1 },
        { code: "B", qty: 2 },
      ],
    };
    // Completed as addItem would build them, from their own price.
    expect(holdRestore(old)).toEqual({
      mode: "sale",
      items: [
        { code: "A", qty: 1, price: 0, discount: 0, unitPrices: [0], unitDiscounts: [0], originalPrice: 0, originalDiscount: 0, note: "" },
        { code: "B", qty: 2, price: 0, discount: 0, unitPrices: [0, 0], unitDiscounts: [0, 0], originalPrice: 0, originalDiscount: 0, note: "" },
      ],
    });
    expect(holdTitle(old)).toBe("#3 — 2 items");
  });
});

describe("a hold that doesn't make sense is never restored as one", () => {
  it("drops returned lines from a hold with no sale to return them to", () => {
    // A sale can't return anything: re-added as a sale, a -1 line would be
    // sold as a new unit.
    const hold = heldAndListed({ version: 1, items: [returned, socks] });
    expect(holdRestore(hold)).toEqual({ mode: "sale", items: [socks] });
  });

  it("restores an exchange hold that has lost its returned lines as a sale", () => {
    const hold = heldAndListed({ version: 1, items: [socks], exchange: { originalInvoiceId: 9 } });
    expect(holdRestore(hold)).toEqual({ mode: "sale", items: [socks] });
  });

  it.each([
    ["no items", { id: 1 }],
    ["items that aren't a list", { id: 1, items: "x" }],
    [
      "a sale id that isn't a number",
      { id: 1, items: [returned], exchange: { originalInvoiceId: "abc" } },
    ],
    ["a fractional sale id", { id: 1, items: [returned], exchange: { originalInvoiceId: 1.5 } }],
    ["nothing at all", null],
  ])("copes with %s", (_, hold) => {
    const restore = holdRestore(hold);
    expect(restore.mode).toBe("sale");
    expect(restore.items.every((i) => !(Number(i.qty) < 0))).toBe(true);
    expect(() => holdTitle(hold)).not.toThrow();
  });

  it("never restores a sale with a returned line, whatever the hold holds", () => {
    let seed = 7;
    const rand = () => (seed = (seed * 1103515245 + 12345) & 0x7fffffff) / 0x7fffffff;
    for (let run = 0; run < 2000; run++) {
      const items = Array.from({ length: Math.floor(rand() * 5) }, (_, i) => ({
        code: `C${i}`,
        qty: Math.floor(rand() * 7) - 3,
      }));
      const id = [0, -1, 5, 1.5, "7", undefined][Math.floor(rand() * 6)];
      const restore = holdRestore({
        items,
        exchange: id === undefined ? undefined : { originalInvoiceId: id },
      });
      if (restore.mode === "sale") {
        expect(restore.items.some((i) => i.qty < 0)).toBe(false);
      } else {
        expect(
          Number.isInteger(restore.exchange.originalInvoiceId) &&
            restore.exchange.originalInvoiceId > 0,
        ).toBe(true);
        expect(restore.items).toEqual(items);
        expect(restore.items.some((i) => i.qty < 0)).toBe(true);
      }
    }
  });
});

describe("holdKind", () => {
  const exchange = { originalInvoiceId: 412, originalInvoiceCode: "SA/CT/2026/02900" };
  it("marks a held exchange in the exchange's orange with the swap icon", () => {
    const hold = heldAndListed(buildHoldSnapshot({ items: [returned, unknown], exchange }));
    expect(holdKind(hold)).toEqual({
      kind: "exchange",
      label: "Exchange",
      icon: "mdi-swap-horizontal",
      color: "orange-darken-3",
    });
  });
  it("marks a held sale in blue with the receipt icon", () => {
    const hold = heldAndListed(buildHoldSnapshot({ items: [socks] }));
    expect(holdKind(hold)).toEqual({
      kind: "sale",
      label: "Sale",
      icon: "mdi-receipt-text-outline",
      color: "primary",
    });
  });
  it("marks a hold the way it will be restored, never the other way", () => {
    // An exchange hold with no returned lines left restores as a sale, and
    // a hold with returned lines but no sale too: both show as sales.
    expect(holdKind({ items: [socks], exchange: { originalInvoiceId: 9 } }).kind).toBe("sale");
    expect(holdKind({ items: [returned] }).kind).toBe("sale");
    expect(holdKind(null).kind).toBe("sale");
    for (const hold of [
      { items: [returned], exchange },
      { items: [socks], exchange },
      { items: [returned, socks] },
      { items: [] },
    ]) {
      expect(holdKind(hold).kind).toBe(holdRestore(hold).mode);
    }
  });
});

// What the user lost: central's price and discount, custom prices and
// discounts, and an unknown item's pictures. A held sale now comes back as it
// was held, with the sale's ref and picture session.
describe("a held sale comes back exactly as it was held", () => {
  const customPrice = { ...socks, code: "A", barcode: "bc-A", price: 85, qty: 2, unitPrices: [85, 90], unitDiscounts: [0, 0], originalPrice: 100, salespersonId: 3 };
  const customDiscount = { ...socks, code: "B", barcode: "bc-B", price: 80, qty: 1, discount: 15, unitPrices: [80], unitDiscounts: [15], originalPrice: 80, note: "gift wrap" };
  const centralPriced = {
    ...socks, code: "JACKET", barcode: "bc-JACKET", price: 90, qty: 1, discount: 50, unitPrices: [90], unitDiscounts: [50],
    originalPrice: 100, remote: { priceCents: 9000, discount: 50, noDeal: false, prevPrice: 100, prevDiscount: 0 },
  };
  const toggledOff = { ...socks, qty: 1, unitPrices: [1.5], unitDiscounts: [0], originalDiscount: 10, discountOff: true, dealOff: true, saleOption: "Summer" };
  const pictured = { ...unknown, unknownDeal: "none" };
  const items = [customPrice, customDiscount, centralPriced, toggledOff, pictured];
  const parts = {
    items,
    client: { id: 4, name: "Rana" },
    salesperson: { id: 3, fullName: "Cashier" },
    calculator: { currentInput: "20", activeMethod: "cash", entries: [{ method: "cash", amount: 20 }] },
    notes: "pick up at 5",
    saleRef: "sale-1",
    unknownClientRef: "pics-123",
  };
  const fresh = () => "fresh-ref";

  it("every line, the sale's ref, its picture session and the rest", () => {
    const held = heldCart(heldAndListed(buildHoldSnapshot(parts)), fresh);
    expect(held.restore).toEqual({ mode: "sale", items });
    expect(held).toMatchObject({
      saleRef: "sale-1",
      unknownClientRef: "pics-123",
      client: parts.client,
      salesperson: parts.salesperson,
      calculator: parts.calculator,
      notes: "pick up at 5",
    });
  });

  it("keeps two lines of one product at their own prices apart", () => {
    const second = { ...customPrice, price: 70, qty: 1, unitPrices: [70], unitDiscounts: [0] };
    const held = heldCart(heldAndListed(buildHoldSnapshot({ items: [customPrice, second] })), fresh);
    expect(held.restore.items).toEqual([customPrice, second]);
  });

  it("gives a hold from before the ref was kept a fresh one and no picture session", () => {
    const held = heldCart(heldAndListed({ version: 1, items: [socks] }), fresh);
    expect([held.saleRef, held.unknownClientRef, held.client, held.notes]).toEqual(["fresh-ref", "", null, ""]);
  });

  it("leaves the ref and session out of a hold that has none", () => {
    const snapshot = buildHoldSnapshot({ items: [socks], saleRef: "", unknownClientRef: "" });
    expect(snapshot).not.toHaveProperty("saleRef");
    expect(snapshot).not.toHaveProperty("unknownClientRef");
  });

  it("an anonymous client (id 0) comes back as none", () => {
    expect(heldCart(heldAndListed(buildHoldSnapshot({ items: [socks], client: { id: 0 } })), fresh).client).toBeNull();
  });

  it("puts back the backend's resumed hold as it was held (testdata/held_sale.json)", () => {
    const held = heldCart(heldSale, fresh);
    expect(held.restore.mode).toBe("sale");
    expect(held.restore.items).toEqual(heldSale.items);
    expect(held.saleRef).toBe("sale-1");
    expect(held.unknownClientRef).toBe("pics-123");
    const [a, b, jacket, socksLine, unknownLine] = held.restore.items;
    expect(a).toMatchObject({ price: 85, unitPrices: [85, 90] });
    expect(b).toMatchObject({ discount: 15, unitDiscounts: [15] });
    expect(jacket.remote).toMatchObject({ priceCents: 9000, discount: 50 });
    expect(socksLine).toMatchObject({ discountOff: true, dealOff: true, saleOption: "Summer" });
    expect(unknownLine).toMatchObject({ unknownRef: "u1abc", unknownDeal: "none" });
    expect(cartCents(held.restore.items)).toBe(cartCents(heldSale.items));
    expect(held.calculator).toEqual(heldSale.calculator);
    expect(heldSale).not.toHaveProperty("liveOverrides");
  });

  it("comes back at the same total, line for line, over a thousand random carts", () => {
    let seed = 11;
    const rand = () => (seed = (seed * 1103515245 + 12345) & 0x7fffffff) / 0x7fffffff;
    const pick = <T>(xs: T[]) => xs[Math.floor(rand() * xs.length)]!;
    for (let run = 0; run < 1000; run++) {
      const cart = Array.from({ length: 1 + Math.floor(rand() * 6) }, (_, i) => {
        const qty = 1 + Math.floor(rand() * 4);
        const unitPrices = Array.from({ length: qty }, () => Math.round(rand() * 50000) / 100);
        const unitDiscounts = Array.from({ length: qty }, () => Math.floor(rand() * 101));
        const line: any = {
          code: `C${i}`, barcode: `bc-${i}`, name: `Item ${i}`, qty, price: unitPrices[0], discount: unitDiscounts[0],
          unitPrices, unitDiscounts, originalPrice: Math.round(rand() * 50000) / 100, originalDiscount: Math.floor(rand() * 101),
          note: pick(["", "gift", "size M"]), salespersonId: Math.floor(rand() * 4),
        };
        if (rand() < 0.3) line.discountOff = true;
        if (rand() < 0.3) line.dealOff = true;
        if (rand() < 0.3) line.saleOption = pick(["Summer", "Winter"]);
        if (rand() < 0.3) line.remote = { priceCents: Math.round(unitPrices[0]! * 100), discount: null, noDeal: rand() < 0.5, prevPrice: 1, prevDiscount: 0 };
        if (rand() < 0.2) Object.assign(line, { code: "UNKNOWN", barcode: "UNKNOWN", unknownRef: `u${run}-${i}` });
        return line;
      });
      const ref = `sale-${run}`;
      const held = heldCart(heldAndListed(buildHoldSnapshot({ items: cart, saleRef: ref, unknownClientRef: `p${run}` }), run), fresh);
      expect(held.restore.items).toEqual(cart);
      expect(cartCents(held.restore.items)).toBe(cartCents(cart));
      expect([held.saleRef, held.unknownClientRef]).toEqual([ref, `p${run}`]);
    }
  });

  it("one cent off anywhere is caught", () => {
    const back = heldCart(heldAndListed(buildHoldSnapshot({ items })), fresh).restore.items;
    back[0].unitPrices[1] = 90.01;
    expect(cartCents(back)).not.toBe(cartCents(items));
  });

  it("a complete line is never changed by completing it", () => {
    for (const line of [...items, socks, unknown]) expect(heldSaleLine(line)).toEqual(line);
  });
});

// A cart's total before deals, in cents: each unit at its price less its
// discount.
function cartCents(items: any[]): number {
  return items.reduce(
    (sum, item) =>
      sum +
      (item.unitPrices as number[]).reduce(
        (s: number, price: number, i: number) => s + Math.round(price * 100 * (100 - item.unitDiscounts[i])) ,
        0,
      ),
    0,
  );
}
