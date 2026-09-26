package main

import (
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// The nav-rail attendance button turns red when the Akuvox is unreachable.
// That used to piggyback on SyncAttendance -- a full door-log download and
// import -- polled from the frontend once a minute. This is the cheap
// replacement: a TCP connect to the device's web port, run from one Go
// goroutine, that only emits an event when the state actually changes.
const (
	attendanceProbeInterval = time.Second
	attendanceProbeTimeout  = 500 * time.Millisecond
	// One dropped probe is normal LAN noise; two in a row is an outage.
	// Recovery is immediate (a single good probe).
	attendanceProbeFailuresToRed = 2
)

type attendanceStatus struct {
	Reachable  bool
	Configured bool
	Message    string
}

type attendanceProbe struct {
	mu       sync.Mutex
	status   attendanceStatus
	failures int
}

// observe folds one probe result into the debounced status and reports
// whether the status the UI sees changed. configured=false (attendance off
// or no IP) is reported immediately; a failed dial is only reported once it
// has repeated attendanceProbeFailuresToRed times.
func (p *attendanceProbe) observe(configured, dialOK bool) (attendanceStatus, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	next := p.status
	switch {
	case !configured:
		p.failures = 0
		next = attendanceStatus{Reachable: false, Configured: false, Message: "Attendance machine is not configured"}
	case dialOK:
		p.failures = 0
		next = attendanceStatus{Reachable: true, Configured: true, Message: "Connected"}
	default:
		p.failures++
		if p.failures >= attendanceProbeFailuresToRed {
			next = attendanceStatus{Reachable: false, Configured: true, Message: "Attendance machine is unreachable"}
		}
	}
	changed := next != p.status
	p.status = next
	return next, changed
}

func (p *attendanceProbe) current() attendanceStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// attendanceDialAddr adds the default web port when the configured device
// address is a bare host.
func attendanceDialAddr(ip string) string {
	ip = strings.TrimRight(strings.TrimSpace(ip), "/")
	if _, _, err := net.SplitHostPort(ip); err == nil {
		return ip
	}
	return net.JoinHostPort(ip, "80")
}

func (a *App) startAttendanceProbe() {
	if a.store == nil {
		return
	}
	// Start optimistic so the button doesn't flash red before the first probe.
	a.attendanceProbe.status = attendanceStatus{Reachable: true, Configured: true, Message: "Connected"}
	stop := make(chan struct{})
	a.attendanceProbeStop = stop

	a.backgroundWG.Add(1)
	go func() {
		defer a.backgroundWG.Done()
		// Emitting needs the real Wails context, which only exists once
		// startup has finished (same rule as emitSyncApplied).
		select {
		case <-a.ready:
		case <-stop:
			return
		}
		ticker := time.NewTicker(attendanceProbeInterval)
		defer ticker.Stop()
		for {
			a.probeAttendanceOnce()
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
	}()
}

func (a *App) stopAttendanceProbe() {
	if a.attendanceProbeStop != nil {
		close(a.attendanceProbeStop)
		a.attendanceProbeStop = nil
	}
}

// probeAttendanceOnce reads the config fresh each pass so changing the IP or
// toggling attendance in settings takes effect within a second.
func (a *App) probeAttendanceOnce() {
	ip, _, _, enabled, _, err := a.attendanceConfig()
	if err != nil {
		log.Printf("attendance probe: load config: %v", err)
		return
	}
	configured := enabled && ip != ""
	dialOK := false
	if configured {
		conn, err := net.DialTimeout("tcp", attendanceDialAddr(ip), attendanceProbeTimeout)
		if err == nil {
			conn.Close()
			dialOK = true
		}
	}
	if status, changed := a.attendanceProbe.observe(configured, dialOK); changed {
		a.emitAttendanceEvent("attendance_status", status.toMap())
	}
}

// emitAttendanceEvent is a Wails-lifecycle-safe wrapper around
// runtime.EventsEmit. Unlike the sync-driven emitters, the probe fires on its
// very first pass, which also runs under tests and headless starts that call
// startup with a plain context; Wails log.Fatal()s on those, so it only
// emits when the context carries the frontend Wails looks up itself.
func (a *App) emitAttendanceEvent(name string, data ...any) {
	select {
	case <-a.ready:
	default:
		return
	}
	if a.ctx == nil || a.ctx.Value("frontend") == nil {
		return
	}
	runtime.EventsEmit(a.ctx, name, data...)
}

func (s attendanceStatus) toMap() map[string]any {
	return map[string]any{"reachable": s.Reachable, "configured": s.Configured, "message": s.Message}
}

// GetAttendanceStatus is the frontend's initial read; changes after that
// arrive as "attendance_status" events.
func (a *App) GetAttendanceStatus() map[string]any {
	return a.attendanceProbe.current().toMap()
}
