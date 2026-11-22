package main

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

// Invoice discounts (000132, central's 000212): a discount on a whole sale
// once it reaches a total -- "invoices of $150 or more get 15% off", or a
// fixed amount off. Central decides the rules and which branches get them;
// the till keeps only those that apply to it.
//
// The total checked is the sale's after item discounts and deals, before
// tax, and the discount comes off that, on top of them. When several rules
// apply, the one that saves the customer most wins; they never add up.
// An amount rule that repeats comes off once for every minimum the total
// reaches: "$50 off every $100" takes $100 off a $250 sale.
//
// A rule that replaces item discounts never comes off on top of the lines'
// own discounts: the sale is also priced with every line at no discount of
// its own and that rule in play, its minimum judged on that total, and the
// cheaper of the two is charged ("$50 off every $100, else the 20%").
//
// The discount is spread over the lines in proportion to what each costs
// and taken off their line totals (invoice_products.invoice_discount holds
// each share), so a return or an exchange credits what the line was
// actually paid. Exchanges don't earn one: their replacements are priced
// without it.

const (
	invoiceDiscountPercent = "percent"
	invoiceDiscountAmount  = "amount"
)

// syncedInvoiceDiscount is a rule as central sends it: every rule changed
// since the last pull, with whether it applies to this branch.
type syncedInvoiceDiscount struct {
	UID       string     `json:"uid"`
	Name      string     `json:"name"`
	MinTotal  int64      `json:"min_total"`
	Kind      string     `json:"kind"`
	Percent   int64      `json:"percent"`
	Amount    int64      `json:"amount"`
	ValidFrom *time.Time `json:"valid_from"`
	ValidTo   *time.Time `json:"valid_to"`
	Applies   bool       `json:"applies"`
	// CashierPicks: the rule counts only when the cashier picks it for the
	// sale (the Rewards button), like a student discount.
	CashierPicks bool `json:"cashier_picks"`
	// Repeats: the amount comes off once for every min_total reached.
	Repeats bool `json:"repeats"`
	// ReplacesItemDiscounts: never on top of the lines' own discounts; when
	// it applies the lines go without them (see priceInvoiceTotals).
	ReplacesItemDiscounts bool `json:"replaces_item_discounts"`
}

func applySyncedInvoiceDiscount(ctx context.Context, q *db.Queries, item syncedInvoiceDiscount) error {
	if item.UID == "" {
		return fmt.Errorf("invoice discount without a uid")
	}
	if !item.Applies {
		return q.DeleteInvoiceDiscount(ctx, item.UID)
	}
	return q.UpsertInvoiceDiscount(ctx, db.UpsertInvoiceDiscountParams{
		Uid: item.UID, Name: item.Name, MinTotal: item.MinTotal, Kind: item.Kind,
		Percent: item.Percent, Amount: item.Amount,
		ValidFrom: rfc3339OrEmpty(item.ValidFrom), ValidTo: rfc3339OrEmpty(item.ValidTo),
		CashierPicks: boolToInt64(item.CashierPicks),
		Repeats:      boolToInt64(item.Repeats),

		ReplacesItemDiscounts: boolToInt64(item.ReplacesItemDiscounts),
	})
}

func rfc3339OrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// bestInvoiceDiscount picks, among the rules in force at now whose minimum
// the total reaches, the one that takes the most off it. A rule the cashier
// picks counts only when picked holds its uid.
func bestInvoiceDiscount(rules []db.InvoiceDiscount, total int64, now time.Time, roundingMode string, picked ...string) invoiceDiscount {
	var best invoiceDiscount
	if total <= 0 {
		return best
	}
	for _, r := range rules {
		if total < r.MinTotal || !invoiceDiscountInForce(r, now) {
			continue
		}
		if r.CashierPicks != 0 && !slices.Contains(picked, r.Uid) {
			continue
		}
		cents := min(invoiceDiscountOff(r, total, roundingMode), total)
		if cents > best.Cents {
			best = invoiceDiscount{Name: r.Name, Cents: cents, replacesItemDiscounts: r.ReplacesItemDiscounts != 0}
		}
	}
	return best
}

