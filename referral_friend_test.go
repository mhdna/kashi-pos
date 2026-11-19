package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	db "kashi-pos/db/sqlc"
)

const ownerCode = "K7M4QH"

// codeFor makes a valid referral code from n (a code is good once, so tests
// that sell many times need many).
func codeFor(n int) string {
	body := make([]byte, 5)
	for i := 4; i >= 0; i-- {
		body[i] = referralAlphabet[n%len(referralAlphabet)]
		n /= len(referralAlphabet)
	}
	check, _ := referralCheckChar(string(body))
	return string(body) + string(check)
}

func setFriendProgram(t *testing.T, shop dealShop, to string, friendPct, minSpend int64) {
	t.Helper()
	if err := applySyncedReferralProgram(context.Background(), shop.store.Queries, syncedReferralProgram{
		Enabled: true, RewardPercent: 15, MinFriendSpend: minSpend, RewardTo: to, FriendPercent: friendPct,
	}); err != nil {
		t.Fatal(err)
	}
}

func newShopClient(t *testing.T, shop dealShop, name, phone, code string) db.Client {
	t.Helper()
	ctx := context.Background()
	c, err := shop.store.CreateClient(ctx, db.CreateClientParams{Name: name, Phone: phone})
	if err != nil {
		t.Fatal(err)
	}
	if code != "" {
		if err := shop.store.SetClientReferralCode(ctx, db.SetClientReferralCodeParams{ReferralCode: code, ID: c.ID}); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

// sellWithCode quotes with the picks the screen sends (the code among them)
// and checks out with the code, as the till does.
func sellWithCode(t *testing.T, shop dealShop, buyer db.Client, code string, picked []string, codes ...string) (SalesInvoiceResult, InvoiceQuote, error) {
	t.Helper()
	var items []salesInvoiceItem
	at := map[string]int{}
	for _, c := range codes {
		if i, ok := at[c]; ok {
			items[i].Qty++
			continue
		}
		at[c] = len(items)
		items = append(items, salesInvoiceItem{Barcode: "bc-" + c, Qty: 1})
	}
	itemsJSON, _ := json.Marshal(items)
	screen := append([]string{}, picked...)
	if code != "" {
		screen = append(screen, referralPickPrefix+code)
	}
	raw, _ := json.Marshal(screen)
	quote, err := shop.app.QuoteInvoiceTotalPicked(string(itemsJSON), buyer.ID, string(raw))
	if err != nil {
		t.Fatal(err)
	}
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: shop.cashbox.ID, AmountCents: quote.GrandTotalCents}})
	result, err := shop.app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(itemsJSON), PaymentsJSON: string(payments), ClientID: buyer.ID, ReferralCode: code, PickedDiscounts: picked})
	return result, quote, err
}

