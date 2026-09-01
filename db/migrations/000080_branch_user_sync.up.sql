-- POS users become a synced-down-from-kashi roster (see salespersons'
-- 000078 migration for the remote_id pattern). username / role_id / active
-- / attendance_user_id are kashi-owned for a synced user; password_hash is
-- never sent by kashi (bcrypt vs sha256, and a PIN has little value at
-- rest) -- it arrives via a set_branch_user_pin command, or a local
-- admin-only reset.
-- sqlite can't add a UNIQUE column via ALTER TABLE -- add it plain, then a
-- separate unique index.
alter table users add column remote_id integer;
create unique index if not exists idx_users_remote_id on users (remote_id);
