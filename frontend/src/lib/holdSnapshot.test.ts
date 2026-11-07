import { describe, expect, it } from "vitest";
import { buildHoldSnapshot, holdKind, holdRestore, holdTitle } from "./holdSnapshot";

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
  unitPrices: [30],
  unitDiscounts: [0],
  unknownRef: "u1",
};
const socks = { code: "L6KS01.5", barcode: "bc-socks", name: "Kids Socks", price: 1.5, qty: 2 };

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
    expect(holdRestore(old)).toEqual({ mode: "sale", items: old.items });
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
