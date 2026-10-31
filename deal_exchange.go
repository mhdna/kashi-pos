package main

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	db "kashi-pos/db/sqlc"
)

// Exchanges and returns of units sold on a deal (deals.go). The rules, as
// agreed for the shop:
//
//   - A paid unit is credited what was paid for it, as any returned unit is:
//     at the till the replacements must cover it.
//   - A reward unit (a free unit of "Buy 2 get 1", the half-price one of "2nd
//     50% off") is credited what was paid for it too, plus swap credit up to
//     its shelf price. The swap credit only buys replacements in the same
//     exchange; what is left of it is forfeited, never paid out. So the free
//     $5 unit swaps for anything up to $5 at no cost, and for a $7 unit the
//     customer pays $2.
//   - The deal has to stay earned. After the exchange the till looks at the
//     sale's deal as the customer now holds it: the units they kept, the
//     reward units they swapped (still theirs, in kind), and the replacements
//     that took the place of returned paid units from the same deal. If that
//     no longer earns the saving they still enjoy, the difference is charged
//     back. Swap a paid shirt for a jacket outside the deal and the free
//     shirt stops being free; swap it for another shirt on the deal and
//     nothing changes.
//
// Saving enjoyed = deal discount on the units kept + on the reward units
// swapped - swap credit forfeited - what earlier exchanges charged back.
// Saving earned  = the deal (the sale's copy of its terms) priced on the
// units kept, the reward units swapped, and the replacements counted.
// Charge back    = enjoyed - earned, when positive.
//
// The amounts ride on the exchange's own lines -- swap credit on the returned
// reward unit's line, a charge back off the returned lines' credit -- so the
// lines still add up to the invoice, for the receipt and for kashi.

// dealExchangeLine is a returned unit's place in its sale's deal.
type dealExchangeLine struct {
	dealUID  string
	unitDeal int64 // the deal discount of each unit, as sold
	reward   bool  // a reward unit: swap credit up to its shelf price
}

type exchangeDealAdjustment struct {
	dealUID   string
	dealName  string
	forfeited int64
	clawback  int64
}

// dealGroupState is one deal of a sale as the customer would hold it after
// the exchange. Prices are shelf prices; deals are unit deal discounts.
type dealGroupState struct {
	terms          dealTerms
	kept           []dealUnit
	swappedRewards []dealUnit
	counted        []dealUnit // deal is 0: bought in the exchange
	forfeited      int64      // this exchange's and earlier ones'
	clawedBack     int64      // earlier exchanges'
}

// fallback is the unit's own fallback on the deal (deals.go withFallback).
type dealUnit struct {
	price, deal int64
	fallback    sql.NullInt64
}

func (g dealGroupState) clawback(roundingMode string) int64 {
	enjoyed := -g.forfeited - g.clawedBack
	units := make([]dealUnit, 0, len(g.kept)+len(g.swappedRewards)+len(g.counted))
	units = append(append(append(units, g.kept...), g.swappedRewards...), g.counted...)
	prices := make([]int64, len(units))
	fallbacks := make([]sql.NullInt64, len(units))
	for i, u := range units {
		enjoyed += u.deal
		prices[i], fallbacks[i] = u.price, u.fallback
	}
	earned := g.terms.totalDiscount(prices, fallbacks, roundingMode)
	return max(0, enjoyed-earned)
}

// soldLineKey tells a sale's lines apart the way a returned line is matched
// to one: product, price, discount, its unit deal discount and fallback.
type soldLineKey struct {
	productID, unitPrice, discount int64
	dealUID                        string
	unitDeal                       int64
	fallback                       sql.NullInt64
}

func keyOfLine(l db.InvoiceProduct) soldLineKey {
	unitDeal := int64(0)
	if l.Quantity != 0 {
		unitDeal = l.DealDiscount / l.Quantity
	}
	return soldLineKey{l.ProductID, l.UnitPrice, l.Discount, l.DealUid, unitDeal, l.DealFallbackPercent}
}

