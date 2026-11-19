package main

import (
	"context"
	"os"
	"strings"
	"testing"

	db "kashi-pos/db/sqlc"
)

// The reported case: Centro's till pulled the referral program on 1.1.161,
// before it knew who the program rewards, so after updating it kept 'owner'
// with its cursor past the program: the receipt said "you get 15% off your
// next visit" and the new customer got nothing. After 000147 the next pull
// fetches the program again (an empty cursor pulls it from the start).
func TestOldTillRefetchesReferralProgram(t *testing.T) {
	_, _, _, database := setupQuoteTestApp(t, 0, "half_up", "LBP")
	q := db.New(database)
	ctx := context.Background()
	for _, e := range []string{"referral_program", "clients"} {
		if err := q.SetSyncCursor(ctx, db.SetSyncCursorParams{Entity: e, Cursor: "2026-11-19T14:58:07Z"}); err != nil {
			t.Fatal(err)
		}
	}
	migration, err := os.ReadFile("db/migrations/000147_refetch_referral_program.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := database.Exec(string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	if c, err := q.GetSyncCursor(ctx, "referral_program"); err == nil && c != "" {
		t.Fatalf("the program's cursor kept: %q", c)
	}
	if c, _ := q.GetSyncCursor(ctx, "clients"); c == "" {
		t.Fatal("the clients cursor went too")
	}
	// What that pull brings: the program as central has it.
	if err := applySyncedReferralProgram(ctx, q, syncedReferralProgram{Enabled: true, RewardPercent: 15, RewardTo: referralRewardBoth, FriendPercent: 15}); err != nil {
		t.Fatal(err)
	}
	p, err := referralProgram(ctx, q)
	if err != nil || p.RewardTo != referralRewardBoth || p.FriendPercent != 15 {
		t.Fatalf("program: %+v %v", p, err)
	}
	if _, note := referralReceiptNote(p, db.Client{ReferralCode: ownerCode, Name: "Rami"}, receiptTexts{}, false); note == "" || !strings.Contains(note, "you both get 15%") {
		t.Fatalf("note: %q", note)
	}
}
