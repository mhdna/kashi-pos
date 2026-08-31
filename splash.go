package main

import _ "embed"

// splashLogoPNG is the brand mark shown on the native pre-webview splash
// (Windows only) and is decoded once at startup. The same asset is inlined
// as a data URI in frontend/index.html for the in-webview loader.
//
//go:embed frontend/src/assets/app-logo.png
var splashLogoPNG []byte
