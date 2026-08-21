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
