// A priced line from the backend's quote (invoice_quote.go LineQuote). A
// cart line on a deal can come back as several of these -- its free units
// apart -- so a cart line's deal is the sum of the quote lines with its
// barcode (or code, for a line without one) and sale option.
export interface DealLine {
  code: string;
  barcode: string;
  // The same product sold as Summer and as Winter is two cart lines.
  saleOption?: string;
  qty: number;
  deal?: string;
  // The deal's terms in short, e.g. "B2G1" (deals.go dealTerms.label).
  dealLabel?: string;
  dealDiscountCents?: number;
  free?: boolean;
  // What the line gave up when the salesperson turned its discount
  // (discount_toggle) or its deal (deal_toggle) off: a percent, or the deal
  // in short ("B2G1").
  offDiscount?: number;
  offDealLabel?: string;
  // The cart line's ref as sent (an unknown item's unknownRef): every
  // unknown item is the same product, so this is what tells them apart.
  lineRef?: string;
  // An unknown item line's deal choice in force (UnknownDealChoice); absent
  // when the till has no "Buy 2 get 1" to offer it.
  unknownDeal?: string;
}

// What a salesperson picks for an unknown item (backend unknown_item_deal.go):
// the shop's "Buy 2 get 1" with 30% or 20% off a unit outside a set, or no
// deal at all.
export type UnknownDealChoice = "b2g1_30" | "b2g1_20" | "none";

export const unknownDealChoices: { value: UnknownDealChoice; title: string; subtitle: string }[] = [
  { value: "b2g1_30", title: "B2G1 · else 30%", subtitle: "Buy 2 get 1 free, 30% off when there's no set" },
  { value: "b2g1_20", title: "B2G1 · else 20%", subtitle: "Buy 2 get 1 free, 20% off when there's no set" },
  { value: "none", title: "No deal", subtitle: "Full price, no discount" },
];

export interface CartLineDeal {
  name: string;
  label: string;
  discountCents: number;
  freeQty: number;
}

// The key a cart line and its quote lines share.
export function dealLineKey(line: {
  code?: string;
  barcode?: string;
  saleOption?: string;
  lineRef?: string;
  unknownRef?: string;
}): string {
  const ref = line.lineRef || line.unknownRef;
  if (ref) return `ref\u0000${ref}`;
  return `${line.barcode || line.code || ""}\u0000${line.saleOption ?? ""}`;
}

export function dealsByLine(lines: DealLine[]): Map<string, CartLineDeal> {
  const out = new Map<string, CartLineDeal>();
  for (const line of lines) {
    if (!line.deal) continue;
    const key = dealLineKey(line);
    const entry = out.get(key) ?? {
      name: line.deal,
      label: line.dealLabel || line.deal,
      discountCents: 0,
      freeQty: 0,
    };
    entry.discountCents += line.dealDiscountCents ?? 0;
    if (line.free) entry.freeQty += line.qty;
    out.set(key, entry);
  }
  return out;
}

// The deal each cart line gave up by having its deal turned off, keyed
// as dealsByLine is.
export function offDealsByLine(lines: DealLine[]): Map<string, string> {
  const out = new Map<string, string>();
  for (const line of lines) {
    if (line.offDealLabel) out.set(dealLineKey(line), line.offDealLabel);
  }
  return out;
}

// Each unknown item line's deal choice in force, keyed as dealsByLine is.
export function unknownDealsByLine(lines: DealLine[]): Map<string, UnknownDealChoice> {
  const out = new Map<string, UnknownDealChoice>();
  for (const line of lines) {
    const choice = unknownDealChoices.find((c) => c.value === line.unknownDeal)?.value;
    if (choice) out.set(dealLineKey(line), choice);
  }
  return out;
}
