-- Deals, mirrored from kashi (its 000201): "Buy 2 get 1 free", "3 for $50",
-- "Buy 3 or more, 20% off all". They arrive with the discount lists whose
-- rows name them, and are known here by uid. See deals.go for the pricing.
CREATE TABLE deals (
    uid text primary key,
    name text not null,
    kind text not null,
    buy_qty integer not null,
    get_qty integer not null default 0,
    percent integer not null default 0,
    bundle_price integer not null default 0
);

-- The deal a list row puts the variant on; '' for a plain discount row. A
-- deal row carries discount 0: the deal replaces the discount.
ALTER TABLE discount_list_items ADD COLUMN deal_uid text not null default '';

-- A deal's terms as they were when the invoice was made. An exchange or
-- return of a deal unit is judged by these, never by the deal as it is now,
-- which kashi may have changed or ended since.
CREATE TABLE invoice_deals (
    invoice_id bigint not null references invoices(id) on delete cascade,
    deal_uid text not null,
    name text not null,
    kind text not null,
    buy_qty integer not null,
    get_qty integer not null default 0,
    percent integer not null default 0,
    bundle_price integer not null default 0,
    primary key (invoice_id, deal_uid)
);

-- A line sold on a deal: deal_uid names it (one of the invoice's
-- invoice_deals), and deal_discount is what the deal took off the whole line,
-- in cents, so line_total = unit_price * quantity - deal_discount. Lines are
-- split so every unit of one line has the same deal discount: a free unit is
-- its own line. unit_price stays the shelf price, which is what a free unit
-- can be swapped for.
--
-- On an exchange's replacement lines deal_uid means "counts towards the
-- sale's deal" (see deal_exchange.go); deal_discount is 0 there. On a
-- returned (negative) line both are copied from the sold line.
ALTER TABLE invoice_products ADD COLUMN deal_uid text not null default '';
ALTER TABLE invoice_products ADD COLUMN deal_discount integer not null default 0;

-- What an exchange or return did to a sale's deal: the swap credit of free
-- units that went unused (forfeited) and the deal discount charged back
-- because what the customer kept no longer earns it (clawback). Both in
-- cents; each later exchange of the same sale reads them back.
CREATE TABLE exchange_deal_adjustments (
    invoice_id bigint not null references invoices(id) on delete cascade,
    sales_invoice_id bigint not null references invoices(id),
    deal_uid text not null,
    forfeited integer not null default 0,
    clawback integer not null default 0,
    primary key (invoice_id, deal_uid)
);
CREATE INDEX idx_exchange_deal_adjustments_sale ON exchange_deal_adjustments(sales_invoice_id);

-- A deal's terms price sales, so they move the pricing version too (000127).
CREATE TRIGGER pricing_version_deals_ins AFTER INSERT ON deals BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_deals_upd AFTER UPDATE ON deals BEGIN UPDATE pricing_version SET version = version + 1; END;
CREATE TRIGGER pricing_version_deals_del AFTER DELETE ON deals BEGIN UPDATE pricing_version SET version = version + 1; END;
