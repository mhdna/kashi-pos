//go:build !compact

package main

// appFlavor names this build variant to kashi, so a till is only ever sent
// an update built the same way (see updater.go).
const appFlavor = "standard"
