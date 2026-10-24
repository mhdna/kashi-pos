// A priced line from the backend's quote (invoice_quote.go LineQuote). A
// cart line on a deal can come back as several of these -- its free units
// apart -- so a cart line's deal is the sum of the quote lines with its
// barcode (or code, for a line without one).
export interface DealLine {
  code: string;
  barcode: string;
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
}

export interface CartLineDeal {
  name: string;
  label: string;
  discountCents: number;
  freeQty: number;
}

export function dealsByLine(lines: DealLine[]): Map<string, CartLineDeal> {
  const out = new Map<string, CartLineDeal>();
  for (const line of lines) {
    if (!line.deal) continue;
    const key = line.barcode || line.code;
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
    if (line.offDealLabel) out.set(line.barcode || line.code, line.offDealLabel);
  }
  return out;
}
