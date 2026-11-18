package main

import (
	"slices"
	"testing"
)

// The pull order: small settings first, so a change to them reaches the till
// during a long products or clients catch-up; everything that refers to
// products or clients after them; every entity once.
func TestPullOrder(t *testing.T) {
	at := func(e string) int {
		i := slices.Index(pullEntities, e)
		if i < 0 {
			t.Fatalf("%s is not pulled", e)
		}
		return i
	}
	before := func(a, b string) {
		if at(a) >= at(b) {
			t.Fatalf("%s must be pulled before %s: %v", a, b, pullEntities)
		}
	}
	// The reported case: referrals switched on wait behind nothing big.
	for _, small := range []string{"referral_program", "invoice_discounts", "calendar_settings", "branches", "salespersons", "branch_users", "coupon_types", "expense_categories", "loan_categories"} {
		before(small, "products")
		before(small, "clients")
	}
	for _, needsProducts := range []string{"price_lists", "discount_lists", "quick_access", "branch_stock"} {
		before("products", needsProducts)
	}
	for _, needsClients := range []string{"referral_rewards", "coupons"} {
		before("clients", needsClients)
	}
	before("coupon_types", "coupons")
	before("salespersons", "branch_targets")
	seen := map[string]bool{}
	for _, e := range pullEntities {
		if seen[e] {
			t.Fatalf("%s pulled twice", e)
		}
		seen[e] = true
	}
	if len(pullEntities) != 23 {
		t.Fatalf("%d entities, want the 23 central serves", len(pullEntities))
	}
}
