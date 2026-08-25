-- Tracks how far this branch has caught up on each pulled entity (see
-- GET /branch/sync/changes in the kashi repo). One row per entity name;
-- missing means "never synced, pull everything from the beginning" — that
-- case needs no special handling, it's just an empty `since`.
create table if not exists sync_cursors (
    entity text primary key,
    cursor text not null default ''
);
