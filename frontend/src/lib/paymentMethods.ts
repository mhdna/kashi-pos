// "On Account" and "On Coupon" are internal bookkeeping cashbox accounts,
// not selectable tenders - every place that lists real payment methods
// (the calculator, Settings' payment-methods table) filters them out
// through this single check instead of re-deriving the special-name list.
export function isSpecialPaymentMethod(name: unknown): boolean {
  const normalized = String(name ?? "").trim().toLowerCase();
  return normalized === "on account" || normalized === "on coupon";
}
