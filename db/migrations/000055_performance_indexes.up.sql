-- invoices has no secondary index at all today, despite shift_id being the
-- dominant filter across the balance page, shift report, and
-- income-by-shift queries.
CREATE INDEX IF NOT EXISTS idx_invoices_shift_id ON invoices(shift_id);

-- date(created_at,'localtime') is filtered constantly (today's income,
-- daily totals, "on account is from today" checks), but SQLite refuses to
-- index that expression: 'localtime' depends on session/system timezone
-- context, which SQLite treats as non-deterministic and rejects at write
-- time ("non-deterministic use of date() in an index"), even though a
-- read-only EXPLAIN QUERY PLAN against it looks fine. Left unindexed - a
-- generated column plus index would need changing what created_at itself
-- stores (it's already written via datetime('now','localtime')), which is
-- a larger, riskier change than this pass should make.

-- invoice_payment_entries is filtered by invoice_id (ListInvoicePaymentEntries)
-- with no supporting index.
CREATE INDEX IF NOT EXISTS idx_invoice_payment_entries_invoice ON invoice_payment_entries(invoice_id);

-- GetSalesInvoiceByCode does a case-insensitive lookup.
CREATE INDEX IF NOT EXISTS idx_invoices_code_lower ON invoices(lower(invoice_code));
