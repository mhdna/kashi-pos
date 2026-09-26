package main

import "testing"

func TestAttendanceProbeDebounce(t *testing.T) {
	var p attendanceProbe
	p.status = attendanceStatus{Reachable: true, Configured: true, Message: "Connected"}

	if _, changed := p.observe(true, true); changed {
		t.Fatal("steady good probe should not report a change")
	}
	if _, changed := p.observe(true, false); changed {
		t.Fatal("a single failed probe must not turn the button red")
	}
	if st, changed := p.observe(true, false); !changed || st.Reachable {
		t.Fatalf("second consecutive failure should go red, got %+v changed=%v", st, changed)
	}
	if _, changed := p.observe(true, false); changed {
		t.Fatal("further failures should not re-report")
	}
	if st, changed := p.observe(true, true); !changed || !st.Reachable {
		t.Fatalf("one good probe should recover immediately, got %+v changed=%v", st, changed)
	}
	// A blip between good probes must reset the failure count.
	p.observe(true, false)
	p.observe(true, true)
	if _, changed := p.observe(true, false); changed {
		t.Fatal("failure count should reset after a good probe")
	}
}

func TestAttendanceProbeUnconfiguredIsImmediate(t *testing.T) {
	var p attendanceProbe
	p.status = attendanceStatus{Reachable: true, Configured: true, Message: "Connected"}
	st, changed := p.observe(false, false)
	if !changed || st.Reachable || st.Configured {
		t.Fatalf("unconfigured should report immediately, got %+v changed=%v", st, changed)
	}
}

func TestAttendanceDialAddr(t *testing.T) {
	cases := map[string]string{
		"192.168.0.110":      "192.168.0.110:80",
		" 192.168.0.110/ ":   "192.168.0.110:80",
		"192.168.0.110:8080": "192.168.0.110:8080",
	}
	for in, want := range cases {
		if got := attendanceDialAddr(in); got != want {
			t.Errorf("attendanceDialAddr(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSameHost(t *testing.T) {
	cases := []struct {
		remote, device string
		want           bool
	}{
		{"192.168.0.110:51234", "192.168.0.110", true},
		{"192.168.0.110:51234", "192.168.0.110:80", true},
		{"192.168.0.110:51234", " 192.168.0.110/ ", true},
		{"192.168.0.55:51234", "192.168.0.110", false},
		{"192.168.0.110:51234", "", false},
	}
	for _, c := range cases {
		if got := sameHost(c.remote, c.device); got != c.want {
			t.Errorf("sameHost(%q, %q) = %v, want %v", c.remote, c.device, got, c.want)
		}
	}
}

func TestAttendanceKindAlternatesPerPersonPerDay(t *testing.T) {
	app, store := setUpTestAttendanceApp(t, "")
	sp := makeSalesperson(t, app, store, "Sara Khoury", "7")
	other := makeSalesperson(t, app, store, "Omar Haddad", "8")
	insert := func(id int64, evID, at, typ string) recordedAttendance {
		date, clock := at[:10], at[11:19]
		if _, err := app.conn.Exec(`INSERT INTO salesperson_attendance_events
			(device_id, event_id, salesperson_id, attendance_user_id, salesperson_name, event_date, event_time, event_at, type, status)
			VALUES ('dev', ?, ?, '', 'x', ?, ?, ?, ?, 'Success')`, evID, id, date, clock, at+"+03:00", typ); err != nil {
			t.Fatal(err)
		}
		return recordedAttendance{salespersonID: id, date: date, eventAtRFC: at + "+03:00", deviceType: typ}
	}
	first := insert(sp.ID, "1", "2026-09-21T08:00:00", "Face")
	otherFirst := insert(other.ID, "2", "2026-09-21T08:05:00", "Face")
	second := insert(sp.ID, "3", "2026-09-21T17:00:00", "Face")
	explicit := insert(sp.ID, "4", "2026-09-21T17:01:00", "in")

	if got := app.attendanceKind(first); got != "IN" {
		t.Errorf("first punch of the day = %s, want IN", got)
	}
	if got := app.attendanceKind(otherFirst); got != "IN" {
		t.Errorf("another person's first punch = %s, want IN (position is per person)", got)
	}
	if got := app.attendanceKind(second); got != "OUT" {
		t.Errorf("second punch = %s, want OUT", got)
	}
	if got := app.attendanceKind(explicit); got != "IN" {
		t.Errorf("device-supplied type should win, got %s", got)
	}
}