// The reported case: with the program rewarding both, a new customer who
// gives a friend's code takes 10% off this very sale.
func TestNewCustomerGetsReferralDiscount(t *testing.T) {
	shop := setUpDealShop(t)
	if !validReferralCode(ownerCode) {
		t.Fatal("test code invalid")
	}
	newShopClient(t, shop, "Rami", "70111111", ownerCode)
	setFriendProgram(t, shop, referralRewardBoth, 10, 0)

	nadia := newShopClient(t, shop, "Nadia", "70222222", "")
	sale, quote, err := sellWithCode(t, shop, nadia, ownerCode, nil, "JACKET")
	if err != nil {
		t.Fatal(err)
	}
	if quote.InvoiceDiscountName != "Referral 10%" || quote.InvoiceDiscountCents != 1000 {
		t.Fatalf("quote: %q %d", quote.InvoiceDiscountName, quote.InvoiceDiscountCents)
	}
	if sale.InvoiceDiscount.Cents != 1000 || sale.GrandTotal != quote.GrandTotalCents {
		t.Fatalf("checkout: %+v total %d, quote %d", sale.InvoiceDiscount, sale.GrandTotal, quote.GrandTotalCents)
	}
	inv, err := shop.store.GetInvoiceByID(context.Background(), sale.InvoiceID)
	if err != nil || inv.InvoiceDiscountName != "Referral 10%" || inv.ReferralCode != ownerCode {
		t.Fatalf("stored %+v %v", inv, err)
	}

	// Her second purchase isn't a first one: the code (and its discount) is refused.
	if _, q2, err := sellWithCode(t, shop, nadia, ownerCode, nil, "JACKET"); err == nil || q2.InvoiceDiscountCents != 0 {
		t.Fatalf("second purchase: discount %d, err %v", q2.InvoiceDiscountCents, err)
	}
	// Walk-in: no discount.
	walkIn, err := shop.store.GetClientByPhone(context.Background(), walkInClientPhone)
	if err == nil {
		if _, q, _ := sellWithCode(t, shop, walkIn, ownerCode, nil, "JACKET"); q.InvoiceDiscountCents != 0 {
			t.Fatalf("walk-in got %d off", q.InvoiceDiscountCents)
		}
	}
	// The owner using their own code: none.
	rami, _ := shop.store.GetClientByReferralCode(context.Background(), ownerCode)
	if _, q, _ := sellWithCode(t, shop, rami, ownerCode, nil, "JACKET"); q.InvoiceDiscountCents != 0 {
		t.Fatalf("own code got %d off", q.InvoiceDiscountCents)
	}

	// Owner only (as before): the new customer pays in full. (A code is good
	// once: a fresh owner's.)
	setFriendProgram(t, shop, referralRewardOwner, 10, 0)
	newShopClient(t, shop, "Hadi", "70999001", "7EDXTH")
	sami := newShopClient(t, shop, "Sami", "70333333", "")
	if sale, _, err := sellWithCode(t, shop, sami, "7EDXTH", nil, "JACKET"); err != nil || sale.InvoiceDiscount.Cents != 0 {
		t.Fatalf("owner only: %+v %v", sale.InvoiceDiscount, err)
	}

	// Under the program's minimum spend: none.
	setFriendProgram(t, shop, referralRewardFriend, 10, 20000)
	newShopClient(t, shop, "Hiba", "70999002", "HQ4XJD")
	lina := newShopClient(t, shop, "Lina", "70444444", "")
	if _, q, _ := sellWithCode(t, shop, lina, "HQ4XJD", nil, "JACKET"); q.InvoiceDiscountCents != 0 {
		t.Fatalf("under the minimum got %d off", q.InvoiceDiscountCents)
	}
}

// The picks alone never give the discount: checkout follows the sale's own
// code, so a sale can't take it without the code being recorded.
func TestReferralDiscountNeedsTheSaleCode(t *testing.T) {
	shop := setUpDealShop(t)
	newShopClient(t, shop, "Rami", "70111111", ownerCode)
	setFriendProgram(t, shop, referralRewardFriend, 10, 0)
	buyer := newShopClient(t, shop, "Nour", "70555555", "")
	items, _ := json.Marshal([]salesInvoiceItem{{Barcode: "bc-JACKET", Qty: 1}})
	payments, _ := json.Marshal([]checkoutPayment{{AccountID: shop.cashbox.ID, AmountCents: 10000}})
	sale, err := shop.app.salesInvoiceTx(SalesInvoiceParams{ItemsJSON: string(items), PaymentsJSON: string(payments), ClientID: buyer.ID,
		PickedDiscounts: []string{referralPickPrefix + ownerCode}})
	if err != nil {
		t.Fatal(err)
	}
	if sale.InvoiceDiscount.Cents != 0 || sale.GrandTotal != 10000 {
		t.Fatalf("picks alone gave %+v", sale.InvoiceDiscount)
	}
}

// It competes with the deals, the biggest winning; where deals add up, it
// adds up with them.
func TestReferralDiscountWithDeals(t *testing.T) {
	shop := setUpDealShop(t)
	newShopClient(t, shop, "Rami", "70111111", ownerCode)
	setFriendProgram(t, shop, referralRewardBoth, 10, 0)
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "t150", Name: "20% above $150", MinTotal: 15000, Kind: invoiceDiscountPercent, Percent: 20})

	a := newShopClient(t, shop, "A", "70600001", "")
	if _, q, err := sellWithCode(t, shop, a, ownerCode, nil, "JACKET", "JACKET"); err != nil || q.InvoiceDiscountName != "20% above $150" {
		t.Fatalf("biggest: %q %v", q.InvoiceDiscountName, err)
	}
	setStack(t, shop, true)
	newShopClient(t, shop, "Owner B", "70600102", codeFor(1001))
	b := newShopClient(t, shop, "B", "70600002", "")
	_, q, err := sellWithCode(t, shop, b, codeFor(1001), nil, "JACKET", "JACKET")
	// 20000 -20% 16000 -10% 14400.
	if err != nil || q.InvoiceDiscountName != "20% above $150 + Referral 10%" || q.InvoiceDiscountCents != 5600 {
		t.Fatalf("added up: %q %d %v", q.InvoiceDiscountName, q.InvoiceDiscountCents, err)
	}
}

