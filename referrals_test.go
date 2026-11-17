package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

// codeWithCheck is body with its check character.
func codeWithCheck(t *testing.T, body string) string {
	t.Helper()
	c, ok := referralCheckChar(body)
	if !ok {
		t.Fatalf("body %q has a character outside the alphabet", body)
	}
	return body + string(c)
}

// The same vectors are in central's referral test (api/referral_code_test.go):
// both sides must agree on which codes are real.
var referralCodeVectors = map[string]string{"AAAAA": "A", "K7M4Q": "H", "XY346": "F", "34679": "P", "CDEFG": "M"}

func TestReferralCheckCharVectors(t *testing.T) {
	for body, want := range referralCodeVectors {
		got, ok := referralCheckChar(body)
		if !ok || string(got) != want {
			t.Errorf("check(%s) = %q, want %q", body, got, want)
		}
		if !validReferralCode(body + want) {
			t.Errorf("%s%s is not valid", body, want)
		}
	}
}

// Every wrong character and every swap of two different neighbours is
// caught, on a few thousand random codes.
func TestReferralCheckCharCatchesTypos(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for n := 0; n < 3000; n++ {
		body := make([]byte, referralCodeLength-1)
		for i := range body {
			body[i] = referralAlphabet[rng.Intn(len(referralAlphabet))]
		}
		code := codeWithCheck(t, string(body))
		if !validReferralCode(code) {
			t.Fatalf("%s rejected", code)
		}
		for i := 0; i < len(code); i++ {
			for j := 0; j < len(referralAlphabet); j++ {
				if referralAlphabet[j] == code[i] {
					continue
				}
				typo := code[:i] + string(referralAlphabet[j]) + code[i+1:]
				if validReferralCode(typo) {
					t.Fatalf("%s (from %s) passed", typo, code)
				}
			}
			if i+1 < len(code) && code[i] != code[i+1] {
				swapped := code[:i] + string(code[i+1]) + string(code[i]) + code[i+2:]
				if validReferralCode(swapped) {
					t.Fatalf("swap %s (from %s) passed", swapped, code)
				}
			}
		}
	}
}

func TestNormalizeAndValidateReferralCode(t *testing.T) {
	code := codeWithCheck(t, "K7M4Q")
	for _, typed := range []string{code, strings.ToLower(code), " K7M-4Q" + code[5:] + " ", "k7m 4q" + strings.ToLower(code[5:])} {
		if got := normalizeReferralCode(typed); got != code || !validReferralCode(got) {
			t.Errorf("%q -> %q", typed, got)
		}
	}
	for _, bad := range []string{"", "K7M4Q", code + "A", "K7M4O" + code[5:], "0" + code[1:], "K7M4Q" + "B"} {
		if validReferralCode(normalizeReferralCode(bad)) {
			t.Errorf("%q passed", bad)
		}
	}
}

// referralShop is the deal shop with referrals on: an owner with a code,
// and the shop's client (a new customer) known to central.
type referralShop struct {
	dealShop
	owner     db.Client
	ownerCode string
}

func setUpReferralShop(t *testing.T) referralShop {
	t.Helper()
	ctx := context.Background()
	shop := setUpDealShop(t)
	q := shop.store.Queries
	if err := applySyncedReferralProgram(ctx, q, syncedReferralProgram{Enabled: true, RewardPercent: 15}); err != nil {
		t.Fatal(err)
	}
	ownerCode := codeWithCheck(t, "K7M4Q")
	if err := applySyncedClient(ctx, q, syncedClient{UID: "uid-owner", Name: "Rami", Phone: "+96171000001", ReferralCode: &ownerCode}); err != nil {
		t.Fatal(err)
	}
	owner, err := q.GetClientByPhone(ctx, "+96171000001")
	if err != nil {
		t.Fatal(err)
	}
	friendCode := codeWithCheck(t, "XY346")
	if err := applySyncedClient(ctx, q, syncedClient{UID: "uid-friend", Name: shop.client.Name, Phone: shop.client.Phone, ReferralCode: &friendCode}); err != nil {
		t.Fatal(err)
	}
	shop.client, err = q.GetClientByID(ctx, shop.client.ID)
	if err != nil {
		t.Fatal(err)
	}
	return referralShop{dealShop: shop, owner: owner, ownerCode: ownerCode}
}

