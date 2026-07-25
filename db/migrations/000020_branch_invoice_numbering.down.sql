CREATE TABLE invoice_indexes_old (
    year integer NOT NULL,
    cashbox_id bigint NOT NULL REFERENCES cashbox_accounts(id),
    last_index bigint NOT NULL,
    type text NOT NULL CHECK (type IN ('sales', 'return')),
    PRIMARY KEY (cashbox_id, year, type)
);

INSERT INTO invoice_indexes_old (year, cashbox_id, last_index, type)
SELECT ii.year, ca.id, ii.last_index, ii.type
FROM invoice_indexes ii
JOIN cashbox_accounts ca ON ca.id = (SELECT MIN(id) FROM cashbox_accounts);

DROP TABLE invoice_indexes;
ALTER TABLE invoice_indexes_old RENAME TO invoice_indexes;
ALTER TABLE settings DROP COLUMN branch_code;
