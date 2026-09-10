-- Buffers the single photo of a printed close-shift receipt that the cashier
-- uploads (via the QR-code flow served by shift_upload_server.go) right
-- after a shift closes. The photo lands here first -- on the POS's own disk
-- -- so the capture works with no internet at all; a background pass in
-- runSyncTick then keeps POSTing it to kashi
-- (/branch/shift_receipt_images) every tick, across app restarts, until
-- kashi confirms receipt. That "keep trying forever" contract is why there
-- is no terminal 'failed' state here, unlike sync_outbox.
--
-- One row per shift close. token is the one-time secret embedded in the QR
-- URL. captured_at NULL means the phone hasn't finished yet; synced_at NULL
-- means kashi hasn't confirmed yet; a non-empty skipped_reason means a
-- manager overrode the capture at close time (the shift still closed).
create table if not exists shift_receipt_uploads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    shift_id integer not null,
    token text not null unique,
    expires_at text not null,

    file_path text not null default '',
    content_type text not null default '',
    size_bytes integer not null default 0,

    captured_at text,
    synced_at text,

    attempt_count integer not null default 0,
    next_attempt_at text not null default (datetime('now')),
    skipped_reason text not null default '',

    created_at text not null default (datetime('now'))
);

create index if not exists idx_shift_receipt_uploads_pending
    on shift_receipt_uploads (synced_at, next_attempt_at);
