package main

import (
	"log"
	"strings"
	"time"
)

// attendanceToastMaxAge keeps the on-screen "checked in" popup for live
// punches: a first-time import or a long-offline catch-up brings in old
// events, and flashing a popup for each of those would be noise.
const (
	attendanceToastMaxAge = 2 * time.Minute
	attendanceToastMax    = 5
)

type recordedAttendance struct {
	salespersonID int64
	name          string
	date, time    string
	eventAt       time.Time
	eventAtRFC    string
	deviceType    string
}

// attendanceKind mirrors eventKind() in attendance.vue so the popup agrees
// with the attendance page: the device's own IN/OUT if it sends one,
// otherwise alternating by the event's position in that person's day.
func (a *App) attendanceKind(e recordedAttendance) string {
	switch {
	case strings.EqualFold(e.deviceType, "IN"):
		return "IN"
	case strings.EqualFold(e.deviceType, "OUT"):
		return "OUT"
	}
	var position int
	err := a.conn.QueryRowContext(a.ctx, `SELECT COUNT(*) FROM salesperson_attendance_events
		WHERE salesperson_id = ? AND event_date = ? AND event_at <= ?`, e.salespersonID, e.date, e.eventAtRFC).Scan(&position)
	if err != nil {
		log.Printf("attendance kind: %v", err)
		return "IN"
	}
	if position%2 == 1 {
		return "IN"
	}
	return "OUT"
}

// announceRecordedAttendance tells the frontend that rows were imported
// ("imported" -- the attendance page reloads on it) and which of them are
// fresh enough for the check-in/out popup ("recent").
func (a *App) announceRecordedAttendance(recorded []recordedAttendance) {
	if len(recorded) == 0 {
		return
	}
	recent := make([]map[string]any, 0)
	for _, e := range recorded {
		if age := time.Since(e.eventAt); age > attendanceToastMaxAge || age < -attendanceToastMaxAge {
			continue
		}
		clock := e.time
		if len(clock) >= 5 {
			clock = clock[:5]
		}
		recent = append(recent, map[string]any{"name": e.name, "time": clock, "kind": a.attendanceKind(e)})
	}
	if len(recent) > attendanceToastMax {
		recent = recent[len(recent)-attendanceToastMax:]
	}
	a.emitAttendanceEvent("attendance_recorded", map[string]any{"imported": len(recorded), "recent": recent})
}
