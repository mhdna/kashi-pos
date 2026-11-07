import { ListSalespersons, SearchClients } from "../../wailsjs/go/main/App";

// The lists the invoice list's Change Client / Change Salesperson dialog
// offers, as last loaded. A dialog shows these the moment it opens and
// replaces them with the fresh copy when it arrives, so opening one never
// waits on the backend. They live for as long as the app runs.
let salespersonRoster: any[] | null = null;
let firstClients: any[] | null = null;

// The invoice list row fields the dialog reads.
export interface ChangeableInvoice {
  id: number;
  clientId?: number;
  clientName?: string;
  clientPhone?: string;
  salespersonId?: number;
  salespersonName?: string;
}

export interface SalespersonOption {
  id: number;
  fullName: string;
}

export function cachedSalespersons(): SalespersonOption[] | null {
  return salespersonRoster;
}

// Staff who can be set as a sale's salesperson: not attendance-only staff
// and not a till-only leftover once kashi manages the roster (see
// ListSalespersons in salespersons.go).
export async function loadSalespersons(): Promise<SalespersonOption[]> {
  const all = (await ListSalespersons()) ?? [];
  salespersonRoster = all.filter((s: any) => s.isSalesperson !== false && s.isCurrent !== false);
  return salespersonRoster;
}

export function cachedFirstClients(): any[] | null {
  return firstClients;
}

// Clients matching a name or phone (the first page of all of them for an
// empty search). The empty search is the list a dialog opens with, so it is
// kept.
export async function searchClients(search: string): Promise<any[]> {
  const term = search.trim();
  const result = (await SearchClients(term, 0)) ?? [];
  if (!term) firstClients = result;
  return result;
}

// Fetches both lists ahead of time so the first dialog opens full.
export function prefetchInvoiceEditOptions(): void {
  if (!salespersonRoster) void loadSalespersons().catch(() => {});
  if (!firstClients) void searchClients("").catch(() => {});
}

// For tests.
export function resetInvoiceEditOptions(): void {
  salespersonRoster = null;
  firstClients = null;
}