// settleExchangeDeals applies the rules above to a priced exchange. Before
// it, every returned line is credited what was paid; after it, returned
// reward lines carry the swap credit used, returned lines in a deal that is
// no longer earned carry the charge back, the replacements that keep a deal
// going name it, and result.requiredReplacement is what the replacements
// must cover at the till (what was paid, never the swap credit).
func (a *App) settleExchangeDeals(q *db.Queries, sourceID int64, sourceLines []db.InvoiceProduct, result *pricedExchange, roundingMode string) error {
	result.requiredReplacement = result.returnedTotal
	invoiceDeals, err := q.ListInvoiceDeals(a.ctx, sourceID)
	if err != nil {
		return fmt.Errorf("sale deals: %w", err)
	}
	if len(invoiceDeals) == 0 {
		for i := range result.replacements {
			result.replacements[i].dealUID = ""
		}
		return nil
	}
	terms := map[string]dealTerms{}
	var order []string
	for _, d := range invoiceDeals {
		terms[d.DealUid] = dealTermsFromInvoice(d)
		order = append(order, d.DealUid)
	}

	// Swap credit: what replacements are left to buy after covering the
	// paid credit, given to the returned reward lines in turn.
	spare := max(0, result.replacementTotal-result.returnedTotal)
	usedByDeal := map[string]int64{}
	allowanceByDeal := map[string]int64{}
	for i := range result.returned {
		r := &result.returned[i]
		if !r.deal.reward {
			continue
		}
		allowance := r.deal.unitDeal * r.qty
		used := min(allowance, spare)
		spare -= used
		r.lineTotal += used
		allowanceByDeal[r.deal.dealUID] += allowance
		usedByDeal[r.deal.dealUID] += used
	}

	// Replacements on a deal count towards it only in place of returned
	// paid units of it, the cheapest first, so a swapped-in unit never
	// earns a reward it wasn't bought for.
	slots := map[string]int64{}
	for _, r := range result.returned {
		if r.deal.dealUID != "" && !r.deal.reward {
			slots[r.deal.dealUID] += r.qty
		}
	}
	result.replacements = countReplacements(result.replacements, slots)

	earlierLines, err := q.ListSaleExchangeDealLines(a.ctx, sourceID)
	if err != nil {
		return fmt.Errorf("earlier deal returns: %w", err)
	}
	earlierAdjustments, err := q.ListSaleDealAdjustments(a.ctx, sourceID)
	if err != nil {
		return fmt.Errorf("earlier deal adjustments: %w", err)
	}

	for _, uid := range order {
		group := dealGroupState{terms: terms[uid]}
		group.forfeited = allowanceByDeal[uid] - usedByDeal[uid]
		for _, adj := range earlierAdjustments {
			if adj.DealUid == uid {
				group.forfeited += adj.Forfeited
				group.clawedBack += adj.Clawback
			}
		}

		// Units of the sale on this deal, less every unit returned so far.
		left := map[soldLineKey]int64{}
		var keys []soldLineKey
		for _, l := range sourceLines {
			if l.DealUid != uid || l.Quantity <= 0 {
				continue
			}
			k := keyOfLine(l)
			if _, seen := left[k]; !seen {
				keys = append(keys, k)
			}
			left[k] += l.Quantity
		}
		returnUnits := func(k soldLineKey, qty int64) {
			left[k] -= qty
			if group.terms.isRewardUnit(k.unitPrice, k.unitDeal, k.fallback, roundingMode) {
				for range qty {
					group.swappedRewards = append(group.swappedRewards, dealUnit{k.unitPrice, k.unitDeal, k.fallback})
				}
			}
		}
		for _, l := range earlierLines {
			if l.DealUid != uid {
				continue
			}
			if l.Quantity < 0 {
				returnUnits(keyOfLine(l), -l.Quantity)
			} else {
				for range l.Quantity {
					group.counted = append(group.counted, dealUnit{price: l.UnitPrice, fallback: l.DealFallbackPercent})
				}
			}
		}
		for _, r := range result.returned {
			if r.deal.dealUID == uid {
				returnUnits(keyOfLine(r.line), r.qty)
			}
		}
		for _, p := range result.replacements {
			if p.dealUID == uid {
				for range p.qty {
					group.counted = append(group.counted, dealUnit{price: p.unitPrice, fallback: replacementDealFallback(p)})
				}
			}
		}
		for _, k := range keys {
			for range max(0, left[k]) {
				group.kept = append(group.kept, dealUnit{k.unitPrice, k.unitDeal, k.fallback})
			}
		}

		// A reward unit returned with nothing bought in its place still counts
		// as swapped; its forfeited credit cancels the saving it brings, so
		// it only keeps the deal's count, which the customer did buy for.
		clawback := group.clawback(roundingMode)
		clawback = takeClawback(result, uid, clawback)
		forfeited := allowanceByDeal[uid] - usedByDeal[uid]
		if forfeited > 0 || clawback > 0 {
			result.dealAdjustments = append(result.dealAdjustments, exchangeDealAdjustment{
				dealUID: uid, dealName: group.terms.Name, forfeited: forfeited, clawback: clawback,
			})
		}
	}

	result.returnedTotal = 0
	for _, r := range result.returned {
		result.returnedTotal += r.lineTotal
	}
	return nil
}

