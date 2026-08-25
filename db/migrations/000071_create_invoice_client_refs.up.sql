-- Maps a local invoice id to the stable client_ref used to identify it to
-- kashi. Needed because a return/exchange must reference its original sale
-- by client_ref even if that sale was never queued for sync itself (e.g.
-- sync was turned on after the sale happened) — this table lets us
-- get-or-create a canonical ref for any invoice id on demand, so the same
-- invoice always resolves to the same client_ref no matter which code path
-- asks for it first.
create table if not exists invoice_client_refs (
    invoice_id integer primary key references invoices(id),
    client_ref text not null unique
);
