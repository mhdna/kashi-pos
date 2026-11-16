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

/** The picker's choices: this month first, then back a year, then every month ("" = all). */
export function monthChoices(now: Date = new Date(), count = 12): { title: string; value: string }[] {
  const choices: { title: string; value: string }[] = [];
  for (let i = 0; i < count; i++) {
    const d = new Date(now.getFullYear(), now.getMonth() - i, 1);
    choices.push({ title: `${MONTH_NAMES[d.getMonth()]} ${d.getFullYear()}`, value: currentMonth(d) });
  }
  choices.push({ title: "All months", value: "" });
  return choices;
}
