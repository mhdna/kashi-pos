package main

// Smaller uploads and downloads (central's api/app_update_compact.go, and the
// till's update_compact.go).
//
//   - The build is uploaded zstd-compressed: ~28 MB becomes ~9 MB.
//   - Then a patch from each build the tills of this flavor run: the new exe
//     zstd'd with the old one as its dictionary, ~1-2 MB. A till on one of
//     those builds downloads only that.
//
// The zstd tool (brew install zstd) is used when it's installed, since it
// packs best; otherwise the Go encoder, a little bigger. Each patch is
// rebuilt here and compared before it's uploaded, so a bad one never ships.
//
// Patches need the old builds. Every build published from this machine is
// kept in ~/.config/gb-cloud/releases/<flavor>/<version>.exe; one that isn't
// there is downloaded from central, while central still has its file.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

const maxDecodedBuild = 512 << 20

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// zstdTool is the zstd command, or "" when it isn't installed.
func zstdTool() string {
	path, err := exec.LookPath("zstd")
	if err != nil {
		return ""
	}
	return path
}

// windowLog fits both builds in zstd's window, so a patch can reach all of
// the old one: 2^27 (128 MB) at least.
func windowLog(sizes ...int) int {
	total := 0
	for _, n := range sizes {
		total += n
	}
	log := bits.Len(uint(total))
	return min(max(log, 27), 30)
}

// runZstd runs the zstd tool on in (a file), returning what it wrote.
func runZstd(tool string, args []string, in string) ([]byte, error) {
	out, err := os.CreateTemp("", "gb-cloud-*.zst")
	if err != nil {
		return nil, err
	}
	out.Close()
	defer os.Remove(out.Name())
	cmd := exec.Command(tool, append(args, "-q", "-f", in, "-o", out.Name())...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("zstd: %v %s", err, strings.TrimSpace(stderr.String()))
	}
	return os.ReadFile(out.Name())
}

func tempFile(content []byte) (string, error) {
	f, err := os.CreateTemp("", "gb-cloud-*.exe")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(content); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// unzstd unpacks data; dict is the old build for a patch.
func unzstd(data, dict []byte) ([]byte, error) {
	opts := []zstd.DOption{zstd.WithDecoderMaxWindow(1 << 30), zstd.WithDecoderMaxMemory(maxDecodedBuild * 2)}
	if dict != nil {
		opts = append(opts, zstd.WithDecoderDictRaw(0, dict))
	}
	dec, err := zstd.NewReader(nil, opts...)
	if err != nil {
		return nil, err
	}
	defer dec.Close()
	return dec.DecodeAll(data, nil)
}

// compressBuild packs the build, and checks it unpacks to exactly the build.
func compressBuild(path string, content []byte) ([]byte, error) {
	var packed []byte
	var err error
	if tool := zstdTool(); tool != "" {
		packed, err = runZstd(tool, []string{"--ultra", "-22", fmt.Sprintf("--long=%d", windowLog(len(content)))}, path)
	} else {
		var enc *zstd.Encoder
		enc, err = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBestCompression), zstd.WithWindowSize(64<<20))
		if err == nil {
			packed = enc.EncodeAll(content, nil)
		}
	}
	if err != nil {
		return nil, err
	}
	if back, err := unzstd(packed, nil); err != nil || !bytes.Equal(back, content) {
		return nil, fmt.Errorf("the compressed build doesn't unpack to the build (%v)", err)
	}
	return packed, nil
}

// makePatch is newB as a patch from base, checked to rebuild newB exactly.
func makePatch(base, newB []byte, newPath string) ([]byte, error) {
	var patch []byte
	var err error
	if tool := zstdTool(); tool != "" {
		var basePath string
		basePath, err = tempFile(base)
		if err != nil {
			return nil, err
		}
		defer os.Remove(basePath)
		patch, err = runZstd(tool, []string{"--ultra", "-22",
			fmt.Sprintf("--long=%d", windowLog(len(base), len(newB))), "--patch-from=" + basePath}, newPath)
	} else {
		var enc *zstd.Encoder
		enc, err = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBestCompression),
			zstd.WithWindowSize(64<<20), zstd.WithEncoderDictRaw(0, base))
		if err == nil {
			patch = enc.EncodeAll(newB, nil)
		}
	}
	if err != nil {
		return nil, err
	}
	if back, err := unzstd(patch, base); err != nil || !bytes.Equal(back, newB) {
		return nil, fmt.Errorf("the patch doesn't rebuild the build (%v)", err)
	}
	return patch, nil
}

