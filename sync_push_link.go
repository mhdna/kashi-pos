package main

import (
	"errors"
	"net/url"
	"sync"
	"time"
)

const (
	pushLinkBackoffBase = 5 * time.Second
	pushLinkBackoffMax  = time.Minute
)

// pushLink is the push loop's view of whether kashi is reachable at all.
//
// Before it existed, every queued row carried its own exponential backoff
// (up to an hour) for *any* failure -- so a till that was offline for a day
// came back online with each invoice waiting out its own timer, trickling up
// over the next hour, and every offline pass sat through a 10s timeout per
// row. A network failure is a fact about the connection, not about the row,
// so it now lives here once: the pass stops at the first one, the whole push
// loop waits (5s, 10s, ... capped at a minute), and the first pass that gets
// through makes every pending row due at once (see runPushTick).
//
// Per-row backoff is still right for an answer kashi actually gave (a 5xx,
// a 408/429): that is about the row, and must not hold the others back.
//
// The zero value is "not known to be up", so the first pass after startup
// probes kashi and releases anything an older build had backed off.
type pushLink struct {
	mu       sync.Mutex
	up       bool
	failures int
	retryAt  time.Time
	// downs only ever grows, one per markDown. A push pass notes it at the
	// start and stops as soon as it moves: the network failed somewhere in
	// this pass, so every remaining row would just time out too.
	downs uint64
}

// markDown records a network failure and schedules the next try.
func (l *pushLink) markDown(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.up = false
	delay := pushLinkBackoffBase << l.failures
	if delay > pushLinkBackoffMax || delay <= 0 {
		delay = pushLinkBackoffMax
	}
	l.failures++
	l.downs++
	l.retryAt = now.Add(delay)
}

func (l *pushLink) downCount() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.downs
}

func (l *pushLink) markUp() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.up = true
	l.failures = 0
	l.retryAt = time.Time{}
}

func (l *pushLink) isUp() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.up
}

// waiting reports whether the push loop should skip this pass because the
// connection's backoff hasn't elapsed yet.
func (l *pushLink) waiting(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return !l.up && now.Before(l.retryAt)
}

// isNetworkError reports whether err means the request never got an answer
// from kashi: no connection, DNS failure, timeout. net/http wraps every such
// failure from Client.Do in a *url.Error; a response kashi did send comes
// back from responseError instead, which is never one. (A malformed server
// URL is a *url.Error too -- equally "can't reach kashi", so that's fine.)
func isNetworkError(err error) bool {
	var ue *url.Error
	return errors.As(err, &ue)
}
