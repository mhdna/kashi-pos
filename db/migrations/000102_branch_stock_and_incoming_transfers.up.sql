-- Cross-branch stock visibility: every OTHER branch's on-hand quantity and
-- selling price for a variant, synced down from kashi's branch_stock feed
-- (see ListBranchStockForSync in the kashi repo). Purely a mirror for
-- browsing/search on the Inventory page -- this till never writes to it.
CREATE TABLE IF NOT EXISTS branch_stock (
    branch_code text NOT NULL,
    variant_uid text NOT NULL,
    barcode text NOT NULL DEFAULT '',
    quantity bigint NOT NULL DEFAULT 0,
    price_cents bigint NOT NULL DEFAULT 0,
    updated_at text NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (branch_code, variant_uid)
);

-- Two-sided transfer confirmation: transfer_requests/transfer_request_items
-- already exist (000101) for this branch's own outgoing proposals.
-- direction distinguishes those from transfers addressed TO this branch
-- (which this till never created, so it has no client_ref of its own --
-- central_id, kashi's own transfers.id, is the key instead).
ALTER TABLE transfer_requests ADD COLUMN direction text NOT NULL DEFAULT 'outgoing';
ALTER TABLE transfer_requests ADD COLUMN central_id INTEGER;
ALTER TABLE transfer_requests ADD COLUMN sender_branch_name text NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_transfer_requests_central_id
    ON transfer_requests(central_id) WHERE central_id IS NOT NULL;

-- This branch's own counted quantity for one line, mirroring the central
-- transfer_items.received_quantity column so the confirm payload this
-- till sends up is a straight field copy.
ALTER TABLE transfer_request_items ADD COLUMN received_quantity INTEGER;
