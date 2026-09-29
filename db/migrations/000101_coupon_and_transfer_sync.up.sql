-- Coupons become centrally managed: kashi (see its 000099 migration) mints
-- coupon_types and coupons and syncs them down; this till mirrors them and
-- reports redemptions back up through the existing invoice outbox. See
-- sync_pull_coupons.go for the apply side.

-- coupon_types.id has always been a local autoincrement -- a type created
-- before this feature (or ever, locally, going forward) keeps its own id.
-- central_id is the separate identity a synced-down type carries, so the
-- pull can find "the row for central coupon_type 7" without colliding with
-- whatever a till already had at local id 7.
ALTER TABLE coupon_types ADD COLUMN central_id INTEGER;
CREATE UNIQUE INDEX IF NOT EXISTS idx_coupon_types_central_id
    ON coupon_types(central_id) WHERE central_id IS NOT NULL;

-- coupons.status has only ever been 'active'/'inactive' locally (redemption
-- sets 'inactive' + redeemed_invoice_id, see client.sql's RedeemCoupon).
-- 'redeemed' is a third, distinct value this till only ever receives from
-- kashi -- it never writes it itself -- reported when the code was spent at
-- a *different* branch. SQLite can't ALTER a CHECK constraint in place, so
-- the table is rebuilt, same approach as 000040's invoice_coupon_payments.
-- Every existing column is carried over (there is no local category_id --
-- that's a kashi-only concept, coupon categories never leave kashi, same
-- as expense/loan categories). discount_type/reason lose their NOT NULL --
-- still populated by the local one-off CreateCoupon/CreateValueCoupon
-- flow, but a coupon that arrives via central sync has neither (kashi's
-- own coupons table dropped both columns entirely -- see its 000099
-- migration -- once value_cents/type_id existed to replace them), and
-- SQLite's CHECK is satisfied vacuously by NULL either way.
CREATE TABLE coupons_new (
    code text primary key,
    status text not null check (status in ('active', 'inactive', 'redeemed')),
    discount_type text check (discount_type in ('fixed', 'percentage')),
    reason text,
    client_id bigint references clients(id),
    valid_until text,
    created_at text not null default (datetime('now')),
    type_id bigint references coupon_types(id),
    value_cents bigint,
    redeemed_invoice_id bigint references invoices(id)
);

INSERT INTO coupons_new (code, status, discount_type, reason, client_id, valid_until, created_at, type_id, value_cents, redeemed_invoice_id)
SELECT code, status, discount_type, reason, client_id, valid_until, created_at, type_id, value_cents, redeemed_invoice_id
FROM coupons;

DROP TABLE coupons;
ALTER TABLE coupons_new RENAME TO coupons;

CREATE INDEX IF NOT EXISTS idx_coupons_type ON coupons(type_id);

-- Mirror of kashi's branches -- populated by the branches down-sync feed,
-- purely to give a till a destination picker for a transfer request. See
-- ListBranchesForSync in the kashi repo.
CREATE TABLE IF NOT EXISTS synced_branches (
    code text primary key,
    name text not null,
    is_active integer not null default 1
);

-- A transfer this till proposed out of its own store inventory. Mirrors
-- kashi's transfers/transfer_items (see its 000099 migration) just enough
-- to stage, queue, and track one: status starts 'requested' and is
-- overwritten wholesale by whatever the transfer_requests down-sync last
-- reported (kashi is authoritative for status the moment it's synced).
--
-- No attachment_path here: a transfer's photo, like a close-shift
-- receipt or an expense receipt, is uploaded straight to kashi from a
-- phone that scanned a QR (see api/transfer_upload.go in the kashi
-- repo) -- it never lands on this till's own disk, so there's nothing
-- local to track beyond the transient "has it landed yet" the QR dialog
-- polls for live (branchTransferUploadStatus).
CREATE TABLE IF NOT EXISTS transfer_requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    client_ref text NOT NULL UNIQUE,
    to_branch_code text NOT NULL,
    note text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'requested',
    created_at text NOT NULL DEFAULT (datetime('now')),
    updated_at text NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS transfer_request_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    transfer_request_id bigint NOT NULL REFERENCES transfer_requests(id) ON DELETE CASCADE,
    barcode text NOT NULL,
    product_uid text NOT NULL DEFAULT '',
    quantity bigint NOT NULL
);
