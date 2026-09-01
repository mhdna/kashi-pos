package main

import (
	"strings"
	"testing"
	"time"
)

func TestVerifyPINRoles(t *testing.T) {
	app := NewApp()

	cashier, err := app.VerifyPIN("1478")
	if err != nil || cashier["role"] != "cashier" {
		t.Fatalf("cashier PIN returned role %v, error %v", cashier["role"], err)
	}

	admin, err := app.VerifyAdminPassword("1963")
	if err != nil || admin["role"] != "admin" {
		t.Fatalf("admin password returned role %v, error %v", admin["role"], err)
	}

	// The admin password is no longer a PIN and the old admin PIN is now just
	// another wrong PIN.
	if _, err := app.VerifyPIN("1963"); err == nil {
		t.Fatal("old admin PIN 1963 should no longer authenticate via VerifyPIN")
	}
}

func TestVerifyPINLocksAfterThreeFailures(t *testing.T) {
	app := NewApp()

	for attempt := 1; attempt <= 2; attempt++ {
		if _, err := app.VerifyPIN("9999"); err == nil || err.Error() != "invalid PIN" {
			t.Fatalf("attempt %d returned %v", attempt, err)
		}
	}
	if _, err := app.VerifyPIN("9999"); err == nil || !strings.HasPrefix(err.Error(), "locked:") {
		t.Fatalf("third attempt should lock login, got %v", err)
	}
	if _, err := app.VerifyPIN("1478"); err == nil || !strings.HasPrefix(err.Error(), "locked:") {
		t.Fatalf("correct PIN should be rejected during lockout, got %v", err)
	}

	app.pinLockedUntil = time.Now().Add(-time.Second)
	if _, err := app.VerifyPIN("1478"); err != nil {
		t.Fatalf("correct PIN should work after lockout: %v", err)
	}
}

func TestVerifyPINLockoutEscalates(t *testing.T) {
	app := NewApp()

	lockAndExpire := func(round int) time.Duration {
		for attempt := 1; attempt <= 3; attempt++ {
			_, err := app.VerifyPIN("9999")
			if err == nil {
				t.Fatalf("round %d attempt %d: expected an error", round, attempt)
			}
			if attempt < 3 && err.Error() != "invalid PIN" {
				t.Fatalf("round %d attempt %d: expected invalid PIN, got %v", round, attempt, err)
			}
			if attempt == 3 && !strings.HasPrefix(err.Error(), "locked:") {
				t.Fatalf("round %d attempt %d: expected lockout, got %v", round, attempt, err)
			}
		}
		duration := time.Until(app.pinLockedUntil)
		app.pinLockedUntil = time.Now().Add(-time.Second)
		return duration
	}

	first := lockAndExpire(1)
	second := lockAndExpire(2)
	if second <= first {
		t.Fatalf("second lockout (%v) should be longer than the first (%v)", second, first)
	}

	third := lockAndExpire(3)
	if third <= second {
		t.Fatalf("third lockout (%v) should be longer than the second (%v)", third, second)
	}
}

func TestUnlockPINLockoutClearsState(t *testing.T) {
	app := NewApp()
	for attempt := 1; attempt <= 3; attempt++ {
		if _, err := app.VerifyPIN("9999"); err == nil {
			t.Fatalf("attempt %d: expected an error", attempt)
		}
	}
	if !time.Now().Before(app.pinLockedUntil) {
		t.Fatalf("expected app to be locked out")
	}

	app.pinMu.Lock()
	app.unlockPINLockout()
	app.pinMu.Unlock()

	if _, err := app.VerifyPIN("1478"); err != nil {
		t.Fatalf("correct PIN should work immediately after unlock: %v", err)
	}
	if app.pinLockoutLevel != 0 {
		t.Fatalf("unlock should reset the escalation level, got %d", app.pinLockoutLevel)
	}
}
