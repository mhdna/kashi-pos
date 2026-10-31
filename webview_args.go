package main

// defaultWebViewArgs are the extra browser arguments the Windows till has
// always run WebView2 with.
const defaultWebViewArgs = "--disable-smooth-scrolling --disable-gpu-rasterization --disable-gpu-vsync"

// webViewArgsEnv overrides defaultWebViewArgs for one till, so the flags can be
// compared on real hardware without a rebuild. An empty value means no extra
// arguments.
const webViewArgsEnv = "GB_CLOUD_WEBVIEW_ARGS"

// chooseWebViewArgs picks the arguments WebView2 starts with. A
// WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS already set in the environment wins
// (set reports false and nothing is changed); then GB_CLOUD_WEBVIEW_ARGS, if
// present; then the defaults.
func chooseWebViewArgs(lookup func(string) (string, bool)) (args string, set bool) {
	if _, preset := lookup("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS"); preset {
		return "", false
	}
	if override, ok := lookup(webViewArgsEnv); ok {
		return override, true
	}
	return defaultWebViewArgs, true
}
