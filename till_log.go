package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// The till's log. A Windows GUI program has no console, so before this
// everything log.Printf wrote (sync errors, why the live link to kashi
// dropped) went nowhere. Now it also goes to a file in the till's data
// folder, which kashi can ask for (the send_logs command, till_log_upload.go)
// and the sync card reads its last error from the app instead.
//
// The file is capped: past tillLogMaxBytes it becomes kashi-pos.log.1 (the
// one before is dropped) and a new one starts, so it can never fill a disk.

const (
	tillLogMaxBytes = 5 << 20
	tillLogName     = "kashi-pos.log"
)

func tillLogDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "kashi-pos", "logs"), nil
}

// rotatingLog is an append-only file that rolls over at maxBytes.
type rotatingLog struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	file     *os.File
	size     int64
}

func openRotatingLog(path string, maxBytes int64) (*rotatingLog, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	l := &rotatingLog{path: path, maxBytes: maxBytes}
	if err := l.open(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *rotatingLog) open() error {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	l.file, l.size = f, info.Size()
	return nil
}

func (l *rotatingLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return len(p), nil
	}
	if l.size > 0 && l.size+int64(len(p)) > l.maxBytes {
		l.file.Close()
		// The previous roll-over is dropped; on Windows Rename won't
		// overwrite, so it goes first.
		_ = os.Remove(l.path + ".1")
		_ = os.Rename(l.path, l.path+".1")
		if err := l.open(); err != nil {
			l.file = nil
			return len(p), nil
		}
	}
	n, err := l.file.Write(p)
	l.size += int64(n)
	return n, err
}

var tillLog *rotatingLog

// setUpTillLog sends the standard logger to the console and the log file.
// A till that can't open the file keeps logging to the console only.
func setUpTillLog() {
	dir, err := tillLogDir()
	if err != nil {
		log.Printf("till log: no data folder: %v", err)
		return
	}
	l, err := openRotatingLog(filepath.Join(dir, tillLogName), tillLogMaxBytes)
	if err != nil {
		log.Printf("till log: %v", err)
		return
	}
	tillLog = l
	log.SetOutput(io.MultiWriter(os.Stderr, l))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

// readLogTail is the newest maxBytes of the log at path, reaching into the
// rolled-over file when the current one is shorter. It starts on a whole
// line.
func readLogTail(path string, maxBytes int64) (string, error) {
	current, err := readFileTail(path, maxBytes)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	var older []byte
	if rest := maxBytes - int64(len(current)); rest > 0 {
		older, _ = readFileTail(path+".1", rest)
	}
	tail := append(older, current...)
	if int64(len(tail)) >= maxBytes {
		// Cut back to the start of a line.
		for i, b := range tail {
			if b == '\n' {
				tail = tail[i+1:]
				break
			}
		}
	}
	return string(tail), nil
}

func readFileTail(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	start := info.Size() - maxBytes
	if start < 0 {
		start = 0
	}
	buf := make([]byte, info.Size()-start)
	if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return buf, nil
}