// countReplacements keeps a replacement's deal only on as many units as
// there are returned paid units of that deal, the cheapest first; a line
// counted in part is split.
func countReplacements(lines []pricedInvoiceProduct, slots map[string]int64) []pricedInvoiceProduct {
	idx := make([]int, len(lines))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return lines[idx[a]].unitPrice < lines[idx[b]].unitPrice })
	countedQty := make([]int64, len(lines))
	for _, i := range idx {
		uid := lines[i].dealUID
		if uid == "" {
			continue
		}
		countedQty[i] = min(lines[i].qty, slots[uid])
		slots[uid] -= countedQty[i]
	}
	out := make([]pricedInvoiceProduct, 0, len(lines))
	for i, l := range lines {
		counted, rest := countedQty[i], l.qty-countedQty[i]
		if counted > 0 {
			c := l
			c.qty = counted
			c.lineTotal = l.lineTotal / l.qty * counted
			out = append(out, c)
		}
		if rest > 0 {
			r := l
			r.qty, r.dealUID = rest, ""
			r.lineTotal = l.lineTotal - l.lineTotal/l.qty*counted
			out = append(out, r)
		}
	}
	return out
}

// takeClawback takes a charge back off the credit of the deal's returned
// lines, paid lines first, and answers how much it took. In an exchange
// what the credits can't cover goes on the first returned line, which then
// charges rather than credits (one paid unit of "Buy 2 get 2" carries two
// rewards). A refund never asks for money: it stops at no credit.
func takeClawback(result *pricedExchange, uid string, amount int64) int64 {
	taken := int64(0)
	first := -1
	for _, rewardPass := range []bool{false, true} {
		for i := range result.returned {
			r := &result.returned[i]
			if r.deal.dealUID != uid || r.deal.reward != rewardPass {
				continue
			}
			if first < 0 {
				first = i
			}
			take := min(amount, max(0, r.lineTotal))
			r.lineTotal -= take
			amount -= take
			taken += take
		}
	}
	if amount > 0 && first >= 0 && len(result.replacements) > 0 {
		result.returned[first].lineTotal -= amount
		taken += amount
	}
	return taken
}

// recordExchangeDealAdjustments keeps what the exchange did to the sale's
// deals, for the next exchange of it to read.
func recordExchangeDealAdjustments(ctx context.Context, q *db.Queries, invoiceID, saleID int64, adjustments []exchangeDealAdjustment) error {
	for _, adj := range adjustments {
		if err := q.CreateExchangeDealAdjustment(ctx, db.CreateExchangeDealAdjustmentParams{
			InvoiceID: invoiceID, SalesInvoiceID: saleID, DealUid: adj.dealUID,
			Forfeited: adj.forfeited, Clawback: adj.clawback,
		}); err != nil {
			return fmt.Errorf("record deal adjustment: %w", err)
		}
	}
	return nil
}

// dealAdjustmentNotes says on the receipt what an exchange did to a deal.
func dealAdjustmentNotes(adjustments []exchangeDealAdjustment) string {
	var lines []string
	for _, adj := range adjustments {
		if adj.clawback > 0 {
			lines = append(lines, fmt.Sprintf("%s no longer applies: $%.2f charged back", adj.dealName, float64(adj.clawback)/100))
		}
		if adj.forfeited > 0 {
			lines = append(lines, fmt.Sprintf("Unused credit of a free item (%s): $%.2f", adj.dealName, float64(adj.forfeited)/100))
		}
	}
	return strings.Join(lines, "\n")
}

// dealNameOf is a sale's deal by name, for the exchange sent to kashi.
func dealNameOf(saleID int64, uid string, q *db.Queries, a *App) string {
	if uid == "" {
		return ""
	}
	deals, err := q.ListInvoiceDeals(a.ctx, saleID)
	if err != nil {
		return ""
	}
	for _, d := range deals {
		if d.DealUid == uid {
			return d.Name
		}
	}
	return ""
}
