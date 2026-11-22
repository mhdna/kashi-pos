-- A sale central opened for exchange past the branch's exchange window
-- (command open_invoice_exchange): it can be exchanged until open_until
-- (UTC RFC3339). A newer opening replaces the old one.
CREATE TABLE IF NOT EXISTS exchange_openings (
  invoice_id integer PRIMARY KEY REFERENCES invoices(id) ON DELETE CASCADE,
  open_until text NOT NULL,
  opened_by text NOT NULL DEFAULT '',
  reason text NOT NULL DEFAULT '',
  created_at text NOT NULL DEFAULT (datetime('now'))
);
