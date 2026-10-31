package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The log rolls over past its cap, keeping one older file, and the tail
// reads across both, starting on a whole line.
func TestRotatingLogRollsOverAndTailReadsBothFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "till.log")
	l, err := openRotatingLog(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		if _, err := l.Write([]byte(strings.Repeat("x", 19) + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	if info, err := os.Stat(path); err != nil || info.Size() > 100 {
		t.Fatalf("current log too big or missing: %v %v", info, err)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("no rolled-over log: %v", err)
	}
	tail, err := readLogTail(path, 150)
	if err != nil {
		t.Fatal(err)
	}
	if len(tail) > 150 || !strings.HasPrefix(tail, "xxx") || !strings.HasSuffix(tail, "\n") {
		t.Fatalf("tail = %q", tail)
	}
	if missing, err := readLogTail(filepath.Join(t.TempDir(), "none.log"), 100); err != nil || missing != "" {
		t.Fatalf("missing log: %q %v", missing, err)
	}
}
