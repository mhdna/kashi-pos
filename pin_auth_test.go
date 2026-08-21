package main

import (
	"strings"
	"testing"
	"time"
)

func TestVerifyPINRoles(t *testing.T) {
	app := NewApp()

	admin, err := app.VerifyPIN("0000")
	if err != nil || admin["role"] != "admin" {
		t.Fatalf("admin PIN returned role %v, error %v", admin["role"], err)
	}

	cashier, err := app.VerifyPIN("1234")
	if err != nil || cashier["role"] != "cashier" {
		t.Fatalf("cashier PIN returned role %v, error %v", cashier["role"], err)
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
	if _, err := app.VerifyPIN("0000"); err == nil || !strings.HasPrefix(err.Error(), "locked:") {
		t.Fatalf("correct PIN should be rejected during lockout, got %v", err)
	}

	app.pinLockedUntil = time.Now().Add(-time.Second)
	if _, err := app.VerifyPIN("0000"); err != nil {
		t.Fatalf("correct PIN should work after lockout: %v", err)
	}
}
