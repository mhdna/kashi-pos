package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
)

// Small updates (central's api/app_update_compact.go). The till tells central the
// SHA-256 of the exe it runs; central offers a patch from exactly that build
// (~1-2 MB) when it has one, and the build zstd-compressed (~9 MB of ~28)
// otherwise. A patch is zstd with the running exe as its dictionary. Whatever
// arrives is unpacked to the plain exe and checked like before (verifyRelease:
// size, SHA-256, release-key signature), so a patch that rebuilds anything
// else is thrown away and the full build is downloaded instead.

const (
	// Most an update may unpack to, and the most memory/window the decoder may
	// use, so a broken file can't run the till out of memory.
	maxUpdateExeBytes   = 512 << 20
	maxUpdateZstdWindow = 1 << 30
)

// runningExeDigest caches the running exe's SHA-256: hashing ~28 MB on every
// update check would be felt on an old till.
var runningExeDigest struct {
	sync.Mutex
	path    string
	size    int64
	modTime time.Time
	sum     string
}

// ownExeSha256 is the SHA-256 of the exe this till runs, or "" when it can't
// be read (central then offers no patch).
func ownExeSha256() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	info, err := os.Stat(exe)
	if err != nil {
		return ""
	}
	d := &runningExeDigest
	d.Lock()
	defer d.Unlock()
	if d.path == exe && d.size == info.Size() && d.modTime.Equal(info.ModTime()) {
		return d.sum
	}
	sum, err := fileSha256(exe)
	if err != nil {
		return ""
	}
	d.path, d.size, d.modTime, d.sum = exe, info.Size(), info.ModTime(), sum
	return sum
}

func fileSha256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// unpackZstd writes the exe a zstd file holds to dest; dict, when set, is the
// raw dictionary a patch was made against (the exe it rebuilds from).
func unpackZstd(src, dest string, dict []byte) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	opts := []zstd.DOption{
		zstd.WithDecoderMaxWindow(maxUpdateZstdWindow),
		zstd.WithDecoderMaxMemory(maxUpdateExeBytes * 2),
		zstd.WithDecoderConcurrency(1),
	}
	if dict != nil {
		// The zstd tool's --patch-from frames carry no dictionary id.
		opts = append(opts, zstd.WithDecoderDictRaw(0, dict))
	}
	dec, err := zstd.NewReader(in, opts...)
	if err != nil {
		return err
	}
	defer dec.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	n, err := io.Copy(out, io.LimitReader(dec, maxUpdateExeBytes+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > maxUpdateExeBytes {
		err = errors.New("the update unpacks to more than 512MB")
	}
	if err != nil {
		os.Remove(dest)
	}
	return err
}

// fetchReleaseSmall gets the release into part the smallest way central offers:
// the patch, then the zstd build. It returns false when central offered
// neither, or both failed, and the plain download should run.
func (a *App) fetchReleaseSmall(ctx context.Context, client *centralClient, m *appUpdateManifest, releaseDir, part string, progress func(have, total int64)) bool {
	if p := m.Patch; p != nil && p.ID > 0 {
		err := a.fetchPatch(ctx, client, m, releaseDir, part, progress)
		if err == nil {
			return true
		}
		if ctx.Err() != nil {
			return false
		}
		log.Printf("updater: patch for %s didn't work, downloading the whole build: %v", m.Version, err)
	}
	if m.Encoding == "zstd" {
		packed := filepath.Join(releaseDir, "release.zst.part")
		err := client.downloadResumable(ctx, fmt.Sprintf("/branch/app_update/download/%d?encoding=zstd", m.ID), packed,
			func(have int64) { progress(have, m.DownloadBytes) })
		// A cut-off download stays, to resume next time; once complete it
		// has served its purpose whether it unpacks or not.
		if err == nil {
			err = unpackZstd(packed, part, nil)
			os.Remove(packed)
			if err == nil {
				err = verifyRelease(part, m)
			}
		}
		if err == nil {
			return true
		}
		os.Remove(part)
		if ctx.Err() != nil {
			return false
		}
		log.Printf("updater: compressed download of %s failed, trying the plain one: %v", m.Version, err)
	}
	return false
}

// fetchPatch downloads the patch and rebuilds the release from the running
// exe, which must be exactly the build the patch was made from.
func (a *App) fetchPatch(ctx context.Context, client *centralClient, m *appUpdateManifest, releaseDir, part string, progress func(have, total int64)) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	base, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(base)
	if hex.EncodeToString(sum[:]) != strings.ToLower(m.Patch.BaseSha256) {
		return errors.New("this till's program isn't the build the patch is for")
	}
	patch := filepath.Join(releaseDir, fmt.Sprintf("patch-%d.part", m.Patch.ID))
	if err := client.downloadResumable(ctx, fmt.Sprintf("/branch/app_update/patch/%d", m.Patch.ID), patch,
		func(have int64) { progress(have, m.Patch.SizeBytes) }); err != nil {
		return err // kept, to resume
	}
	err = unpackZstd(patch, part, base)
	os.Remove(patch)
	if err != nil {
		return err
	}
	if err := verifyRelease(part, m); err != nil {
		os.Remove(part)
		return err
	}
	return nil
}
