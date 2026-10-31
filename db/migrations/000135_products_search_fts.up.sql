-- A trigram index over what the sale screen's product search matches, so a
-- search no longer scans every product with LIKE '%x%'. Additive: nothing
-- existing changes, and an older build running on this database keeps the
-- index current through the triggers (it just never reads it).
--
--   all_text  = name || ' ' || code || ' ' || barcode   (default search)
--   code_text = code || ' ' || barcode                   (code-only search)
--
-- rowid is products.id. FTS5's trigram tokenizer answers LIKE '%term%' from
-- the index for terms of 3+ characters; shorter terms still use plain LIKE
-- (products_search.go).
CREATE VIRTUAL TABLE products_search USING fts5(
    all_text,
    code_text,
    tokenize = 'trigram'
);

INSERT INTO products_search (rowid, all_text, code_text)
SELECT id, name || ' ' || code || ' ' || barcode, code || ' ' || barcode FROM products;

CREATE TRIGGER products_search_ins AFTER INSERT ON products BEGIN
    INSERT INTO products_search (rowid, all_text, code_text)
    VALUES (new.id, new.name || ' ' || new.code || ' ' || new.barcode, new.code || ' ' || new.barcode);
END;

CREATE TRIGGER products_search_upd AFTER UPDATE OF id, name, code, barcode ON products BEGIN
    DELETE FROM products_search WHERE rowid = old.id;
    INSERT INTO products_search (rowid, all_text, code_text)
    VALUES (new.id, new.name || ' ' || new.code || ' ' || new.barcode, new.code || ' ' || new.barcode);
END;

CREATE TRIGGER products_search_del AFTER DELETE ON products BEGIN
    DELETE FROM products_search WHERE rowid = old.id;
END;
