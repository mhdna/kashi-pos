-- The LBP total the till showed and printed for an invoice, and the LBP rate
-- (LBP per USD) it used, stamped at checkout and synced to kashi, so its
-- reports carry what was actually sold in LBP rather than re-deriving it
-- later at whatever rate is current then. NULL on invoices from before this
-- (and when no LBP currency is configured).
ALTER TABLE invoices ADD COLUMN lbp_total INTEGER;
ALTER TABLE invoices ADD COLUMN lbp_rate INTEGER;
