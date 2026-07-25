CREATE TABLE IF NOT EXISTS invoice_indexes_old (
    year integer not null,
    cashbox_id bigint not null references cashbox_accounts(id),
    last_index bigint not null,
    type text not null check (type in ('sales', 'return')),
    primary key (cashbox_id, year)
);
INSERT INTO invoice_indexes_old SELECT * FROM invoice_indexes;
DROP TABLE invoice_indexes;
ALTER TABLE invoice_indexes_old RENAME TO invoice_indexes;