func (s referralShop) giveReward(t *testing.T, r syncedReferralReward) {
	t.Helper()
	if r.Status == "" {
		r.Status = "available"
	}
	if err := applySyncedReferralReward(context.Background(), s.store.Queries, r); err != nil {
		t.Fatal(err)
	}
}

// sellTo checks out codes for clientID, paying the quoted total.
func (s referralShop) sellTo(clientID int64, referralCode string, codes ...string) (SalesInvoiceResult, InvoiceQuote, error) {
	var items []salesInvoiceItem
	at := map[string]int{}
	for _, code := range codes {
		if i, ok := at[code]; ok {
			items[i].Qty++
			continue
		}
		at[code] = len(items)
		items = append(items, salesInvoiceItem{Barcode: "bc-" + code, Qty: 1})
	}
	itemsJSON, _ := json.Marshal(items)
	quote, err := s.app.QuoteInvoiceTotal(string(itemsJSON), clientID)
	if err != nil {
		return SalesInvoiceResult{}, quote, err
	}
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: s.cashbox.ID, AmountCents: quote.GrandTotalCents}})
	result, err := s.app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(itemsJSON), PaymentsJSON: string(payments), ClientID: clientID, ReferralCode: referralCode})
	return result, quote, err
}

func salePayload(t *testing.T, store *db.Store, code string) branchInvoicePayload {
	t.Helper()
	entries, err := store.ListDueSyncOutboxEntries(context.Background(), 200)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		var p branchInvoicePayload
		if e.EntityType == "sales_invoice" && json.Unmarshal([]byte(e.PayloadJson), &p) == nil && p.BranchInvoiceCode == code {
			return p
		}
	}
	t.Fatalf("no payload for %s", code)
	return branchInvoicePayload{}
}

