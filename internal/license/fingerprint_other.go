//go:build !windows

package license

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

// Fingerprint is a stub for non-Windows dev/build machines (the shipped
// product only ever runs on Windows). It is intentionally NOT tied to any
// real hardware identifier — it exists so the package, its tests, and the
// main app's build compile and run on macOS/Linux during development. It
// deliberately returns a stable-per-host value (hostname-derived) so
// licensing tests on this machine can exercise the full round trip.
func Fingerprint() (string, error) {
	host, err := os.Hostname()
	if err != nil {
		host = "dev-host"
	}
	sum := sha256.Sum256([]byte("non-windows-stub|" + host))
	return hex.EncodeToString(sum[:]), nil
}
