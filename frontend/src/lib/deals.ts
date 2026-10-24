// A priced line from the backend's quote (invoice_quote.go LineQuote). A
// cart line on a deal can come back as several of these -- its free units
// apart -- so a cart line's deal is the sum of the quote lines with its
// barcode (or code, for a line without one).
export interface DealLine {
  code: string;
  barcode: string;
  qty: number;
  deal?: string;
  dealDiscountCents?: number;
  free?: boolean;
}

export interface CartLineDeal {
  name: string;
  discountCents: number;
  freeQty: number;
}

export function dealsByLine(lines: DealLine[]): Map<string, CartLineDeal> {
  const out = new Map<string, CartLineDeal>();
  for (const line of lines) {
    if (!line.deal) continue;
    const key = line.barcode || line.code;
    const entry = out.get(key) ?? { name: line.deal, discountCents: 0, freeQty: 0 };
    entry.discountCents += line.dealDiscountCents ?? 0;
    if (line.free) entry.freeQty += line.qty;
    out.set(key, entry);
  }
  return out;
}