// The user's case: a friend's first sale names the owner's code (the
// friend gets nothing), central gives the owner 15%, and the owner's next
// sale has it off; the one after doesn't.
func TestReferralFriendThenOwnerNextVisit(t *testing.T) {
	ctx := context.Background()
	shop := setUpReferralShop(t)

	friendSale, friendQuote, err := shop.sellTo(shop.client.ID, strings.ToLower(shop.ownerCode), "JACKET")
	if err != nil {
		t.Fatal(err)
	}
	if friendSale.GrandTotal != 10000 || friendQuote.InvoiceDiscountCents != 0 {
		t.Fatalf("the friend got something: %d, %+v", friendSale.GrandTotal, friendQuote)
	}
	inv, err := shop.store.GetInvoiceByID(ctx, friendSale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	if inv.ReferralCode != shop.ownerCode || inv.ReferralRewardUid != "" {
		t.Fatalf("invoice = %q / %q", inv.ReferralCode, inv.ReferralRewardUid)
	}
	if p := salePayload(t, shop.store, friendSale.InvoiceCode); p.ReferralCode != shop.ownerCode || p.ReferralRewardUID != "" {
		t.Fatalf("payload = %+v", p)
	}

	// Central answers with the owner's reward.
	shop.giveReward(t, syncedReferralReward{UID: "rw-1", OwnerClientUID: "uid-owner", Percent: 15})

	info, err := shop.app.GetReferralInfo(shop.owner.ID)
	if err != nil || !info.Enabled || info.Code != shop.ownerCode || info.RewardOfferPercent != 15 {
		t.Fatalf("info = %+v, %v", info, err)
	}

	// $100 jacket + $20 socks = $120, 15% = $18.
	sale, quote, err := shop.sellTo(shop.owner.ID, "", "JACKET", "SOCKS")
	if err != nil {
		t.Fatal(err)
	}
	want := invoiceDiscount{Name: "Referral reward 15%", Cents: 1800, RewardUID: "rw-1"}
	if sale.GrandTotal != 10200 || sale.InvoiceDiscount != want || quote.GrandTotalCents != 10200 || quote.InvoiceDiscountName != want.Name {
		t.Fatalf("sale = %d %+v, quote = %+v", sale.GrandTotal, sale.InvoiceDiscount, quote)
	}
	inv, err = shop.store.GetInvoiceByID(ctx, sale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	if inv.InvoiceDiscount != 1800 || inv.InvoiceDiscountName != want.Name || inv.ReferralRewardUid != "rw-1" || inv.ReferralCode != "" {
		t.Fatalf("invoice = %+v", inv)
	}
	lines, err := shop.store.ListInvoiceProducts(ctx, sale.InvoiceID)
	if err != nil {
		t.Fatal(err)
	}
	var shares int64
	for _, l := range lines {
		shares += l.InvoiceDiscount
	}
	if shares != 1800 {
		t.Fatalf("line shares = %d", shares)
	}
	if p := salePayload(t, shop.store, sale.InvoiceCode); p.ReferralRewardUID != "rw-1" || p.InvoiceDiscount != 1800 || p.ReferralCode != "" {
		t.Fatalf("payload = %+v", p)
	}

	// Used: the next visit pays in full, even before central hears of it.
	shop.giveReward(t, syncedReferralReward{UID: "rw-1", OwnerClientUID: "uid-owner", Percent: 15})
	again, _, err := shop.sellTo(shop.owner.ID, "", "JACKET")
	if err != nil {
		t.Fatal(err)
	}
	if again.GrandTotal != 10000 || again.InvoiceDiscount.Cents != 0 {
		t.Fatalf("second visit = %d %+v", again.GrandTotal, again.InvoiceDiscount)
	}
	if info, _ := shop.app.GetReferralInfo(shop.owner.ID); info.RewardOfferPercent != 0 {
		t.Fatalf("info after use = %+v", info)
	}
}

// A reward competes with the spend tiers: the one that takes more off wins,
// a tie goes to the tier, and a reward that lost stays for a later sale.
func TestReferralRewardAgainstTiers(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name       string
		codes      []string
		percent    int64
		maxCents   int64
		wantCents  int64
		wantReward bool
	}{
		// $100 + $20 = $120: the 30% tier ($100+) takes $36, more than 15%.
		{"tier wins", []string{"JACKET", "SOCKS"}, 15, 0, 3600, false},
		// $40: no tier reaches, the reward takes $6.
		{"reward under every tier", []string{"SOCKS", "SOCKS"}, 15, 0, 600, true},
		// $60: the 20% tier ($50+) takes $12, 25% takes $15.
		{"bigger reward beats a tier", []string{"SOCKS", "SOCKS", "SOCKS"}, 25, 0, 1500, true},
		// $60: 20% tier = $12 = a 20% reward: the tier, so the reward stays.
		{"a tie keeps the reward", []string{"SOCKS", "SOCKS", "SOCKS"}, 20, 0, 1200, false},
		// $40, 50% capped at $5.
		{"capped reward", []string{"SOCKS", "SOCKS"}, 50, 500, 500, true},
		// $60, 50% capped at $5 loses to the $12 tier.
		{"cap makes it lose", []string{"SOCKS", "SOCKS", "SOCKS"}, 50, 500, 1200, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shop := setUpReferralShop(t)
			for _, rule := range []syncedInvoiceDiscount{
				{UID: "t50", Name: "20% at $50", MinTotal: 5000, Kind: invoiceDiscountPercent, Percent: 20, Applies: true},
				{UID: "t100", Name: "30% at $100", MinTotal: 10000, Kind: invoiceDiscountPercent, Percent: 30, Applies: true},
			} {
				if err := applySyncedInvoiceDiscount(ctx, shop.store.Queries, rule); err != nil {
					t.Fatal(err)
				}
			}
			shop.giveReward(t, syncedReferralReward{UID: "rw", OwnerClientUID: "uid-owner", Percent: tc.percent, MaxCents: tc.maxCents})
			sale, quote, err := shop.sellTo(shop.owner.ID, "", tc.codes...)
			if err != nil {
				t.Fatal(err)
			}
			if sale.InvoiceDiscount.Cents != tc.wantCents || quote.InvoiceDiscountCents != tc.wantCents {
				t.Fatalf("took %d (quote %d), want %d", sale.InvoiceDiscount.Cents, quote.InvoiceDiscountCents, tc.wantCents)
			}
			if got := sale.InvoiceDiscount.RewardUID == "rw"; got != tc.wantReward {
				t.Fatalf("reward used = %v, want %v (%+v)", got, tc.wantReward, sale.InvoiceDiscount)
			}
			_, stillThere, err := usableReferralReward(ctx, shop.store.Queries, shop.owner.ID, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if stillThere == tc.wantReward {
				t.Fatalf("reward still usable = %v after a sale that used it = %v", stillThere, tc.wantReward)
			}
		})
	}
}

// Dates: not before available_from, not from expires_at on. The first to
// expire is used first.
func TestReferralRewardDates(t *testing.T) {
	ctx := context.Background()
	shop := setUpReferralShop(t)
	now := time.Now()
	later, past, soon, far := now.Add(time.Hour), now.Add(-time.Hour), now.Add(24*time.Hour), now.Add(240*time.Hour)
	shop.giveReward(t, syncedReferralReward{UID: "not-yet", OwnerClientUID: "uid-owner", Percent: 40, AvailableFrom: &later})
	shop.giveReward(t, syncedReferralReward{UID: "ended", OwnerClientUID: "uid-owner", Percent: 40, ExpiresAt: &past})
	shop.giveReward(t, syncedReferralReward{UID: "cancelled", OwnerClientUID: "uid-owner", Percent: 40, Status: "cancelled"})
	shop.giveReward(t, syncedReferralReward{UID: "someone-else", OwnerClientUID: "uid-friend", Percent: 40})
	if _, ok, _ := usableReferralReward(ctx, shop.store.Queries, shop.owner.ID, now); ok {
		t.Fatal("a reward outside its dates, cancelled or someone else's was usable")
	}
	shop.giveReward(t, syncedReferralReward{UID: "far", OwnerClientUID: "uid-owner", Percent: 10, ExpiresAt: &far})
	shop.giveReward(t, syncedReferralReward{UID: "open", OwnerClientUID: "uid-owner", Percent: 10})
	shop.giveReward(t, syncedReferralReward{UID: "soon", OwnerClientUID: "uid-owner", Percent: 10, ExpiresAt: &soon})
	got, ok, err := usableReferralReward(ctx, shop.store.Queries, shop.owner.ID, now)
	if err != nil || !ok || got.Uid != "soon" {
		t.Fatalf("picked %+v %v %v, want soon", got, ok, err)
	}
	// The walk-in never has one, whatever its uid.
	if _, ok, _ := usableReferralReward(ctx, shop.store.Queries, 0, now); ok {
		t.Fatal("no client had a reward")
	}
}

func TestCheckReferralCode(t *testing.T) {
	ctx := context.Background()
	shop := setUpReferralShop(t)
	q := shop.store.Queries

	got, err := shop.app.CheckReferralCode(" "+strings.ToLower(shop.ownerCode), shop.client.ID)
	if err != nil || got.Code != shop.ownerCode || got.OwnerName != "Rami" {
		t.Fatalf("got %+v, %v", got, err)
	}

	typo := shop.ownerCode[:5] + "A"
	if shop.ownerCode[5] == 'A' {
		typo = shop.ownerCode[:5] + "C"
	}
	walkIn, err := q.CreateClient(ctx, db.CreateClientParams{Name: "Walk-in", Phone: "00000000"})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		code     string
		clientID int64
		want     error
	}{
		{"typo", typo, shop.client.ID, errReferralCodeInvalid},
		{"nobody's", codeWithCheck(t, "AAAAA"), shop.client.ID, errReferralCodeUnknown},
		{"own code", shop.ownerCode, shop.owner.ID, errReferralOwnCode},
		{"no client", shop.ownerCode, 0, errReferralNeedsClient},
		{"walk-in", shop.ownerCode, walkIn.ID, errReferralNeedsClient},
	}
	for _, tc := range cases {
		if _, err := shop.app.CheckReferralCode(tc.code, tc.clientID); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v, want %v", tc.name, err, tc.want)
		}
	}

	// A customer who already bought here isn't new.
	if _, _, err := shop.sellTo(shop.client.ID, "", "SOCKS"); err != nil {
		t.Fatal(err)
	}
	if _, err := shop.app.CheckReferralCode(shop.ownerCode, shop.client.ID); !errors.Is(err, errReferralNotNewClient) {
		t.Fatalf("returning customer: %v", err)
	}

	// Off at central: no code is taken.
	if err := applySyncedReferralProgram(ctx, q, syncedReferralProgram{Enabled: false, RewardPercent: 15}); err != nil {
		t.Fatal(err)
	}
	if _, err := shop.app.CheckReferralCode(shop.ownerCode, shop.owner.ID); !errors.Is(err, errReferralsOff) {
		t.Fatalf("off: %v", err)
	}
}

