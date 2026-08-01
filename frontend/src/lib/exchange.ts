import type { Client } from "@/lib/client";

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
  returned: ExchangeItem;
  replacements: ExchangeItem[];
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
  const discountCents = roundDiscount(grossCents, Number(item.discount || 0), roundingMode);
  return (grossCents - discountCents) / 100;
}

export function replacementTotal(items: ExchangeItem[], roundingMode = "half_up"): number {
  return items.reduce((total, item) => total + exchangeLineTotal(item, roundingMode), 0);
}

function roundDiscount(grossCents: number, discount: number, mode: string): number {
  const value = (grossCents * Math.trunc(discount)) / 100;
  if (mode === "down") return Math.floor(value);
  if (mode === "up") return Math.ceil(value);
  return Math.floor(value + 0.5);
}
