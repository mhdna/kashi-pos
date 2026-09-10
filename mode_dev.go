//go:build dev

package main

// isDevMode is true when the binary was compiled by `wails dev`, which passes
// the `dev` build tag. Used to run windowed instead of fullscreen during
// development.
const isDevMode = true