// A sale with a code checkout refuses isn't made at all.
func TestCheckoutRefusesABadReferralCode(t *testing.T) {
	shop := setUpReferralShop(t)
	if _, _, err := shop.sellTo(shop.owner.ID, shop.ownerCode, "SOCKS"); !errors.Is(err, errReferralOwnCode) {
		t.Fatalf("own code: %v", err)
	}
	var n int
	if err := shop.app.conn.QueryRow(`SELECT count(*) FROM invoices`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("%d invoices, %v", n, err)
	}
	// A customer holding a reward isn't new either.
	shop.giveReward(t, syncedReferralReward{UID: "rw", OwnerClientUID: "uid-friend", Percent: 15})
	if _, _, err := shop.sellTo(shop.client.ID, shop.ownerCode, "SOCKS"); !errors.Is(err, errReferralCodeAndReward) {
		t.Fatalf("customer with a reward: %v", err)
	}
}

// Cancelling the sale that used a reward gives it back; a return doesn't.
func TestCancelGivesTheReferralRewardBack(t *testing.T) {
	ctx := context.Background()
	shop := setUpReferralShop(t)
	shop.giveReward(t, syncedReferralReward{UID: "rw", OwnerClientUID: "uid-owner", Percent: 15})
	sale, _, err := shop.sellTo(shop.owner.ID, "", "JACKET", "SOCKS")
	if err != nil || sale.InvoiceDiscount.RewardUID != "rw" {
		t.Fatalf("%+v %v", sale.InvoiceDiscount, err)
	}
	shop.refund(t, sale.InvoiceID, "SOCKS")
	if _, ok, _ := usableReferralReward(ctx, shop.store.Queries, shop.owner.ID, time.Now()); ok {
		t.Fatal("a return gave the reward back")
	}
	shop.giveReward(t, syncedReferralReward{UID: "rw-2", OwnerClientUID: "uid-owner", Percent: 15})
	sale, _, err = shop.sellTo(shop.owner.ID, "", "JACKET")
	if err != nil || sale.InvoiceDiscount.RewardUID != "rw-2" {
		t.Fatalf("%+v %v", sale.InvoiceDiscount, err)
	}
	if err := shop.app.cancelInvoice(ctx, sale.InvoiceID, "rung up by mistake", receiptAsSettings); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := usableReferralReward(ctx, shop.store.Queries, shop.owner.ID, time.Now()); !ok {
		t.Fatal("cancel didn't give the reward back")
	}
}

