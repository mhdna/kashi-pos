//go:build !dev

package main

// isDevMode is false for `wails build` (production) binaries, which run
// fullscreen with a fixed window.
const isDevMode = false