// buildCacheDir keeps every build published from here, for later patches.
func buildCacheDir(flavor string) string {
	return filepath.Join(filepath.Dir(defaultKeyPath()), "releases", flavor)
}

func cacheBuild(flavor, version string, content []byte) {
	dir := buildCacheDir(flavor)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, version+".exe"), content, 0o600)
}

type publishedRelease struct {
	ID          int64  `json:"id"`
	Version     string `json:"version"`
	Flavor      string `json:"flavor"`
	Sha256      string `json:"sha256"`
	FileRemoved bool   `json:"file_removed"`
}

// baseBuild is the exe of an earlier release: from the cache, else from
// central. Either way it must be exactly what central says that release is.
func baseBuild(baseURL, token string, r publishedRelease) ([]byte, error) {
	cached := filepath.Join(buildCacheDir(r.Flavor), r.Version+".exe")
	if b, err := os.ReadFile(cached); err == nil && sha256Hex(b) == r.Sha256 {
		return b, nil
	}
	if r.FileRemoved {
		return nil, errors.New("not kept here, and central cleaned its file up")
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/app_releases/%d/file", baseURL, r.ID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 15 * time.Minute}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Central answered %d: %s", resp.StatusCode, errorMessage(resp.Body))
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxDecodedBuild))
	if err != nil {
		return nil, err
	}
	if resp.Header.Get("X-Encoding") == "zstd" {
		if b, err = unzstd(b, nil); err != nil {
			return nil, err
		}
	}
	if sha256Hex(b) != r.Sha256 {
		return nil, errors.New("central's file isn't the build it says")
	}
	cacheBuild(r.Flavor, r.Version, b)
	return b, nil
}

// publishPatches uploads a patch to the new release from each build that
// active tills of this flavor run. A patch that can't be made is skipped:
// those tills just download the whole (compressed) build.
func publishPatches(baseURL, token string, releaseID int64, version, flavor, newPath string, content []byte) {
	var regs struct {
		Registers []register `json:"registers"`
	}
	var rels struct {
		Releases []publishedRelease `json:"releases"`
	}
	if err := apiJSON(http.MethodGet, baseURL+"/app_updates/registers", token, nil, &regs); err != nil {
		fmt.Printf("No patches: couldn't list the tills (%v)\n", err)
		return
	}
	if err := apiJSON(http.MethodGet, baseURL+"/app_releases", token, nil, &rels); err != nil {
		fmt.Printf("No patches: couldn't list the releases (%v)\n", err)
		return
	}
	running := map[string]bool{}
	for _, r := range regs.Registers {
		if r.IsActive && r.AppFlavor == flavor && r.AppVersion != "" && r.AppVersion != version {
			running[r.AppVersion] = true
		}
	}
	byVersion := map[string]publishedRelease{}
	for _, r := range rels.Releases {
		if r.Flavor == flavor {
			byVersion[r.Version] = r
		}
	}
	versions := make([]string, 0, len(running))
	for v := range running {
		versions = append(versions, v)
	}
	sort.Strings(versions)
	for _, v := range versions {
		r, ok := byVersion[v]
		if !ok {
			fmt.Printf("  no patch from %s: central has no release of it\n", v)
			continue
		}
		base, err := baseBuild(baseURL, token, r)
		if err != nil {
			fmt.Printf("  no patch from %s: %v\n", v, err)
			continue
		}
		patch, err := makePatch(base, content, newPath)
		if err != nil {
			fmt.Printf("  no patch from %s: %v\n", v, err)
			continue
		}
		if err := uploadPatch(baseURL, token, releaseID, r, patch); err != nil {
			fmt.Printf("  no patch from %s: upload failed: %v\n", v, err)
			continue
		}
		fmt.Printf("  patch from %s: %.1f MB\n", v, float64(len(patch))/(1<<20))
	}
}

func uploadPatch(baseURL, token string, releaseID int64, base publishedRelease, patch []byte) error {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range map[string]string{
		"base_sha256": base.Sha256, "base_version": base.Version, "file_sha256": sha256Hex(patch),
	} {
		_ = w.WriteField(k, v)
	}
	part, err := w.CreateFormFile("file", "update.patch")
	if err != nil {
		return err
	}
	if _, err := part.Write(patch); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/app_releases/%d/patches", baseURL, releaseID), &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("status %d: %s", resp.StatusCode, errorMessage(resp.Body))
	}
	return nil
}