// friendReferralRule is the new customer's referral discount as one more
// invoice discount, when the sale carries a referral code (referralPickPrefix
// in the picks), central's program rewards the new customer, and the code is
// one this customer may use (checkReferralCode: a first purchase, not their
// own code...). As a rule it competes with the deals, the biggest winning,
// or adds up with them where the branch lets deals add up; its minimum is
// the program's minimum spend.
func friendReferralRule(ctx context.Context, q *db.Queries, clientID int64, picked []string) (db.InvoiceDiscount, bool, error) {
	code := referralFromPicks(picked)
	if code == "" {
		return db.InvoiceDiscount{}, false, nil
	}
	program, err := referralProgram(ctx, q)
	if err != nil {
		return db.InvoiceDiscount{}, false, err
	}
	if !referralRewardsFriend(program) {
		return db.InvoiceDiscount{}, false, nil
	}
	if _, _, err := checkReferralCode(ctx, q, code, clientID); err != nil {
		// A code this customer can't use takes nothing off; checkout's own
		// check refuses it with the reason.
		return db.InvoiceDiscount{}, false, nil
	}
	return db.InvoiceDiscount{
		Uid: referralPickPrefix + code, Name: friendReferralLabel(program.FriendPercent),
		Kind: invoiceDiscountPercent, Percent: program.FriendPercent, MinTotal: program.MinFriendSpend,
	}, true, nil
}

// invoiceDiscountOff is what a rule takes off a sale of total: its percent
// of it, or its amount, once for every min_total in it when it repeats.
func invoiceDiscountOff(r db.InvoiceDiscount, total int64, roundingMode string) int64 {
	switch r.Kind {
	case invoiceDiscountPercent:
		return percentOf(total, r.Percent, roundingMode)
	case invoiceDiscountAmount:
		return r.Amount * invoiceDiscountTimes(r, total)
	}
	return 0
}

// invoiceDiscountTimes is how many times an amount rule comes off: once, or
// once per min_total the total reaches when it repeats.
func invoiceDiscountTimes(r db.InvoiceDiscount, total int64) int64 {
	if r.Repeats == 0 || r.MinTotal <= 0 {
		return 1
	}
	return max(total/r.MinTotal, 1)
}

// invoiceDiscountsStacked is the sale's invoice discounts when a branch lets
// them add up (deals_stack): every rule that applies (its minimum judged on
// the sale before any of them, a picked one only when picked) comes off one
// after the other. Percents go first, biggest first, each on what is left;
// then fixed amounts, biggest first (a repeating one counted by the sale
// before any of them), never past what is left. The names are
// joined in that order ("Students 15% + 10% above $50"); one rule alone is
// what bestInvoiceDiscount would give.
func invoiceDiscountsStacked(rules []db.InvoiceDiscount, total int64, now time.Time, roundingMode string, picked ...string) invoiceDiscount {
	if total <= 0 {
		return invoiceDiscount{}
	}
	var on []db.InvoiceDiscount
	for _, r := range rules {
		if total < r.MinTotal || !invoiceDiscountInForce(r, now) {
			continue
		}
		if r.CashierPicks != 0 && !slices.Contains(picked, r.Uid) {
			continue
		}
		if (r.Kind == invoiceDiscountPercent && r.Percent > 0) || (r.Kind == invoiceDiscountAmount && r.Amount > 0) {
			on = append(on, r)
		}
	}
	slices.SortFunc(on, func(a, b db.InvoiceDiscount) int {
		if (a.Kind == invoiceDiscountPercent) != (b.Kind == invoiceDiscountPercent) {
			if a.Kind == invoiceDiscountPercent {
				return -1
			}
			return 1
		}
		if a.Kind == invoiceDiscountPercent && a.Percent != b.Percent {
			return int(b.Percent - a.Percent)
		}
		if a.Kind == invoiceDiscountAmount {
			if x, y := a.Amount*invoiceDiscountTimes(a, total), b.Amount*invoiceDiscountTimes(b, total); x != y {
				return int(y - x)
			}
		}
		// Same size: by name, so a receipt names them in the same order.
		return strings.Compare(a.Name, b.Name)
	})
	left := total
	var names []string
	replaces := false
	for _, r := range on {
		if left <= 0 {
			break
		}
		var cents int64
		if r.Kind == invoiceDiscountPercent {
			cents = percentOf(left, r.Percent, roundingMode)
		} else {
			cents = r.Amount * invoiceDiscountTimes(r, total)
		}
		cents = min(cents, left)
		if cents <= 0 {
			continue
		}
		left -= cents
		names = append(names, r.Name)
		replaces = replaces || r.ReplacesItemDiscounts != 0
	}
	return invoiceDiscount{Name: strings.Join(names, " + "), Cents: total - left, replacesItemDiscounts: replaces}
}

