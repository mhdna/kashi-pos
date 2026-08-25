-- Salespersons become a synced-down-from-kashi roster (see branch_targets'
-- 000076 migration comment for the remote_id pattern). full_name/is_active
-- are now kashi-owned; attendance_user_id stays local-only (per-branch
-- Akuvox device pairing, kashi has no opinion on it).
-- sqlite can't add a UNIQUE column via ALTER TABLE -- add it plain, then
-- enforce uniqueness with a separate index instead.
alter table salespersons add column remote_id integer;
alter table salespersons add column is_active integer not null default 1;
create unique index if not exists idx_salespersons_remote_id on salespersons (remote_id);
