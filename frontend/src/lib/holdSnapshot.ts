// An invoice on hold, as home.vue saves it (HoldInvoice keeps the JSON as
// sent) and restores it. Whatever was held comes back as it was: every line
// with its prices, per-unit prices, discounts, toggles, notes and central's
// marks, the sale's ref (central's live-cart overrides of the sale go by it,
// and ResumeHoldInvoice puts them back) and its unknown-item picture session
// (the pictures already taken stay with the sale). An exchange also keeps the
// sale it returns items from, so it comes back as the same exchange: its
// returned lines (negative, locked, at their sold price), its replacements,
// and the sale to price them against.

export interface HoldExchange {
  originalInvoiceId: number;
  originalInvoiceCode?: string;
}

export interface HoldSnapshot {
  version: 1;
  items: any[];
  client?: any;
  salesperson?: any;
  calculator?: any;
  notes?: string;
  // The sale's ref for central's live cart (home.vue saleRef).
  saleRef?: string;
  // The unknown-item pictures session opened for this cart.
  unknownClientRef?: string;
  // Only on a held exchange.
  exchange?: HoldExchange;
}

export function buildHoldSnapshot(parts: {
  items: any[];
  client?: any;
  salesperson?: any;
  calculator?: any;
  notes?: string;
  saleRef?: string;
  unknownClientRef?: string;
  exchange?: HoldExchange | null;
}): HoldSnapshot {
  const snapshot: HoldSnapshot = {
    version: 1,
    // Copies, so a later edit of the cart can't reach into the hold.
    items: parts.items.map((item) => copy(item)),
    client: parts.client,
    salesperson: parts.salesperson,
    calculator: parts.calculator,
    notes: parts.notes,
  };
  if (parts.saleRef) snapshot.saleRef = parts.saleRef;
  if (parts.unknownClientRef) snapshot.unknownClientRef = parts.unknownClientRef;
  const exchange = parts.exchange;
  if (exchange && exchange.originalInvoiceId > 0) {
    snapshot.exchange = {
      originalInvoiceId: exchange.originalInvoiceId,
      ...(exchange.originalInvoiceCode
        ? { originalInvoiceCode: exchange.originalInvoiceCode }
        : {}),
    };
  }
  return snapshot;
}

function copy<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

export type HoldRestore =
  { mode: "exchange"; items: any[]; exchange: HoldExchange } | { mode: "sale"; items: any[] };

// How a hold comes back: line for line, as it was held. Both a sale and an
// exchange are put back on the cart as they are, never re-added through
// addItem, which would price them again from the lists and lose a custom
// price, a per-unit price, a custom discount or central's price. A returned
// line can only belong to an exchange, so a sale never brings one back (a
// hold from before exchanges could be held, or a damaged one). An exchange
// hold that has lost its returned lines is just a sale.
export function holdRestore(hold: any): HoldRestore {
  const items: any[] = Array.isArray(hold?.items) ? hold.items : [];
  const id = Number(hold?.exchange?.originalInvoiceId ?? 0);
  const hasReturns = items.some((item) => Number(item?.qty) < 0);
  if (id > 0 && Number.isInteger(id) && hasReturns) {
    const code = hold.exchange.originalInvoiceCode;
    return {
      mode: "exchange",
      items: items.map((item) => copy(item)),
      exchange: { originalInvoiceId: id, ...(code ? { originalInvoiceCode: String(code) } : {}) },
    };
  }
  return {
    mode: "sale",
    items: items.filter((item) => item && !(Number(item.qty) < 0)).map(heldSaleLine),
  };
}

// A held sale line as the cart keeps it. A line the screen held already is
// complete and comes back unchanged; one from an older hold (no per-unit
// prices, no list price) is completed the way addItem builds a line, from
// its own price and discount, never the lists'.
export function heldSaleLine(item: any): any {
  const line = copy(item);
  const qty = Math.max(1, Math.trunc(Number(line.qty) || 1));
  const price = Number(line.price) || 0;
  const discount = Number(line.discount) || 0;
  line.qty = qty;
  line.price = price;
  line.discount = discount;
  if (!(Array.isArray(line.unitPrices) && line.unitPrices.length === qty)) {
    line.unitPrices = Array(qty).fill(price);
  }
  if (!(Array.isArray(line.unitDiscounts) && line.unitDiscounts.length === qty)) {
    line.unitDiscounts = Array(qty).fill(discount);
  }
  line.originalPrice ??= price;
  line.originalDiscount ??= discount;
  line.note ??= "";
  return line;
}

// Everything the sale screen puts back from a hold. The same sale again:
// its ref (central's overrides of it go by it) and its unknown-item picture
// session, so the pictures already taken stay with it. A hold from before
// they were kept gets a fresh ref and no session.
export interface HeldCart {
  restore: HoldRestore;
  saleRef: string;
  unknownClientRef: string;
  client: any | null;
  salesperson: any | null;
  notes: string;
  calculator: any;
}

export function heldCart(hold: any, freshSaleRef: () => string): HeldCart {
  const saleRef = typeof hold?.saleRef === "string" && hold.saleRef ? hold.saleRef : freshSaleRef();
  return {
    restore: holdRestore(hold),
    saleRef,
    unknownClientRef: typeof hold?.unknownClientRef === "string" ? hold.unknownClientRef : "",
    client: hold?.client?.id ? hold.client : null,
    salesperson: hold?.salesperson ?? null,
    notes: typeof hold?.notes === "string" ? hold.notes : "",
    calculator: hold?.calculator,
  };
}

// The hold list's title: "#12 — 3 items", or for an exchange
// "#12 — Exchange of SA/CT/2026/00012 · 2 returned, 1 new".
export function holdTitle(hold: any): string {
  const id = hold?.id ?? "";
  const items: any[] = Array.isArray(hold?.items) ? hold.items : [];
  const restore = holdRestore(hold);
  if (restore.mode === "exchange") {
    const returned = items.filter((i) => Number(i?.qty) < 0).length;
    const added = items.length - returned;
    const of = restore.exchange.originalInvoiceCode
      ? ` of ${restore.exchange.originalInvoiceCode}`
      : "";
    return `#${id} — Exchange${of} · ${returned} returned, ${added} new`;
  }
  const count = Number(hold?.itemCount ?? items.length);
  return `#${id} — ${count} ${count === 1 ? "item" : "items"}`;
}

// How the hold list marks a hold: a sale in blue with a receipt, an
// exchange in the exchange's own orange (the Exchange checkout button's)
// with the swap icon, so the two can be told apart at a glance.
export interface HoldKind {
  kind: "sale" | "exchange";
  label: string;
  icon: string;
  color: string;
}

export function holdKind(hold: any): HoldKind {
  return holdRestore(hold).mode === "exchange"
    ? { kind: "exchange", label: "Exchange", icon: "mdi-swap-horizontal", color: "orange-darken-3" }
    : { kind: "sale", label: "Sale", icon: "mdi-receipt-text-outline", color: "primary" };
}
