/** One client's open on-account debt, as ListOnAccountClients returns it (USD cents). */
export interface OnAccountClient {
  id: number;
  name: string;
  phone?: string;
  invoiceCount: number;
  total: number;
  paid: number;
  outstanding: number;
  lastSaleAt?: string | null;
}

export interface OnAccountTotals {
  total: number;
  paid: number;
  outstanding: number;
}

/** The three numbers over the top of the page: summed from the rows shown. */
export function onAccountTotals(clients: OnAccountClient[]): OnAccountTotals {
  const totals = { total: 0, paid: 0, outstanding: 0 };
  for (const client of clients) {
    totals.total += Number(client.total) || 0;
    totals.paid += Number(client.paid) || 0;
    totals.outstanding += Number(client.outstanding) || 0;
  }
  return totals;
}

/** Rows whose name or phone contains the search text; empty text keeps every row. */
export function filterOnAccountClients(clients: OnAccountClient[], search: string): OnAccountClient[] {
  const needle = search.trim().toLowerCase();
  if (!needle) return clients;
  return clients.filter(
    (client) => client.name.toLowerCase().includes(needle) || (client.phone ?? "").replace(/\s/g, "").includes(needle.replace(/\s/g, "")),
  );
}

/** "YYYY-MM" of the local month the till is in. */
export function currentMonth(now: Date = new Date()): string {
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
}

const MONTH_NAMES = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];

/** "November 2026" for "2026-11". */
export function monthTitle(month: string): string {
  const [year, m] = month.split("-").map(Number);
  return `${MONTH_NAMES[(m ?? 1) - 1]} ${year}`;
}

/**
 * The picker's choices: only the months that still have something owed
 * (from ListOnAccountMonths, newest first), then every month ("" = all).
 */
export function monthChoices(months: string[]): { title: string; value: string }[] {
  const unique = [...new Set(months)].sort().reverse();
  return [...unique.map((value) => ({ title: monthTitle(value), value })), { title: "All months", value: "" }];
}

/** Opens on this month when it has debts, else the newest month that does, else all months. */
export function defaultMonth(months: string[], now: Date = new Date()): string {
  const current = currentMonth(now);
  if (months.includes(current)) return current;
  return [...months].sort().reverse()[0] ?? "";
}
