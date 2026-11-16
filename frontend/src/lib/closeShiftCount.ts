// The close-shift count: what autofill puts in each field and when the
// count can be submitted.
//
// LBP, Visa and Whish can be negative. When more LBP went out as change
// than came in (or more was refunded to a card than was paid by it) the
// drawer is counted as it stands, minus sign and all, so the shift closes
// with no difference instead of moving the shortfall into Cash $$. USD is
// counted in bills, so a bill count is never negative.

export const usdDenominations = [100, 50, 20, 10, 5, 1] as const;

export type ExpectedTotals = {
  expectedLbp?: number;
  expectedVisa?: number;
  expectedWhish?: number;
  usdBillCounts?: Record<string, number>;
};

export type CountFields = {
  lbp: string;
  visa: string; // dollars, two decimals
  whish: string;
  bills: Record<number, string>;
};

/** The fields autofill fills in from the expected totals, negatives kept. */
export function autofillCounts(totals: ExpectedTotals): CountFields {
  const cents = (v: unknown) => Math.round(Number(v) || 0);
  const bills: Record<number, string> = {};
  for (const denomination of usdDenominations) {
    const count = Number(totals.usdBillCounts?.[String(denomination)] ?? 0);
    bills[denomination] = count > 0 ? String(count) : "";
  }
  return {
    lbp: String(cents(totals.expectedLbp)),
    visa: (cents(totals.expectedVisa) / 100).toFixed(2),
    whish: (cents(totals.expectedWhish) / 100).toFixed(2),
    bills,
  };
}

/** Dollars typed in a field as cents. */
export function fieldCents(value: string): number {
  return Math.round(Number(value || 0) * 100);
}

/** Whether the count can be submitted (a closing person is checked apart). */
export function countValid(fields: CountFields): boolean {
  const lbp = Number(fields.lbp);
  if (fields.lbp === "" || !Number.isFinite(lbp) || !Number.isInteger(lbp)) return false;
  if (!Number.isFinite(fieldCents(fields.visa)) || !Number.isFinite(fieldCents(fields.whish))) return false;
  return usdDenominations.every((denomination) => {
    const raw = fields.bills[denomination] ?? "";
    if (raw === "") return true;
    const count = Number(raw);
    return Number.isInteger(count) && count >= 0;
  });
}
