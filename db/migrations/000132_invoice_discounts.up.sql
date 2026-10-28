-- Invoice discounts, pulled from kashi (its 000212): "invoices of $150 or
-- more get 15% off". Only the rules that apply to this branch are kept; one
-- that stops applying is deleted. Dates are RFC3339, '' for an open end.
-- See invoice_discounts.go for how a sale gets one.
CREATE TABLE invoice_discounts (
    uid text primary key,
    name text not null,
    min_total integer not null,
    kind text not null,
    percent integer not null default 0,
    amount integer not null default 0,
    valid_from text not null default '',
    valid_to text not null default ''
);

-- What a sale's invoice discount took off, and the rule's name; each line's
-- share of it. line_total already has the share off, so a return refunds
-- what the line was paid.
ALTER TABLE invoices ADD COLUMN invoice_discount integer not null default 0;
ALTER TABLE invoices ADD COLUMN invoice_discount_name text not null default '';
ALTER TABLE invoice_products ADD COLUMN invoice_discount integer not null default 0;
