package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// PriceStatus reports products the POS would show at $0: on list-priced
// databases (POS 000097+) a product sells only at a price from an active
// price list.
type PriceStatus struct {
	ListPricing bool   `json:"listPricing"`
	Connected   bool   `json:"connected"`
	PriceList   string `json:"priceList"`
	Missing     int    `json:"missing"`   // no price in any active list
	Fixable     int    `json:"fixable"`   // ...of which have their own price > 0
	NoPrice     int    `json:"noPrice"`   // ...of which have no own price either
	Discounts   int    `json:"discounts"` // own discount not in any active discount list
}

// FixResult is what fixPrices changed.
type FixResult struct {
	PriceList       string `json:"priceList"`
	CreatedList     bool   `json:"createdList"`
	Priced          int64  `json:"priced"`
	Discounted      int64  `json:"discounted"`
	CreatedDiscList bool   `json:"createdDiscountList"`
}

const (
	noActivePrice = `NOT EXISTS (SELECT 1 FROM price_list_items i JOIN price_lists l ON l.id = i.price_list_id
		WHERE l.is_active = 1 AND i.product_uid = p.uid)`
	noActiveDiscount = `NOT EXISTS (SELECT 1 FROM discount_list_items i JOIN discount_lists l ON l.id = i.discount_list_id
		WHERE l.is_active = 1 AND i.product_uid = p.uid)`
	newUID = `lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' ||
		substr(lower(hex(randomblob(2))), 2) || '-' ||
		substr('89ab', 1 + (abs(random()) % 4), 1) || substr(lower(hex(randomblob(2))), 2) || '-' ||
		lower(hex(randomblob(6)))`
)

func priceStatus(dbPath string) (PriceStatus, error) {
	k, err := openKashiDB(dbPath)
	if err != nil {
		return PriceStatus{}, err
	}
	defer k.conn.Close()
	st := PriceStatus{ListPricing: k.listPricing, PriceList: k.priceListName, Connected: k.connected()}
	if !k.listPricing {
		return st, nil
	}
	err = k.conn.QueryRow(`SELECT count(*), COALESCE(sum(p.price > 0), 0), COALESCE(sum(p.price <= 0), 0)
		FROM products p WHERE p.uid IS NOT NULL AND `+noActivePrice).Scan(&st.Missing, &st.Fixable, &st.NoPrice)
	if err != nil {
		return st, err
	}
	err = k.conn.QueryRow(`SELECT count(*) FROM products p
		WHERE p.uid IS NOT NULL AND p.discount BETWEEN 1 AND 100 AND ` + noActiveDiscount).Scan(&st.Discounts)
	return st, err
}

// fixPrices gives every product without a list price its own price in the
// active default price list (creating that list if there is none), and does
// the same for discounts. Prices already in a list are never changed.
func fixPrices(dbPath string) (FixResult, error) {
	k, err := openKashiDB(dbPath)
	if err != nil {
		return FixResult{}, err
	}
	defer k.conn.Close()
	if !k.listPricing {
		return FixResult{}, fmt.Errorf("this database sells at each product's own price; nothing to fix")
	}
	tx, err := k.conn.Begin()
	if err != nil {
		return FixResult{}, err
	}
	defer tx.Rollback()

	var res FixResult
	priceListID, name, created, err := defaultList(tx, "price_lists")
	if err != nil {
		return res, err
	}
	res.PriceList, res.CreatedList = name, created
	r, err := tx.Exec(`INSERT OR IGNORE INTO price_list_items (price_list_id, product_uid, price)
		SELECT ?, p.uid, p.price FROM products p
		WHERE p.uid IS NOT NULL AND p.price > 0 AND `+noActivePrice, priceListID)
	if err != nil {
		return res, fmt.Errorf("add prices: %w", err)
	}
	res.Priced, _ = r.RowsAffected()

	var needDiscounts int
	if err := tx.QueryRow(`SELECT count(*) FROM products p
		WHERE p.uid IS NOT NULL AND p.discount BETWEEN 1 AND 100 AND ` + noActiveDiscount).Scan(&needDiscounts); err != nil {
		return res, err
	}
	if needDiscounts > 0 {
		discountListID, _, createdDisc, err := defaultList(tx, "discount_lists")
		if err != nil {
			return res, err
		}
		res.CreatedDiscList = createdDisc
		r, err := tx.Exec(`INSERT OR IGNORE INTO discount_list_items (discount_list_id, product_uid, discount)
			SELECT ?, p.uid, p.discount FROM products p
			WHERE p.uid IS NOT NULL AND p.discount BETWEEN 1 AND 100 AND `+noActiveDiscount, discountListID)
		if err != nil {
			return res, fmt.Errorf("add discounts: %w", err)
		}
		res.Discounted, _ = r.RowsAffected()
	}
	return res, tx.Commit()
}

// defaultList returns the active default list in table ("price_lists" or
// "discount_lists"), creating an active "Default" one if there is none.
func defaultList(tx *sql.Tx, table string) (id int64, name string, created bool, err error) {
	err = tx.QueryRow(`SELECT id, name FROM `+table+` WHERE is_default = 1 AND is_active = 1 ORDER BY id LIMIT 1`).Scan(&id, &name)
	if !errors.Is(err, sql.ErrNoRows) {
		return id, name, false, err
	}
	r, err := tx.Exec(`INSERT INTO ` + table + ` (uid, name, is_active, is_default) VALUES (` + newUID + `, 'Default', 1, 1)`)
	if err != nil {
		return 0, "", false, fmt.Errorf("create Default list: %w", err)
	}
	id, err = r.LastInsertId()
	return id, "Default", true, err
}

func (k *kashiDB) connected() bool {
	if ok, _ := k.hasColumn("settings", "central_api_url"); !ok {
		return false
	}
	var central sql.NullString
	_ = k.conn.QueryRow(`SELECT central_api_url FROM settings LIMIT 1`).Scan(&central)
	return strings.TrimSpace(central.String) != ""
}
