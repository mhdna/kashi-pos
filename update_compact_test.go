package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

// The till asks for the smallest download: a patch from the exe it runs
// (here the test binary), else the build compressed, else the plain exe.
// Whatever it gets must rebuild exactly the signed build, or it moves on.
func TestCheckAndDownload_PatchThenZstdThenPlain(t *testing.T) {
	oldVersion := appVersion
	appVersion = "1.0.8"
	t.Cleanup(func() { appVersion = oldVersion })

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	base, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	// The "new build": the running one with a change in the middle, as a
	// release mostly is.
	content := append(append(append([]byte{}, base[:len(base)/2]...), []byte("a new feature")...), base[len(base)/2:]...)
	m := signedManifest(t, content, "1.0.9")

	patchEnc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBestCompression),
		zstd.WithWindowSize(64<<20), zstd.WithEncoderDictRaw(0, base))
	if err != nil {
		t.Fatal(err)
	}
	goodPatch := patchEnc.EncodeAll(content, nil)
	fullEnc, _ := zstd.NewWriter(nil)
	goodPacked := fullEnc.EncodeAll(content, nil)
	if len(goodPatch) > len(goodPacked)/4 {
		t.Fatalf("patch %d bytes vs compressed %d: it should be far smaller", len(goodPatch), len(goodPacked))
	}

	cases := []struct {
		name          string
		patch, packed []byte
		want          string // which download staged it
	}{
		{"patch", goodPatch, goodPacked, "patch"},
		{"broken patch falls back to zstd", []byte("not a patch"), goodPacked, "zstd"},
		{"broken zstd falls back to plain", []byte("not a patch"), []byte("not zstd"), "plain"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("LOCALAPPDATA", t.TempDir())
			var asked, served []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/branch/app_update":
					asked = append(asked, r.URL.RawQuery)
					offer := *m
					offer.Encoding, offer.DownloadBytes = "zstd", int64(len(tc.packed))
					offer.Patch = &appUpdatePatchInfo{ID: 5, BaseSha256: r.URL.Query().Get("have"), SizeBytes: int64(len(tc.patch))}
					_ = json.NewEncoder(w).Encode(map[string]any{"release": offer})
				case "/branch/app_update/patch/5":
					served = append(served, "patch")
					http.ServeContent(w, r, "p", time.Now(), bytes.NewReader(tc.patch))
				case "/branch/app_update/download/7":
					name := "plain"
					body := content
					if r.URL.Query().Get("encoding") == "zstd" {
						name, body = "zstd", tc.packed
					}
					served = append(served, name)
					http.ServeContent(w, r, "x", time.Now(), bytes.NewReader(body))
				default:
				}
			}))
			defer srv.Close()

			a := &App{ctx: context.Background(), ready: make(chan struct{})}
			if err := a.checkAndDownload(context.Background(), newCentralClient(srv.URL, "CN", "key")); err != nil {
				t.Fatal(err)
			}
			if len(asked) != 1 || !strings.Contains(asked[0], "accept=zstd%2Cpatch") || !strings.Contains(asked[0], "have="+ownExeSha256()) {
				t.Fatalf("asked %v: want accept=zstd,patch and have=<this exe>", asked)
			}
			s := a.updateStateSnapshot()
			staged, err := os.ReadFile(s.StagedPath)
			if err != nil || !bytes.Equal(staged, content) {
				t.Fatalf("staged %q (%v): not the release", s.StagedPath, err)
			}
			if served[len(served)-1] != tc.want {
				t.Fatalf("downloads %v, want the build staged from the %s one", served, tc.want)
			}
		})
	}
}