// Central's status rules: used or cancelled there is gone here.
func TestReferralRewardStatusFromCentral(t *testing.T) {
	ctx := context.Background()
	shop := setUpReferralShop(t)
	shop.giveReward(t, syncedReferralReward{UID: "rw", OwnerClientUID: "uid-owner", Percent: 15})
	shop.giveReward(t, syncedReferralReward{UID: "rw", OwnerClientUID: "uid-owner", Percent: 15, Status: "used"})
	if _, ok, _ := usableReferralReward(ctx, shop.store.Queries, shop.owner.ID, time.Now()); ok {
		t.Fatal("a reward used at another branch was usable here")
	}
	if err := applySyncedReferralReward(ctx, shop.store.Queries, syncedReferralReward{UID: "x"}); err == nil {
		t.Fatal("a reward without an owner was taken")
	}
}

// Randomized: with tiers and a reward in play, the quote is what checkout
// charges and stores, the lines' shares add up to the discount, and the
// winner is the bigger one.
func TestReferralQuoteMatchesCheckoutRandom(t *testing.T) {
	ctx := context.Background()
	rng := rand.New(rand.NewSource(42))
	codes := []string{"A", "B", "C", "D", "JACKET", "SOCKS"}
	shop := setUpReferralShop(t)
	q := shop.store.Queries
	for i := 0; i < 150; i++ {
		tierPercent, rewardPercent := int64(rng.Intn(40)), int64(1+rng.Intn(60))
		var maxCents int64
		if rng.Intn(3) == 0 {
			maxCents = int64(100 + rng.Intn(5000))
		}
		if err := applySyncedInvoiceDiscount(ctx, q, syncedInvoiceDiscount{
			UID: "tier", Name: "tier", MinTotal: int64(rng.Intn(20000)), Kind: invoiceDiscountPercent, Percent: tierPercent, Applies: tierPercent > 0,
		}); err != nil {
			t.Fatal(err)
		}
		uid := fmt.Sprintf("rw-%d", i)
		shop.giveReward(t, syncedReferralReward{UID: uid, OwnerClientUID: "uid-owner", Percent: rewardPercent, MaxCents: maxCents})
		var cart []string
		for n := 1 + rng.Intn(5); n > 0; n-- {
			cart = append(cart, codes[rng.Intn(len(codes))])
		}
		sale, quote, err := shop.sellTo(shop.owner.ID, "", cart...)
		if err != nil {
			t.Fatalf("case %d %v: %v", i, cart, err)
		}
		inv, err := q.GetInvoiceByID(ctx, sale.InvoiceID)
		if err != nil {
			t.Fatal(err)
		}
		lines, err := q.ListInvoiceProducts(ctx, sale.InvoiceID)
		if err != nil {
			t.Fatal(err)
		}
		var shares, totals int64
		for _, l := range lines {
			shares += l.InvoiceDiscount
			totals += l.LineTotal
		}
		before := inv.DiscountedTotal + inv.InvoiceDiscount
		rewardCents := referralRewardCents(db.ReferralReward{Percent: rewardPercent, MaxCents: maxCents}, before, "half_up")
		switch {
		case quote.GrandTotalCents != sale.GrandTotal || sale.GrandTotal != inv.GrandTotal:
			t.Fatalf("case %d: quote %d, charged %d, stored %d", i, quote.GrandTotalCents, sale.GrandTotal, inv.GrandTotal)
		case quote.InvoiceDiscountCents != inv.InvoiceDiscount || shares != inv.InvoiceDiscount || totals != inv.DiscountedTotal:
			t.Fatalf("case %d: quote %d, stored %d, shares %d, totals %d vs %d", i, quote.InvoiceDiscountCents, inv.InvoiceDiscount, shares, totals, inv.DiscountedTotal)
		case inv.InvoiceDiscount < rewardCents:
			t.Fatalf("case %d: took %d, the reward alone was %d", i, inv.InvoiceDiscount, rewardCents)
		case (inv.ReferralRewardUid == uid) != (sale.InvoiceDiscount.RewardUID == uid):
			t.Fatalf("case %d: stored reward %q, sale %q", i, inv.ReferralRewardUid, sale.InvoiceDiscount.RewardUID)
		}
		if inv.ReferralRewardUid == uid && inv.InvoiceDiscount != rewardCents {
			t.Fatalf("case %d: the reward won with %d, it is %d", i, inv.InvoiceDiscount, rewardCents)
		}
		if inv.ReferralRewardUid != uid {
			// Lost or tied: still there; clear it for the next case.
			if _, ok, _ := usableReferralReward(ctx, q, shop.owner.ID, time.Now()); !ok && rewardCents > 0 {
				t.Fatalf("case %d: a reward that lost was used up", i)
			}
			shop.giveReward(t, syncedReferralReward{UID: uid, OwnerClientUID: "uid-owner", Percent: rewardPercent, Status: "cancelled"})
		}
	}
}

