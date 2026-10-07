// "On Account" and "On Coupon" are internal bookkeeping cashbox accounts,
// not selectable tenders - every place that lists real payment methods
// (the calculator, Settings' payment-methods table) filters them out
// through this single check instead of re-deriving the special-name list.
export function isSpecialPaymentMethod(name: unknown): boolean {
  const normalized = String(name ?? "").trim().toLowerCase();
  return normalized === "on account" || normalized === "on coupon";
}

// Physical cash drawer accounts are admin-named starting with "Cash"
// (e.g. "Cash $$", "Cash LL") to distinguish them from card/wallet methods
// (Visa, Whish, ...), which have no other machine-readable marker.
export function isCashPaymentMethod(name: unknown): boolean {
  return String(name ?? "").trim().toLowerCase().startsWith("cash");
}
