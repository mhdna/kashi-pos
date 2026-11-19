//go:build windows

package license

import (
	"errors"
	"testing"
)

func resetFingerprintCache(t *testing.T) {
	t.Helper()
	fingerprintMu.Lock()
	fingerprintCached, fingerprintReady = "", false
	fingerprintMu.Unlock()
	original := computeFingerprintFn
	t.Cleanup(func() {
		fingerprintMu.Lock()
		fingerprintCached, fingerprintReady = "", false
		fingerprintMu.Unlock()
		computeFingerprintFn = original
	})
}

// TestFingerprintRetriesAfterTransientFailure guards against a real bug: the
// original sync.Once-based cache remembered a WMI read failure forever, so a
// single transient hiccup (WMI service restarting after a Windows update,
// momentary AV interference) permanently reported an invalid license for the
// rest of the process's life. A failure must not be cached - only a success.
func TestFingerprintRetriesAfterTransientFailure(t *testing.T) {
	resetFingerprintCache(t)

	calls := 0
	computeFingerprintFn = func() (string, error) {
		calls++
		if calls == 1 {
			return "", errors.New("simulated transient WMI failure")
		}
		return "stable-fingerprint", nil
	}

	if _, err := Fingerprint(); err == nil {
		t.Fatal("expected the first call to surface the simulated failure")
	}
	fp, err := Fingerprint()
	if err != nil {
		t.Fatalf("expected the second call to recover, got error: %v", err)
	}
	if fp != "stable-fingerprint" {
		t.Fatalf("fingerprint = %q, want %q", fp, "stable-fingerprint")
	}
	if calls != 2 {
		t.Fatalf("computeFingerprintFn called %d times, want 2 (no caching of the failure)", calls)
	}
}

// TestFingerprintCachesOnlyAfterSuccess ensures a successful computation is
// still cached for the rest of the process's life (the original perf reason
// for caching at all - GetLicenseStatus is invoked on every route change).
func TestFingerprintCachesOnlyAfterSuccess(t *testing.T) {
	resetFingerprintCache(t)

	calls := 0
	computeFingerprintFn = func() (string, error) {
		calls++
		return "cached-fingerprint", nil
	}

	for i := range 3 {
		fp, err := Fingerprint()
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		if fp != "cached-fingerprint" {
			t.Fatalf("call %d: fingerprint = %q, want %q", i, fp, "cached-fingerprint")
		}
	}
	if calls != 1 {
		t.Fatalf("computeFingerprintFn called %d times, want 1 (successful result must be cached)", calls)
	}
}

// TestFingerprintFromValuesRejectsBlankIdentifier guards the empty-WMI-value
// case: Get-CimInstance can exit 0 but return a blank property (seen after
// certain driver/firmware updates). Hashing a blank value in produces a
// validly-different-but-wrong fingerprint that looks exactly like tampering
// to the user instead of a retryable read failure.
func TestFingerprintFromValuesRejectsBlankIdentifier(t *testing.T) {
	base := map[string]string{"board": "BOARD1", "bios": "BIOS1", "cpu": "CPU1"}

	if _, err := fingerprintFromValues(base); err != nil {
		t.Fatalf("expected all-present values to succeed, got: %v", err)
	}

	for _, blankKey := range []string{"board", "bios", "cpu"} {
		values := map[string]string{"board": "BOARD1", "bios": "BIOS1", "cpu": "CPU1"}
		values[blankKey] = "  "
		if _, err := fingerprintFromValues(values); err == nil {
			t.Fatalf("expected a blank %q to be rejected as a read failure, not hashed in", blankKey)
		}
	}
}

// TestFingerprintFromValuesFallsBackWhenPrimaryIsBlank guards the common
// real-world case (e.g. many Lenovo boards never set
// Win32_BaseBoard.SerialNumber): a blank primary identifier should be
// substituted with its fallback property rather than failing outright.
func TestFingerprintFromValuesFallsBackWhenPrimaryIsBlank(t *testing.T) {
	withFallback := map[string]string{
		"board": "", "board_fallback": "BOARD-PRODUCT",
		"bios": "BIOS1", "cpu": "CPU1",
	}
	fp, err := fingerprintFromValues(withFallback)
	if err != nil {
		t.Fatalf("expected fallback to resolve the blank primary, got error: %v", err)
	}

	want, err := fingerprintFromValues(map[string]string{"board": "BOARD-PRODUCT", "bios": "BIOS1", "cpu": "CPU1"})
	if err != nil {
		t.Fatal(err)
	}
	if fp != want {
		t.Fatalf("fingerprint using fallback value = %q, want it to match hashing the fallback directly (%q)", fp, want)
	}
}

// TestFingerprintFromValuesRejectsBlankPrimaryAndFallback ensures a slot with
// neither primary nor fallback populated still fails loudly instead of
// hashing in an empty string.
func TestFingerprintFromValuesRejectsBlankPrimaryAndFallback(t *testing.T) {
	values := map[string]string{"board": "  ", "board_fallback": " ", "bios": "BIOS1", "cpu": "CPU1"}
	if _, err := fingerprintFromValues(values); err == nil {
		t.Fatal("expected a blank primary with a blank fallback to be rejected as a read failure")
	}
}

// TestFingerprintFromValuesIsDeterministic guards against a regression where
// the same identifiers produce different fingerprints across calls.
func TestFingerprintFromValuesIsDeterministic(t *testing.T) {
	values := map[string]string{"board": "BOARD1", "bios": "BIOS1", "cpu": "CPU1"}
	first, err := fingerprintFromValues(values)
	if err != nil {
		t.Fatal(err)
	}
	second, err := fingerprintFromValues(values)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("fingerprintFromValues is not deterministic: %q vs %q", first, second)
	}
}