func TestReferralCodeOnTheReceipt(t *testing.T) {
	shop := setUpReferralShop(t)
	program, err := referralProgram(context.Background(), shop.store.Queries)
	if err != nil {
		t.Fatal(err)
	}
	code, note := referralReceiptNote(program, shop.owner)
	if code != shop.ownerCode || !strings.Contains(note, "15% off your next visit") {
		t.Fatalf("code %q note %q", code, note)
	}
	out := string(buildESCPosReceipt(printReceiptData{ReceiptNo: "SA-1", ReferralCode: code, ReferralNote: note}))
	if !strings.Contains(out, "YOUR CODE: "+shop.ownerCode) || !strings.Contains(out, "you get 15% off") {
		t.Fatalf("receipt has no code:\n%s", out)
	}
	if strings.ContainsAny(note, "—–·") {
		t.Fatalf("note has a long dash or dot: %q", note)
	}
	if out := string(buildESCPosReceipt(printReceiptData{ReceiptNo: "SA-1"})); strings.Contains(out, "YOUR CODE") {
		t.Fatal("a receipt without a code printed the block")
	}
	walkIn := shop.owner
	walkIn.Phone = "00000000"
	if code, _ := referralReceiptNote(program, walkIn); code != "" {
		t.Fatal("the walk-in got a code")
	}
	if code, _ := referralReceiptNote(db.ReferralProgram{RewardPercent: 15}, shop.owner); code != "" {
		t.Fatal("referrals off still printed a code")
	}
	if code, _ := shop.app.receiptReferral(shop.owner.ID); code != shop.ownerCode {
		t.Fatalf("receiptReferral = %q", code)
	}
}

// A client pull stores the code; a central without codes leaves it.
func TestClientSyncKeepsTheReferralCode(t *testing.T) {
	ctx := context.Background()
	shop := setUpReferralShop(t)
	if err := applySyncedClient(ctx, shop.store.Queries, syncedClient{UID: "uid-owner", Name: "Rami K", Phone: "+96171000001"}); err != nil {
		t.Fatal(err)
	}
	got, err := shop.store.GetClientByID(ctx, shop.owner.ID)
	if err != nil || got.ReferralCode != shop.ownerCode || got.Name != "Rami K" {
		t.Fatalf("got %+v %v", got, err)
	}
}

// Central's socket messages about referrals pull the right entities, and
// both are pulled entities at all.
func TestReferralSocketMessagesPull(t *testing.T) {
	for msg, want := range map[string]string{"referrals_updated": "referral_rewards", "referral_program_updated": "referral_program"} {
		if got := wsMessageEntity[msg]; got != want {
			t.Errorf("%s pulls %q, want %q", msg, got, want)
		}
		if !slices.Contains(pullEntities, want) {
			t.Errorf("%s isn't pulled", want)
		}
	}
}
