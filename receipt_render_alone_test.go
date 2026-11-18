package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The receipt renderer (scripts/receipt-render-files.txt) must build on its
// own, without the app or the database: central copies exactly these files
// to draw the Receipt Designer's live receipt (make receipt-renderer).
func TestReceiptRendererBuildsAlone(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a module")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go toolchain")
	}
	list, err := os.ReadFile("scripts/receipt-render-files.txt")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, name := range strings.Fields(string(list)) {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		out := strings.Replace(string(src), "package main\n", "package receiptrender\n", 1)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, assets := range []string{"assets/fonts", "pdf-icons"} {
		cmd := exec.Command("cp", "-R", assets, filepath.Join(dir, filepath.Dir(assets)+"/"))
		_ = os.MkdirAll(filepath.Join(dir, filepath.Dir(assets)), 0o755)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("copy %s: %v %s", assets, err, out)
		}
	}
	goMod, _ := os.ReadFile("go.mod")
	version := ""
	for _, line := range strings.Split(string(goMod), "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[0] == "golang.org/x/image" {
			version = f[1]
		}
	}
	mod := "module receiptrender\n\ngo 1.22\n\nrequire golang.org/x/image " + version + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(goBin, "vet", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the renderer doesn't build alone (move what it needs into receipt_render_deps.go):\n%s", out)
	}
}
