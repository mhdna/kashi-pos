//go:build !windows

package license

// Development machines (macOS, Linux) have no DPAPI; the fingerprint-keyed
// AES-GCM layer in seal.go is all there is. Tills only run on Windows.
func platformSeal(b []byte) ([]byte, error)   { return b, nil }
func platformUnseal(b []byte) ([]byte, error) { return b, nil }
