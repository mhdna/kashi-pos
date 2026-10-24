package main

import (
	"encoding/json"
	"testing"
)

// TestTransferShiftBalanceTakesNegativeSourceFurtherNegativeForAdmin is the
// LBP-change case: the source is already negative and the destination should
// still gain the amount, with the source going further below zero.
func TestTransferShiftBalanceTakesNegativeSourceFurtherNegativeForAdmin(t *testing.T) {
	fixture := newBalanceSwapFixture(t, -1000)
	fixture.app.currentRole = "admin"

	preview, err := fixture.app.PreviewShiftBalanceReplacement(fixture.shiftID, fixture.sourceID, fixture.destinationID, 2000, true)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Valid || preview.SourceRemainingCents != -3000 || preview.DestinationReceivesCents != 2000 {
		t.Fatalf("transfer preview = %+v", preview)
	}
	if err := fixture.app.ReplaceShiftBalance(fixture.shiftID, fixture.sourceID, fixture.destinationID, 2000, true); err != nil {
		t.Fatal(err)
	}
	balances := fixtureBalances(t, fixture)
	if balances[fixture.sourceID] != -3000 || balances[fixture.destinationID] != 2000 {
		t.Fatalf("transfer balances = source %d, destination %d", balances[fixture.sourceID], balances[fixture.destinationID])
	}
}

func TestTransferShiftBalanceBelowZeroIsAdminOnly(t *testing.T) {
	fixture := newBalanceSwapFixture(t, -1000)
	fixture.app.currentRole = "cashier"

	preview, err := fixture.app.PreviewShiftBalanceReplacement(fixture.shiftID, fixture.sourceID, fixture.destinationID, 2000, true)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Valid {
		t.Fatal("non-admin preview of a below-zero transfer was valid")
	}
	if err := fixture.app.ReplaceShiftBalance(fixture.shiftID, fixture.sourceID, fixture.destinationID, 2000, true); err == nil {
		t.Fatal("non-admin below-zero transfer was accepted")
	}
	if replacementCount(t, fixture.database) != 0 {
		t.Fatal("rejected transfer wrote a replacement record")
	}
}

// A transfer that leaves the source at or above zero is just a swap, so a
// non-admin may make it.
func TestTransferShiftBalanceStayingAboveZeroIsAllowedForNonAdmin(t *testing.T) {
	fixture := newBalanceSwapFixture(t, 1000)
	fixture.app.currentRole = "cashier"
	if err := fixture.app.ReplaceShiftBalance(fixture.shiftID, fixture.sourceID, fixture.destinationID, 400, true); err != nil {
		t.Fatal(err)
	}
	balances := fixtureBalances(t, fixture)
	if balances[fixture.sourceID] != 600 || balances[fixture.destinationID] != 400 {
		t.Fatalf("transfer balances = source %d, destination %d", balances[fixture.sourceID], balances[fixture.destinationID])
	}
}

// kashi checks its own user's permission before queuing the command, so the
// till applies it to the open shift whoever is signed in there.
func TestReplaceShiftBalanceCommandTransfersOnOpenShift(t *testing.T) {
	fixture := newBalanceSwapFixture(t, -1000)
	fixture.app.currentRole = "cashier"
	payload, err := json.Marshal(replaceShiftBalancePayload{FromAccount: "Cash", ToAccount: "Card", AmountCents: 2000, AllowNegative: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.app.executeCommand(fixture.app.ctx, branchCommand{Type: "replace_shift_balance", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	balances := fixtureBalances(t, fixture)
	if balances[fixture.sourceID] != -3000 || balances[fixture.destinationID] != 2000 {
		t.Fatalf("command balances = source %d, destination %d", balances[fixture.sourceID], balances[fixture.destinationID])
	}

	unknown, err := json.Marshal(replaceShiftBalancePayload{FromAccount: "Nope", ToAccount: "Card", AmountCents: 100})
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.app.executeCommand(fixture.app.ctx, branchCommand{Type: "replace_shift_balance", Payload: unknown}); err == nil {
		t.Fatal("command with an unknown payment method was accepted")
	}
}
