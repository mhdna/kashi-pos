// How a payment reads in the invoice details: an LBP account's payment in
// L.L. (the amount the cashier counted, from the backend's lbpAmount) with
// its dollars beside it; any other in dollars.

export type PaymentRow = { amount?: number; currencyCode?: string; lbpAmount?: number | null };

export function usdLabel(cents: number | undefined): string {
  const value = Number(cents) || 0;
  const sign = value < 0 ? "-" : "";
  return `${sign}$${(Math.abs(value) / 100).toFixed(2)}`;
}

export function lbpLabel(units: number): string {
  const sign = units < 0 ? "-" : "";
  return `${sign}${Math.abs(units).toLocaleString("en-US")} L.L.`;
}

// paymentLabels: the main amount, and the dollars beside it ("" for none).
export function paymentLabels(p: PaymentRow): { main: string; equivalent: string } {
  if (typeof p.lbpAmount === "number" && Number.isFinite(p.lbpAmount)) {
    return { main: lbpLabel(p.lbpAmount), equivalent: usdLabel(p.amount) };
  }
  return { main: usdLabel(p.amount), equivalent: "" };
}
