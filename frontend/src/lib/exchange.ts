import type { Client } from "@/lib/client";
import { roundDiscountCents, normalizeRoundingMode } from "@/lib/money";

export interface ExchangeItem {
  code: string;
  barcode: string;
  name: string;
  category?: string;
  color?: string;
  size?: string;
  price: number;
  soldUnitPrice?: number;
  soldUnitDiscount?: number;
  discount: number;
  available?: number;
  qty?: number;
  salespersonId?: number;
  salespersonName?: string;
  // Units sold on a deal (deal_exchange.go): its name, and how many of the
  // available units were free -- a free one swaps for anything up to its
  // price, and is the one taken back first.
  deal?: string;
  freeQty?: number;
}

export interface ExchangeInvoice {
  id: number;
  invoiceCode: string;
  client: Client;
  items: ExchangeItem[];
}

export interface ExchangeSelection {
  originalInvoiceId: number;
  client: Client;
  returned: ExchangeItem[];
}

// A product's free and paid units of a deal are separate sold lines at the
// same price; on the shelf they are alike, so the dialog shows them as one
// item and the backend takes the free one back first.
export function mergeDealUnits(items: (ExchangeItem & { free?: boolean })[]): ExchangeItem[] {
  const out: ExchangeItem[] = [];
  const at = new Map<string, number>();
  for (const { free, ...item } of items) {
    const key = `${normalizeExchangeCode(item.code)}|${item.barcode}|${item.price}|${item.discount}`;
    const freeQty = free ? Number(item.available || 0) : 0;
    const index = at.get(key);
    if (index === undefined) {
      at.set(key, out.length);
      out.push({ ...item, freeQty });
      continue;
    }
    const merged = out[index]!;
    merged.available = Number(merged.available || 0) + Number(item.available || 0);
    merged.freeQty = Number(merged.freeQty || 0) + freeQty;
    merged.deal ||= item.deal;
  }
  return out;
}

export function normalizeExchangeCode(value: string): string {
  return value.trim().toLowerCase();
}

export function effectiveExchangePrice(item: ExchangeItem, roundingMode = "half_up"): number {
  return exchangeLineTotal({ ...item, qty: 1 }, roundingMode);
}

export function exchangeLineTotal(item: ExchangeItem, roundingMode = "half_up"): number {
  const qty = Number(item.qty || 1);
  const grossCents = Math.round(Number(item.price) * 100) * qty;
  const discountCents = roundDiscountCents(grossCents, Number(item.discount || 0), normalizeRoundingMode(roundingMode));
  return (grossCents - discountCents) / 100;
}
