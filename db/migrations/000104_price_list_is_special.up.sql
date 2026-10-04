-- Mirrors kashi's 000105: is_special is an explicit tag an admin sets
-- centrally on a list, not an inference from "some list priced this
-- product". See products.go's productMap, which used to badge every priced
-- product as "special" and is being fixed to only badge one whose winning
-- list carries this flag.
ALTER TABLE price_lists ADD COLUMN is_special integer NOT NULL DEFAULT 0;
ALTER TABLE discount_lists ADD COLUMN is_special integer NOT NULL DEFAULT 0;
