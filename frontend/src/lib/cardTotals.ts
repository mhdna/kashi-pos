// What the calculator's total card says besides its big number.
//
// The big number is what is left to pay (or the change) once something has been
// paid, so the invoice's own total would be lost: this gives the label that keeps
// it on the card, only while the big number is something else. And the discount
// label: everything taken off the sale in all.

import { formatUsd } from "./format";

export interface CardTotalsInput {
  quoteFailed: boolean;
  exchangeMode: boolean;
  hasItems: boolean;
  settlementInvoiceId?: number;
  totalCents: number;
  // paid - total, as the card computes it: negative while something is owed.
  diffCents: number;
}

/** "Total $250.00" while the card's big number is not the total; "" otherwise. */
export function invoiceTotalLabel(input: CardTotalsInput): string {
  if (input.quoteFailed || input.exchangeMode) return "";
  if (!input.hasItems && !input.settlementInvoiceId) return "";
  if (input.totalCents <= 0) return "";
  // Nothing paid yet: the big number already is the total.
  if (Math.abs(input.diffCents) === input.totalCents) return "";
  return `Total ${formatUsd(input.totalCents / 100)}`;
}

/** "Discount −$42.00" for everything taken off the sale; "" for none. */
export function discountLabel(discountCents: number | undefined): string {
  const cents = discountCents ?? 0;
  if (!(cents > 0)) return "";
  return `Discount −${formatUsd(cents / 100)}`;
}
