// The shift income card's totals. Every IncomeRow.total is USD cents (the row's
// currency only picks how it is shown, see formatBalance), so a total is a
// plain sum of the rows on screen: it can never disagree with its lines.
// (It used to come from a separate shift-totals list that could be empty or
// stale, so the card showed Cash $243.00 above Income total $0.00.)

export interface IncomeLine {
  total: number;
  isOnAccount?: boolean;
}

const cents = (n: unknown) => Number(n) || 0;

/** What went into the drawer: every line except sales on account. */
export function collectedTotalCents(rows: IncomeLine[]): number {
  return rows.filter((r) => !r.isOnAccount).reduce((sum, r) => sum + cents(r.total), 0);
}

/** Sold on account: owed, not collected. */
export function onAccountTotalCents(rows: IncomeLine[]): number {
  return rows.filter((r) => r.isOnAccount).reduce((sum, r) => sum + cents(r.total), 0);
}

/** Collected minus what went out (expenses and loans, already in USD cents). */
export function netIncomeCents(collected: number, spentCents: number): number {
  return collected - spentCents;
}
