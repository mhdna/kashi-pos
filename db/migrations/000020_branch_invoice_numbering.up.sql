ALTER TABLE settings ADD COLUMN branch_code text NOT NULL DEFAULT 'CN';

CREATE TABLE invoice_indexes_new (
    year integer NOT NULL,
    last_index bigint NOT NULL,
    type text NOT NULL CHECK (type IN ('sales', 'return')),
    PRIMARY KEY (year, type)
);

INSERT INTO invoice_indexes_new (year, last_index, type)
SELECT year, MAX(last_index), type
FROM invoice_indexes
GROUP BY year, type;

DROP TABLE invoice_indexes;
ALTER TABLE invoice_indexes_new RENAME TO invoice_indexes;
