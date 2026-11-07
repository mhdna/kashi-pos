// An invoice on hold, as home.vue saves it (HoldInvoice keeps the JSON as
// sent) and restores it. A sale and an exchange can both be held; an
// exchange keeps the sale it returns items from, so it comes back as the
// same exchange: its returned lines (negative, locked, at their sold price)
// exactly as they were, its replacements, and the sale to price them against.

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
  // Only on a held exchange.
  exchange?: HoldExchange;
}

export function buildHoldSnapshot(parts: {
  items: any[];
  client?: any;
  salesperson?: any;
  calculator?: any;
  notes?: string;
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

// How a hold comes back. An exchange is restored as it was, line for line.
// A sale's lines are re-added one by one (addItem prices them again); a
// returned line can only belong to an exchange, so a sale never brings one
// back (a hold from before exchanges could be held, or a damaged one).
// An exchange hold that has lost its returned lines is just a sale.
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
  return { mode: "sale", items: items.filter((item) => !(Number(item?.qty) < 0)) };
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
