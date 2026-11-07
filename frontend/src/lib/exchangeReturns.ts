// What an exchange returns, changed while it is on screen: the "Returned
// items" picker lists what is left to return on the sale (GetExchangeInvoice,
// deal units merged as the exchange dialog shows them) and how many of each
// the cart returns now. Changing a count changes the cart's returned line:
// up to what the sale has left, down to 0 ("not returned": the item stays on
// the sale). An exchange always returns at least one unit; to return nothing,
// cancel the exchange.
//
// The returned line itself is built here for both ways in (the exchange
// dialog's selection, and this picker), so they can't differ.
import { effectiveExchangePrice, normalizeExchangeCode, type ExchangeItem } from "@/lib/exchange";

// A sold item picked for return, as the exchange dialog keeps it: priced at
// what one unit was paid (its discount taken off), the sold price and
// discount kept to match it to its sold line.
export function selectionItem(
  saleItem: ExchangeItem,
  qty: number,
  roundingMode = "half_up",
): ExchangeItem {
  return {
    ...saleItem,
    soldUnitPrice: Number(saleItem.price),
    soldUnitDiscount: Number(saleItem.discount || 0),
    price: effectiveExchangePrice(saleItem, roundingMode),
    discount: 0,
    qty,
  };
}

// The cart line returning a selection item: negative, locked, at its price.
export function asReturnedLine(selected: ExchangeItem): any {
  return {
    ...selected,
    category: selected.category ?? "",
    price: Number(selected.price),
    discount: 0,
    qty: -Number(selected.qty || 1),
    soldUnitPrice: Number(selected.soldUnitPrice ?? selected.price),
    soldUnitDiscount: Number(selected.soldUnitDiscount ?? selected.discount),
    unitPrices: [],
    unitDiscounts: [],
    locked: true,
    originalPrice: Number((selected as any).originalPrice ?? selected.price),
  };
}

// The cart line returning qty units of a sold item.
export function returnedCartLine(
  saleItem: ExchangeItem,
  qty: number,
  roundingMode = "half_up",
): any {
  return asReturnedLine(selectionItem(saleItem, qty, roundingMode));
}

// One of the sale's items as the picker shows it.
export interface ReturnRow {
  key: string;
  item: ExchangeItem;
  // What the sale has left to return (returns already checked out are off).
  available: number;
  // What the cart returns now.
  returned: number;
}

// A sold item and the cart's returned line for it share the product and
// what it was sold at.
export function saleItemKey(item: ExchangeItem): string {
  return returnKey(item.code, item.barcode, Number(item.price), Number(item.discount || 0));
}

function returnKey(
  code: string,
  barcode: string | undefined,
  price: number,
  discount: number,
): string {
  return `${normalizeExchangeCode(code)}|${String(barcode ?? "").trim()}|${Math.round(price * 100)}|${Math.trunc(discount)}`;
}

// The key of a returned cart line, or "" for a line that isn't a return.
export function cartReturnKey(line: any): string {
  if (!(Number(line?.qty) < 0)) return "";
  return returnKey(
    String(line.code ?? ""),
    line.barcode,
    Number(line.soldUnitPrice ?? line.price),
    Number(line.soldUnitDiscount ?? line.discount ?? 0),
  );
}

export function returnRows(saleItems: ExchangeItem[], cart: any[]): ReturnRow[] {
  const inCart = new Map<string, number>();
  for (const line of cart) {
    const key = cartReturnKey(line);
    if (key) inCart.set(key, (inCart.get(key) ?? 0) - Math.trunc(Number(line.qty)));
  }
  return saleItems.map((item) => {
    const key = saleItemKey(item);
    return {
      key,
      item,
      available: Math.max(0, Math.trunc(Number(item.available ?? 0))),
      returned: inCart.get(key) ?? 0,
    };
  });
}

export function totalReturned(rows: ReturnRow[]): number {
  return rows.reduce((sum, row) => sum + row.returned, 0);
}

// Why a row can't be set to qty, or "" when it can.
export function returnQtyProblem(rows: ReturnRow[], key: string, qty: number): string {
  const row = rows.find((r) => r.key === key);
  if (!row) return "This item isn't on the sale.";
  if (!Number.isInteger(qty) || qty < 0) return "Enter a whole number of units.";
  if (qty > row.available) {
    return row.available === 1
      ? "The sale has only 1 of these left to return."
      : `The sale has only ${row.available} of these left to return.`;
  }
  if (totalReturned(rows) - row.returned + qty < 1) {
    return "An exchange returns at least one item. To return nothing, cancel the exchange.";
  }
  return "";
}

// The cart with the sold item returned qty times: its returned line
// changed, added (after the other returned lines) or removed. Every other
// line is left as it is, the same object. Throws on a qty returnQtyProblem
// refuses, so a bad count never reaches the cart.
export function setReturnedQty(
  cart: any[],
  saleItems: ExchangeItem[],
  key: string,
  qty: number,
  roundingMode = "half_up",
): any[] {
  const rows = returnRows(saleItems, cart);
  const problem = returnQtyProblem(rows, key, qty);
  if (problem) throw new Error(problem);
  const row = rows.find((r) => r.key === key)!;
  const out: any[] = [];
  let placed = false;
  for (const line of cart) {
    if (cartReturnKey(line) !== key) {
      out.push(line);
      continue;
    }
    // The first returned line of the item takes the new count (keeping its
    // salesperson and note); any others of it go.
    if (!placed && qty > 0) out.push({ ...line, qty: -qty });
    placed = true;
  }
  if (!placed && qty > 0) {
    out.splice(lastReturnedIndex(out) + 1, 0, returnedCartLine(row.item, qty, roundingMode));
  }
  return out;
}

function lastReturnedIndex(cart: any[]): number {
  let last = -1;
  cart.forEach((line, i) => {
    if (Number(line?.qty) < 0) last = i;
  });
  return last;
}