// Randomized: hundreds of new customers, with or without a code, in every
// program mode and with deals on or adding up: what the quote takes off is
// what checkout charges and stores, and the lines carry it to the cent.
func TestReferralDiscountRandomized(t *testing.T) {
	shop := setUpDealShop(t)
	newShopClient(t, shop, "Rami", "70111111", ownerCode)
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "t100", Name: "15% above $100", MinTotal: 10000, Kind: invoiceDiscountPercent, Percent: 15})
	addDiscount(t, shop, syncedInvoiceDiscount{UID: "stu", Name: "Students 15%", Kind: invoiceDiscountPercent, Percent: 15, CashierPicks: true})
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	modes := []string{referralRewardOwner, referralRewardFriend, referralRewardBoth}
	codes := []string{"A", "B", "C", "JACKET", "SOCKS"}
	ctx := context.Background()
	for run := 0; run < 150; run++ {
		setFriendProgram(t, shop, modes[rng.Intn(3)], int64(1+rng.Intn(30)), int64(rng.Intn(3))*5000)
		setStack(t, shop, rng.Intn(2) == 0)
		buyer := newShopClient(t, shop, fmt.Sprintf("C%d", run), fmt.Sprintf("71%06d", run), "")
		var cart []string
		for n := 1 + rng.Intn(4); n > 0; n-- {
			cart = append(cart, codes[rng.Intn(len(codes))])
		}
		code := ""
		if rng.Intn(3) != 0 {
			code = codeFor(5000 + run)
			newShopClient(t, shop, fmt.Sprintf("O%d", run), fmt.Sprintf("72%06d", run), code)
		}
		var picked []string
		if rng.Intn(2) == 0 {
			picked = []string{"stu"}
		}
		sale, quote, err := sellWithCode(t, shop, buyer, code, picked, cart...)
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if sale.GrandTotal != quote.GrandTotalCents || sale.InvoiceDiscount.Cents != quote.InvoiceDiscountCents || sale.InvoiceDiscount.Name != quote.InvoiceDiscountName {
			t.Fatalf("run %d: quote %d %q, checkout %d %+v", run, quote.GrandTotalCents, quote.InvoiceDiscountName, sale.GrandTotal, sale.InvoiceDiscount)
		}
		lines, err := shop.store.ListInvoiceProducts(ctx, sale.InvoiceID)
		if err != nil {
			t.Fatal(err)
		}
		var shares int64
		for _, l := range lines {
			shares += l.InvoiceDiscount
		}
		if shares != sale.InvoiceDiscount.Cents {
			t.Fatalf("run %d: lines carry %d of %d", run, shares, sale.InvoiceDiscount.Cents)
		}
		inv, err := shop.store.GetInvoiceByID(ctx, sale.InvoiceID)
		if err != nil || inv.GrandTotal != sale.GrandTotal || inv.InvoiceDiscountName != sale.InvoiceDiscount.Name {
			t.Fatalf("run %d: stored %+v %v", run, inv, err)
		}
	}
}

