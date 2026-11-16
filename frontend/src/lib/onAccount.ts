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
