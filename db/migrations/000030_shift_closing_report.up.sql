ALTER TABLE shifts ADD COLUMN closed_by_salesperson_id bigint NOT NULL DEFAULT 0;
ALTER TABLE shifts ADD COLUMN counted_usd bigint NOT NULL DEFAULT 0;
ALTER TABLE shifts ADD COLUMN counted_lbp bigint NOT NULL DEFAULT 0;
ALTER TABLE shifts ADD COLUMN counted_usd_breakdown text NOT NULL DEFAULT '{}';