// The reported case: a code is good once. After a friend used it, it is no
// longer printed on its owner's receipts, and the till refuses it.
func TestReferralCodeGoodOnce(t *testing.T) {
	shop := setUpDealShop(t)
	newShopClient(t, shop, "Rami", "70111111", ownerCode)
	setFriendProgram(t, shop, referralRewardBoth, 15, 0)
	texts := receiptTexts{}
	ctx := context.Background()
	owner, _ := shop.store.GetClientByReferralCode(ctx, ownerCode)
	program, err := referralProgram(ctx, shop.store.Queries)
	if err != nil {
		t.Fatal(err)
	}
	used, err := referralCodeUsed(ctx, shop.store.Queries, owner)
	if err != nil || used {
		t.Fatalf("fresh code used=%v %v", used, err)
	}
	if code, _ := referralReceiptNote(program, owner, texts, used); code != ownerCode {
		t.Fatal("an unused code isn't printed")
	}
	friend := newShopClient(t, shop, "Nadia", "70222222", "")
	if _, _, err := sellWithCode(t, shop, friend, ownerCode, nil, "JACKET"); err != nil {
		t.Fatal(err)
	}
	if used, _ = referralCodeUsed(ctx, shop.store.Queries, owner); !used {
		t.Fatal("the sale with the code doesn't count as its use")
	}
	if code, _ := referralReceiptNote(program, owner, texts, used); code != "" {
		t.Fatal("a used code is still printed")
	}
	other := newShopClient(t, shop, "Sami", "70333333", "")
	if _, _, err := checkReferralCode(ctx, shop.store.Queries, ownerCode, other.ID); err != errReferralCodeUsed {
		t.Fatalf("second use: %v", err)
	}
	// A reward central sends for the owner counts too (a sale elsewhere).
	newShopClient(t, shop, "Lina", "70444444", "PCTT9T")
	owner2, _ := shop.store.GetClientByReferralCode(ctx, "PCTT9T")
	if !validReferralCode("PCTT9T") {
		t.Skip("sample code invalid")
	}
	if err := shop.store.UpsertReferralReward(ctx, db.UpsertReferralRewardParams{Uid: "rw-1", OwnerClientUid: owner2.Uid.String, Percent: 15, Status: "available"}); err != nil {
		t.Fatal(err)
	}
	if used, _ := referralCodeUsed(ctx, shop.store.Queries, owner2); !used {
		t.Fatal("a reward from central doesn't count as a use")
	}
}

// The note says what the program does: both, the new customer only, or the
// owner only; in English and Arabic.
func TestReferralNoteFollowsTheProgram(t *testing.T) {
	cases := map[string][2]string{
		referralRewardBoth:   {"you both get 15% off the whole invoice", "بتاخدوا انتو التنين حسم ١٥٪"},
		referralRewardFriend: {"they get 15% off their first invoice", "بياخد حسم ١٥٪ على أول فاتورة"},
		referralRewardOwner:  {"you get 15% off your next visit", "بتاخد انت حسم ١٥٪ على زيارتك الجاية"},
	}
	for to, want := range cases {
		texts := receiptTexts{ReferralRewardTo: to}
		en := texts.referralNote(15)
		if !strings.Contains(en, want[0]) {
			t.Fatalf("%s english: %q", to, en)
		}
		if ar := texts.arabicWords("referral_note", en); !strings.Contains(ar, want[1]) {
			t.Fatalf("%s arabic: %q", to, ar)
		}
	}
}

// The reported case: only the code's line prints white on black; the note
// under it stays as it was.
func TestReferralCodeLineWhiteOnBlack(t *testing.T) {
	data := receiptPreviewData(designBase(), receiptModeReferral)
	_, img, marks := drawnTexts(t, data, "classic", "A")
	var title, note receiptMark
	for _, m := range marks {
		if m.Kind == "line" && m.Part == "referral_title" {
			title = m
		}
		if m.Kind == "line" && m.Part == "referral_note" {
			note = m
		}
	}
	if title.Bottom <= title.Top || note.Bottom <= note.Top {
		t.Fatal("no referral lines")
	}
	darkShare := func(top, bottom int) float64 {
		dark, b := 0, img.Bounds()
		for y := top; y < bottom; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				if r, _, _, _ := img.At(x, y).RGBA(); r < 0x8000 {
					dark++
				}
			}
		}
		return float64(dark) / float64((bottom-top)*b.Dx())
	}
	if s := darkShare(title.Top, title.Bottom); s < 0.5 {
		t.Fatalf("the code's line is only %.0f%% black", s*100)
	}
	if s := darkShare(note.Top, note.Bottom); s > 0.3 {
		t.Fatalf("the note is %.0f%% black: it should stay on white", s*100)
	}
}
