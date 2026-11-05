package main

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

// A build compresses and patches with the zstd tool when it's installed and
// with the Go encoder otherwise; either way it unpacks to exactly the build,
// and a patch is a fraction of the compressed build.
func TestCompressAndPatch(t *testing.T) {
	base := make([]byte, 4<<20)
	if _, err := rand.Read(base); err != nil {
		t.Fatal(err)
	}
	newB := append(append(append([]byte{}, base[:2<<20]...), []byte("a new feature")...), base[2<<20:]...)
	newPath := filepath.Join(t.TempDir(), "GB-Cloud.exe")
	if err := os.WriteFile(newPath, newB, 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(t *testing.T) {
		packed, err := compressBuild(newPath, newB)
		if err != nil {
			t.Fatal(err)
		}
		back, err := unzstd(packed, nil)
		if err != nil || !bytes.Equal(back, newB) {
			t.Fatalf("compressed build doesn't unpack: %v", err)
		}
		patch, err := makePatch(base, newB, newPath)
		if err != nil {
			t.Fatal(err)
		}
		if len(patch) > len(newB)/50 {
			t.Fatalf("patch is %d bytes for a %d-byte build with one change", len(patch), len(newB))
		}
	}
	if zstdTool() != "" {
		t.Run("zstd tool", run)
	}
	t.Run("go encoder", func(t *testing.T) {
		t.Setenv("PATH", "")
		run(t)
	})
}

func TestWindowLog(t *testing.T) {
	for _, tc := range []struct {
		sizes []int
		want  int
	}{
		{[]int{28 << 20}, 27},
		{[]int{28 << 20, 29 << 20}, 27},
		{[]int{70 << 20, 70 << 20}, 28},
		{[]int{2 << 30}, 30},
	} {
		if got := windowLog(tc.sizes...); got != tc.want {
			t.Fatalf("windowLog(%v) = %d, want %d", tc.sizes, got, tc.want)
		}
	}
}