// invoiceDiscountInForce: valid_to is the first moment the rule is over.
// A date the till can't read keeps the rule out rather than in.
func invoiceDiscountInForce(r db.InvoiceDiscount, now time.Time) bool {
	if r.ValidFrom != "" {
		from, err := time.Parse(time.RFC3339, r.ValidFrom)
		if err != nil || now.Before(from) {
			return false
		}
	}
	if r.ValidTo != "" {
		to, err := time.Parse(time.RFC3339, r.ValidTo)
		if err != nil || !now.Before(to) {
			return false
		}
	}
	return true
}

// spreadInvoiceDiscount takes cents off the lines in proportion to their
// totals. What rounding leaves goes to the dearest line, so the shares add
// up to cents exactly; no line goes below zero since cents never exceeds
// their sum.
func spreadInvoiceDiscount(products []pricedInvoiceProduct, cents int64) []pricedInvoiceProduct {
	var sum int64
	dearest := -1
	for i, p := range products {
		if p.lineTotal <= 0 {
			continue
		}
		sum += p.lineTotal
		if dearest < 0 || p.lineTotal > products[dearest].lineTotal {
			dearest = i
		}
	}
	if cents <= 0 || sum <= 0 {
		return products
	}
	cents = min(cents, sum)
	out := make([]pricedInvoiceProduct, len(products))
	copy(out, products)
	var given int64
	for i := range out {
		if out[i].lineTotal <= 0 {
			continue
		}
		share := cents * out[i].lineTotal / sum
		out[i].invoiceDiscount = share
		given += share
	}
	out[dearest].invoiceDiscount += cents - given
	for i := range out {
		out[i].lineTotal -= out[i].invoiceDiscount
	}
	return out
}

// applyInvoiceDiscount finds the sale's invoice discount and takes it off
// its lines and its discounted subtotal: the biggest rule, or all of them
// added up where the branch says so (deals_stack). The buyer's referral
// reward competes with that: it wins only when it takes off more, so a tie
// keeps the reward for a later sale.
// picked are the uids of the rules the cashier picked for the sale.
// withReplacing lets the rules that replace item discounts in: only for the
// sale priced without its lines' own discounts.
func (a *App) applyInvoiceDiscount(q *db.Queries, totals *invoiceTotals, roundingMode string, clientID int64, picked []string, withReplacing bool) error {
	all, err := q.ListInvoiceDiscounts(a.ctx)
	if err != nil {
		return fmt.Errorf("load invoice discounts: %w", err)
	}
	var rules []db.InvoiceDiscount
	for _, r := range all {
		if withReplacing || r.ReplacesItemDiscounts == 0 {
			rules = append(rules, r)
		}
	}
	if friend, ok, err := friendReferralRule(a.ctx, q, clientID, picked); err != nil {
		return err
	} else if ok {
		rules = append(rules, friend)
	}
	now := time.Now()
	best := bestInvoiceDiscount(rules, totals.DiscountedSubtotal, now, roundingMode, picked...)
	// A branch can let its deals add up instead of the biggest taking over.
	if v, err := q.GetFeatureFlag(a.ctx, dealsStackFlag); err == nil && v != 0 {
		best = invoiceDiscountsStacked(rules, totals.DiscountedSubtotal, now, roundingMode, picked...)
	}
	reward, ok, err := usableReferralReward(a.ctx, q, clientID, now)
	if err != nil {
		return fmt.Errorf("load referral reward: %w", err)
	}
	if ok {
		if cents := referralRewardCents(reward, totals.DiscountedSubtotal, roundingMode); cents > best.Cents {
			best = invoiceDiscount{Name: referralRewardLabel(reward.Percent), Cents: cents, RewardUID: reward.Uid}
		}
	}
	if best.Cents <= 0 {
		return nil
	}
	totals.Products = spreadInvoiceDiscount(totals.Products, best.Cents)
	totals.DiscountedSubtotal -= best.Cents
	totals.InvoiceDiscount = best
	return nil
}

