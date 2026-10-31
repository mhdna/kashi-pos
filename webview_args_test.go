package main

import "testing"

func envOf(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := values[key]
		return v, ok
	}
}

func TestChooseWebViewArgs(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		want    string
		wantSet bool
	}{
		{"defaults", map[string]string{}, defaultWebViewArgs, true},
		{"override", map[string]string{"GB_CLOUD_WEBVIEW_ARGS": "--disable-smooth-scrolling"}, "--disable-smooth-scrolling", true},
		{"empty override means no extra flags", map[string]string{"GB_CLOUD_WEBVIEW_ARGS": ""}, "", true},
		{"a preset WebView2 variable is left alone", map[string]string{"WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS": "--x", "GB_CLOUD_WEBVIEW_ARGS": "--y"}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, set := chooseWebViewArgs(envOf(tc.env))
			if got != tc.want || set != tc.wantSet {
				t.Fatalf("got (%q, %v), want (%q, %v)", got, set, tc.want, tc.wantSet)
			}
		})
	}
}
