package license

import (
	"crypto/ed25519"
	"time"
)

// Why a till may not run. The cashier is never told which; the screen says
// the same vague thing for all of them.
const (
	ReasonNone         = ""
	ReasonBadLease     = "bad_lease"
	ReasonWrongMachine = "wrong_machine"
	ReasonOff          = "off"
	ReasonExpired      = "expired"
	ReasonClock        = "clock"
)

// ClockSlack is how far the PC clock may sit behind the server's time (or
// behind the latest time this till ran at) before it counts as set back.
const ClockSlack = 15 * time.Minute

// WarnBefore is how long before an offline lease runs out the owner is told
// to connect the till.
const WarnBefore = 48 * time.Hour

type Decision struct {
	OK     bool
	Reason string
	Lease  Lease
	// Left is how long the lease still holds without a check-in.
	Left time.Duration
	// Warn is set when Left is under WarnBefore.
	Warn bool
}

// Judge decides, offline, whether the saved lease lets this till run now.
func Judge(pub ed25519.PublicKey, st State, fingerprint string, now time.Time) Decision {
	l, err := Verify(pub, st.Lease)
	if err != nil {
		return Decision{Reason: ReasonBadLease}
	}
	d := Decision{Lease: l}
	switch {
	case l.InstallID != st.InstallID || l.Fingerprint != fingerprint:
		d.Reason = ReasonWrongMachine
	case l.Status != StatusOn:
		d.Reason = ReasonOff
	case now.Before(l.IssuedAt.Add(-ClockSlack)), !st.LastSeen.IsZero() && now.Before(st.LastSeen.Add(-ClockSlack)):
		d.Reason = ReasonClock
	case !now.Before(l.ValidUntil):
		d.Reason = ReasonExpired
	default:
		d.OK = true
		d.Left = l.ValidUntil.Sub(now)
		d.Warn = d.Left < WarnBefore
	}
	return d
}
