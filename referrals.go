package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

// Referrals (central's referrals.go decides everything; this is the till's
// part). Every client has a code central gave them, printed on their
// receipts. A new customer's first sale can name the code of the client who
// sent them. Central then gives that client a reward: a percent off one
// later sale. The friend gets nothing.
//
// A reward is used like an invoice discount (invoice_discounts.go), and
// competes with them: whichever takes more off wins, and the reward is only
// used up when it is the one that wins.

// referralAlphabet holds the characters a code is made of: none that read
// like another (no 0/O, 1/I/L, 2/Z, 5/S, 8/B, U/V). Its size, 23, is prime,
// which is what lets the check character catch every single wrong character
// and every swap of two neighbours. Central uses the same alphabet.
const referralAlphabet = "ACDEFGHJKMNPQRTWXY34679"

// referralCodeLength is 5 random characters and the check character.
const referralCodeLength = 6

// normalizeReferralCode is a code as typed with spaces and dashes left out,
// in capitals.
func normalizeReferralCode(code string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(code) {
		if r == ' ' || r == '-' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// referralCheckChar is the check character of a code's first characters:
// their positions in the alphabet weighted 1, 2, 3 ..., summed, mod 23.
func referralCheckChar(body string) (byte, bool) {
	sum := 0
	for i := 0; i < len(body); i++ {
		v := strings.IndexByte(referralAlphabet, body[i])
		if v < 0 {
			return 0, false
		}
		sum += (i + 1) * v
	}
	return referralAlphabet[sum%len(referralAlphabet)], true
}

// validReferralCode reports whether a normalized code has the right length,
// only alphabet characters and a matching check character.
func validReferralCode(code string) bool {
	if len(code) != referralCodeLength {
		return false
	}
	want, ok := referralCheckChar(code[:referralCodeLength-1])
	return ok && code[referralCodeLength-1] == want
}

// syncedReferralProgram is central's rules as the till pulls them.
type syncedReferralProgram struct {
	Enabled        bool  `json:"enabled"`
	RewardPercent  int64 `json:"reward_percent"`
	MinFriendSpend int64 `json:"min_friend_spend"`
	// RewardTo is who a code rewards: owner (as before), friend or both;
	// FriendPercent the new customer's percent off the sale they use it on.
	RewardTo      string `json:"reward_to"`
	FriendPercent int64  `json:"friend_percent"`
}

func applySyncedReferralProgram(ctx context.Context, q *db.Queries, p syncedReferralProgram) error {
	to := p.RewardTo
	if to != referralRewardFriend && to != referralRewardBoth {
		to = referralRewardOwner
	}
	return q.UpsertReferralProgram(ctx, db.UpsertReferralProgramParams{
		Enabled: boolToInt64(p.Enabled), RewardPercent: p.RewardPercent, MinFriendSpend: p.MinFriendSpend,
		RewardTo: to, FriendPercent: p.FriendPercent,
	})
}

// Who a referral code rewards.
const (
	referralRewardOwner  = "owner"
	referralRewardFriend = "friend"
	referralRewardBoth   = "both"
)

// referralRewardsFriend says whether the new customer gets the program's
// friend percent off the sale they use a code on.
func referralRewardsFriend(p db.ReferralProgram) bool {
	return p.Enabled != 0 && p.FriendPercent > 0 && (p.RewardTo == referralRewardFriend || p.RewardTo == referralRewardBoth)
}

// referralPickPrefix marks the sale's referral code in its picked deals
// ("referral:K7M4QH"): the quote, the payment check and checkout all carry
// the picks, so the new customer's discount is priced the same by all three.
const referralPickPrefix = "referral:"

// referralFromPicks is the code among the picks (normalized), "" for none.
func referralFromPicks(picked []string) string {
	for _, p := range picked {
		if strings.HasPrefix(p, referralPickPrefix) {
			return normalizeReferralCode(strings.TrimPrefix(p, referralPickPrefix))
		}
	}
	return ""
}

// withReferralPick is the picks with any referral code taken out and the
// sale's own code (if any) put in.
func withReferralPick(picked []string, code string) []string {
	out := make([]string, 0, len(picked)+1)
	for _, p := range picked {
		if !strings.HasPrefix(p, referralPickPrefix) {
			out = append(out, p)
		}
	}
	if code = normalizeReferralCode(code); code != "" {
		out = append(out, referralPickPrefix+code)
	}
	return out
}

// friendReferralLabel names the new customer's discount on the invoice.
func friendReferralLabel(percent int64) string { return fmt.Sprintf("Referral %d%%", percent) }

// syncedReferralReward is a reward as central has it.
type syncedReferralReward struct {
	UID            string     `json:"uid"`
	OwnerClientUID string     `json:"owner_client_uid"`
	Percent        int64      `json:"percent"`
	MaxCents       int64      `json:"max_cents"`
	Status         string     `json:"status"`
	AvailableFrom  *time.Time `json:"available_from"`
	ExpiresAt      *time.Time `json:"expires_at"`
}

func applySyncedReferralReward(ctx context.Context, q *db.Queries, r syncedReferralReward) error {
	if r.UID == "" || r.OwnerClientUID == "" {
		return fmt.Errorf("referral reward without a uid or an owner")
	}
	return q.UpsertReferralReward(ctx, db.UpsertReferralRewardParams{
		Uid: r.UID, OwnerClientUid: r.OwnerClientUID, Percent: r.Percent, MaxCents: r.MaxCents,
		Status: r.Status, AvailableFrom: rfc3339OrEmpty(r.AvailableFrom), ExpiresAt: rfc3339OrEmpty(r.ExpiresAt),
	})
}

// referralProgram is central's rules, or referrals off when central never
// sent any.
func referralProgram(ctx context.Context, q *db.Queries) (db.ReferralProgram, error) {
	p, err := q.GetReferralProgram(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return db.ReferralProgram{ID: 1}, nil
	}
	return p, err
}

// referralRewardUsable: a reward is usable from available_from (when set)
// until expires_at, the first moment it is over. A date the till can't
// read keeps it out.
func referralRewardUsable(r db.ReferralReward, now time.Time) bool {
	return invoiceDiscountInForce(db.InvoiceDiscount{ValidFrom: r.AvailableFrom, ValidTo: r.ExpiresAt}, now)
}

// referralRewardCents is what a reward takes off a total: its percent,
// no more than its cap when it has one, and never more than the total.
func referralRewardCents(r db.ReferralReward, total int64, roundingMode string) int64 {
	if total <= 0 || r.Percent <= 0 {
		return 0
	}
	cents := percentOf(total, r.Percent, roundingMode)
	if r.MaxCents > 0 {
		cents = min(cents, r.MaxCents)
	}
	return min(cents, total)
}

// referralRewardLabel names the receipt row of a sale that used a reward.
func referralRewardLabel(percent int64) string {
	return fmt.Sprintf("Referral reward %d%%", percent)
}

// usableReferralReward is the buyer's reward the till would use now: the
// first to expire among those usable. ok is false for a walk-in, a client
// central doesn't know yet, or a client with none.
func usableReferralReward(ctx context.Context, q *db.Queries, clientID int64, now time.Time) (db.ReferralReward, bool, error) {
	if clientID <= 0 {
		return db.ReferralReward{}, false, nil
	}
	client, err := q.GetClientByID(ctx, clientID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.ReferralReward{}, false, nil
	}
	if err != nil {
		return db.ReferralReward{}, false, err
	}
	if !client.Uid.Valid || client.Uid.String == "" || isWalkInClient(client) {
		return db.ReferralReward{}, false, nil
	}
	rewards, err := q.ListUsableReferralRewards(ctx, client.Uid.String)
	if err != nil {
		return db.ReferralReward{}, false, err
	}
	for _, r := range rewards {
		if referralRewardUsable(r, now) {
			return r, true, nil
		}
	}
	return db.ReferralReward{}, false, nil
}

// ReferralCodeCheck is what the cart shows for a code typed in: whose it
// is.
type ReferralCodeCheck struct {
	Code      string `json:"code"`
	OwnerName string `json:"ownerName"`
}

var (
	errReferralsOff          = errors.New("Referral codes are switched off")
	errReferralCodeInvalid   = errors.New("This isn't a referral code. Check it for a typo")
	errReferralCodeUnknown   = errors.New("No client has this referral code")
	errReferralNeedsClient   = errors.New("Pick the customer first: a referral code is for a new customer")
	errReferralOwnCode       = errors.New("This is the customer's own code")
	errReferralNotNewClient  = errors.New("A referral code is only for a new customer's first purchase")
	errReferralCodeAndReward = errors.New("This customer has a referral reward, so they aren't new")
)

// checkReferralCode decides whether code can go on a sale to clientID, and
// returns it normalized with its owner. What central checks again when the
// sale arrives (the customer's purchases at every branch) it can only
// check here for this till.
func checkReferralCode(ctx context.Context, q *db.Queries, code string, clientID int64) (string, db.Client, error) {
	program, err := referralProgram(ctx, q)
	if err != nil {
		return "", db.Client{}, err
	}
	if program.Enabled == 0 {
		return "", db.Client{}, errReferralsOff
	}
	code = normalizeReferralCode(code)
	if !validReferralCode(code) {
		return "", db.Client{}, errReferralCodeInvalid
	}
	owner, err := q.GetClientByReferralCode(ctx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return "", db.Client{}, errReferralCodeUnknown
	}
	if err != nil {
		return "", db.Client{}, err
	}
	if clientID <= 0 {
		return "", db.Client{}, errReferralNeedsClient
	}
	buyer, err := q.GetClientByID(ctx, clientID)
	if err != nil {
		return "", db.Client{}, fmt.Errorf("client: %w", err)
	}
	if isWalkInClient(buyer) {
		return "", db.Client{}, errReferralNeedsClient
	}
	if buyer.ID == owner.ID {
		return "", db.Client{}, errReferralOwnCode
	}
	hasSale, err := q.ClientHasSale(ctx, buyer.ID)
	if err != nil {
		return "", db.Client{}, err
	}
	if hasSale {
		return "", db.Client{}, errReferralNotNewClient
	}
	if _, has, err := usableReferralReward(ctx, q, buyer.ID, time.Now()); err != nil {
		return "", db.Client{}, err
	} else if has {
		return "", db.Client{}, errReferralCodeAndReward
	}
	return code, owner, nil
}

// CheckReferralCode is the cart's check of a code as the cashier types it.
func (a *App) CheckReferralCode(code string, clientID int64) (ReferralCodeCheck, error) {
	if a.store == nil {
		return ReferralCodeCheck{}, fmt.Errorf("database not initialized")
	}
	normalized, owner, err := checkReferralCode(a.ctx, a.store.Queries, code, clientID)
	if err != nil {
		return ReferralCodeCheck{}, err
	}
	return ReferralCodeCheck{Code: normalized, OwnerName: owner.Name}, nil
}

// ReferralInfo is what the sale screen needs to know about referrals: if
// they're on, and the picked client's own code and usable reward.
type ReferralInfo struct {
	Enabled       bool   `json:"enabled"`
	RewardPercent int64  `json:"rewardPercent"`
	Code          string `json:"code"`
	// The client's usable reward, 0 when none.
	RewardOfferPercent int64 `json:"rewardOfferPercent"`
	// FriendPercent is what a new customer takes off the sale they use a
	// code on, 0 when the program doesn't reward them.
	FriendPercent int64 `json:"friendPercent"`
}

// GetReferralInfo answers for clientID (0 for none).
func (a *App) GetReferralInfo(clientID int64) (ReferralInfo, error) {
	var info ReferralInfo
	if a.store == nil {
		return info, fmt.Errorf("database not initialized")
	}
	program, err := referralProgram(a.ctx, a.store.Queries)
	if err != nil {
		return info, err
	}
	info.Enabled, info.RewardPercent = program.Enabled != 0, program.RewardPercent
	if referralRewardsFriend(program) {
		info.FriendPercent = program.FriendPercent
	}
	if clientID <= 0 {
		return info, nil
	}
	client, err := a.store.GetClientByID(a.ctx, clientID)
	if err != nil {
		return info, nil
	}
	if !isWalkInClient(client) {
		info.Code = client.ReferralCode
	}
	if reward, ok, err := usableReferralReward(a.ctx, a.store.Queries, clientID, time.Now()); err != nil {
		return info, err
	} else if ok {
		info.RewardOfferPercent = reward.Percent
	}
	return info, nil
}

// referralReceiptNote is the receipt's lines about the customer's code, or
// nothing when referrals are off or the customer has none.
// The wording is the branch's (receipt_texts.go), and the branch can leave
// the block out.
func referralReceiptNote(program db.ReferralProgram, client db.Client, texts receiptTexts) (code, note string) {
	if program.Enabled == 0 || program.RewardPercent <= 0 || client.ReferralCode == "" || isWalkInClient(client) || texts.hidden(receiptSectionReferral) {
		return "", ""
	}
	return client.ReferralCode, texts.referralNote(program.RewardPercent)
}
