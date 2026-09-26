package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// The Akuvox can call a URL when the door unlocks (Setting > Action URL).
// Pointing "Relay Triggered" at /attendance/ping turns a check-in into an
// immediate sync instead of waiting for the next poll. The ping carries no
// trust: it only wakes a sync of the device we already poll, so a spoofed or
// lost ping costs nothing -- the slow fallback sync still catches everything.
const (
	defaultAttendancePushListen = "0.0.0.0:8789"
	// The device may fire the URL a moment before the door-log entry exists.
	attendancePushSettle = 500 * time.Millisecond
	// One retry when the first sync imported nothing.
	attendancePushRetryDelay = 1500 * time.Millisecond
)

type attendancePushServer struct {
	server *http.Server
}

func (a *App) attendancePushListenAddr() string {
	if v := strings.TrimSpace(os.Getenv("KASHI_ATTENDANCE_PUSH_LISTEN")); v != "" {
		return v
	}
	return defaultAttendancePushListen
}

func (a *App) startAttendancePushServer() {
	if a.store == nil {
		return
	}
	addr := a.attendancePushListenAddr()
	a.attendanceKick = make(chan struct{}, 1)
	stop := make(chan struct{})
	a.attendancePushStop = stop

	mux := http.NewServeMux()
	mux.HandleFunc("/attendance/ping", a.handleAttendancePing)
	s := &attendancePushServer{server: &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}}
	a.attendancePush = s

	go func() {
		log.Printf("attendance push listener on %s", addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("attendance push listener stopped: %v", err)
		}
	}()

	a.backgroundWG.Add(1)
	go func() {
		defer a.backgroundWG.Done()
		for {
			select {
			case <-stop:
				return
			case <-a.attendanceKick:
				a.runAttendancePushSync()
			}
		}
	}()
}

func (a *App) stopAttendancePushServer(ctx context.Context) {
	if a.attendancePushStop != nil {
		close(a.attendancePushStop)
		a.attendancePushStop = nil
	}
	if a.attendancePush != nil && a.attendancePush.server != nil {
		if err := a.attendancePush.server.Shutdown(ctx); err != nil {
			log.Printf("stop attendance push listener: %v", err)
		}
	}
}

// sameHost reports whether the request's remote address is the configured
// device (the config value may carry a port).
func sameHost(remoteAddr, deviceAddr string) bool {
	remote, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		remote = remoteAddr
	}
	device := strings.TrimRight(strings.TrimSpace(deviceAddr), "/")
	if host, _, err := net.SplitHostPort(device); err == nil {
		device = host
	}
	return device != "" && remote == device
}

func (a *App) handleAttendancePing(w http.ResponseWriter, r *http.Request) {
	ip, _, _, enabled, _, err := a.attendanceConfig()
	if err != nil || !enabled || !sameHost(r.RemoteAddr, ip) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	// Logged so we can see which variables the device actually sends.
	log.Printf("attendance ping from %s: %s", r.RemoteAddr, r.URL.RawQuery)
	select {
	case a.attendanceKick <- struct{}{}:
	default: // a sync is already queued; it will pick this event up too
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (a *App) runAttendancePushSync() {
	time.Sleep(attendancePushSettle)
	for attempt := 0; attempt < 2; attempt++ {
		result, err := a.SyncAttendance("")
		if err != nil {
			log.Printf("attendance push sync: %v", err)
			return
		}
		imported, _ := result["events"].(int)
		if imported > 0 {
			return
		}
		if attempt == 0 {
			time.Sleep(attendancePushRetryDelay)
		}
	}
}