// replacingRuleOffered: a rule that replaces item discounts is in force and,
// when the cashier picks it, picked; only then is the sale priced the second
// way.
func replacingRuleOffered(rules []db.InvoiceDiscount, now time.Time, picked []string) bool {
	for _, r := range rules {
		if r.ReplacesItemDiscounts == 0 || !invoiceDiscountInForce(r, now) {
			continue
		}
		if r.CashierPicks == 0 || slices.Contains(picked, r.Uid) {
			return true
		}
	}
	return false
}

// returnedInvoiceDiscount is the part of a sold line's invoice discount
// that qty returned units carry, after earlier returns of it: a whole cent
// per unit, and the units that empty the line take what's left over, so
// returning all of it gives back exactly its share.
func returnedInvoiceDiscount(line db.InvoiceProduct, alreadyReturned, qty int64) int64 {
	if line.InvoiceDiscount <= 0 || line.Quantity <= 0 {
		return 0
	}
	perUnit := line.InvoiceDiscount / line.Quantity
	if alreadyReturned+qty >= line.Quantity {
		return line.InvoiceDiscount - perUnit*alreadyReturned
	}
	return perUnit * qty
}

// parsePickedDiscounts reads the picked invoice discounts the screen sends: a
// JSON list of uids, "" for none; each uid once.
func parsePickedDiscounts(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var uids []string
	if err := json.Unmarshal([]byte(raw), &uids); err != nil {
		return nil, fmt.Errorf("picked discounts: %w", err)
	}
	var out []string
	for _, uid := range uids {
		if uid = strings.TrimSpace(uid); uid != "" && !slices.Contains(out, uid) {
			out = append(out, uid)
		}
	}
	return out, nil
}

// RewardDeal is an invoice discount as the Rewards button lists it.
type RewardDeal struct {
	UID      string `json:"uid"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Percent  int64  `json:"percent"`
	Amount   int64  `json:"amount"`
	MinTotal int64  `json:"minTotal"`
	// CashierPicks: the cashier turns it on for a sale; otherwise it applies
	// by itself once the total reaches MinTotal (shown for reference, only
	// central changes it).
	CashierPicks bool `json:"cashierPicks"`
	// Repeats: the amount comes off once for every MinTotal reached.
	Repeats bool `json:"repeats"`
	// ReplacesItemDiscounts: the lines lose their own discounts when it
	// applies, so it never adds up with them.
	ReplacesItemDiscounts bool `json:"replacesItemDiscounts"`
}

// ListRewardDeals is every invoice discount in force at this branch now,
// the ones the cashier picks first.
func (a *App) ListRewardDeals() ([]RewardDeal, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rules, err := a.store.ListInvoiceDiscounts(a.ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := []RewardDeal{}
	for _, r := range rules {
		if !invoiceDiscountInForce(r, now) {
			continue
		}
		out = append(out, RewardDeal{UID: r.Uid, Name: r.Name, Kind: r.Kind, Percent: r.Percent, Amount: r.Amount, MinTotal: r.MinTotal, CashierPicks: r.CashierPicks != 0, Repeats: r.Repeats != 0, ReplacesItemDiscounts: r.ReplacesItemDiscounts != 0})
	}
	slices.SortStableFunc(out, func(x, y RewardDeal) int {
		if x.CashierPicks != y.CashierPicks {
			if x.CashierPicks {
				return -1
			}
			return 1
		}
		return 0
	})
	return out, nil
}
