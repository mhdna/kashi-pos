package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	db "kashi-pos/db/sqlc"
)

type akuvoxResponse struct {
	Retcode int    `json:"retcode"`
	Message string `json:"message"`
	Data    struct {
		Items []struct {
			Code   string `json:"Code"`
			Date   string `json:"Date"`
			ID     string `json:"ID"`
			Name   string `json:"Name"`
			Status string `json:"Status"`
			Time   string `json:"Time"`
			Type   string `json:"Type"`
			UserID string `json:"UserID"`
		} `json:"item"`
	} `json:"data"`
}

const attendanceCashierHistoryFlag = "attendance_cashier_history"

func (a *App) attendanceConfig() (string, string, string, bool, int64, error) {
	var ip, username, password string
	var enabled int64
	var duplicateInterval int64
	err := a.conn.QueryRowContext(a.ctx, `SELECT akuvox_ip, akuvox_username, akuvox_password, attendance_enabled, duplicate_interval_seconds FROM attendance_config WHERE id = 1`).Scan(&ip, &username, &password, &enabled, &duplicateInterval)
	return strings.TrimSpace(ip), username, password, enabled != 0, duplicateInterval, err
}

func (a *App) GetAttendanceConfig() (map[string]any, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	ip, username, _, enabled, duplicateInterval, err := a.attendanceConfig()
	if err != nil {
		return nil, err
	}
	history, _ := a.store.GetFeatureFlag(a.ctx, attendanceCashierHistoryFlag)
	var approverPasswordHash string
	if err := a.conn.QueryRowContext(a.ctx, `SELECT approver_password_hash FROM attendance_config WHERE id = 1`).Scan(&approverPasswordHash); err != nil {
		return nil, err
	}
	return map[string]any{"ip": ip, "username": username, "enabled": enabled, "cashierHistory": history != 0, "approverPasswordSet": approverPasswordHash != "", "duplicateIntervalSeconds": duplicateInterval}, nil
}

func (a *App) UpdateAttendanceApproverPassword(password string) error {
	if a.conn == nil {
		return fmt.Errorf("database not initialized")
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return fmt.Errorf("approver password is required")
	}
	_, err := a.conn.ExecContext(a.ctx, `UPDATE attendance_config SET approver_password_hash = ? WHERE id = 1`, hashPassword(password))
	return err
}

func (a *App) UpdateAttendanceConfig(ip, username, password string, enabled bool) error {
	if a.conn == nil {
		return fmt.Errorf("database not initialized")
	}
	if strings.TrimSpace(password) == "" {
		var current string
		if err := a.conn.QueryRowContext(a.ctx, `SELECT akuvox_password FROM attendance_config WHERE id = 1`).Scan(&current); err == nil {
			password = current
		}
	}
	_, err := a.conn.ExecContext(a.ctx, `INSERT INTO attendance_config(id, akuvox_ip, akuvox_username, akuvox_password, attendance_enabled) VALUES (1, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET akuvox_ip=excluded.akuvox_ip, akuvox_username=excluded.akuvox_username, akuvox_password=excluded.akuvox_password, attendance_enabled=excluded.attendance_enabled`, strings.TrimSpace(ip), strings.TrimSpace(username), password, boolInt(enabled))
	return err
}

func (a *App) SetAttendanceCashierHistoryEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.store.SetFeatureFlag(a.ctx, db.SetFeatureFlagParams{Key: attendanceCashierHistoryFlag, Enabled: boolInt(enabled)})
}

func (a *App) SetAttendanceDuplicateInterval(seconds int64) error {
	if a.conn == nil {
		return fmt.Errorf("database not initialized")
	}
	if seconds < 0 {
		return fmt.Errorf("duplicate interval cannot be negative")
	}
	_, err := a.conn.ExecContext(a.ctx, `UPDATE attendance_config SET duplicate_interval_seconds = ? WHERE id = 1`, seconds)
	return err
}

func (a *App) SyncAttendance(date string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	ip, username, password, enabled, duplicateInterval, err := a.attendanceConfig()
	if err != nil {
		return nil, err
	}
	if !enabled || ip == "" {
		return map[string]any{"reachable": false, "configured": false, "message": "Attendance machine is not configured"}, nil
	}
	if removed, err := a.collapseAttendanceDuplicates(duplicateInterval); err != nil {
		return nil, err
	} else if removed > 0 {
		// Existing records are cleaned before importing the latest device data.
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	url := "http://" + strings.TrimRight(ip, "/") + "/api/doorlog/get"
	req, _ := http.NewRequestWithContext(a.ctx, http.MethodGet, url, nil)
	client := &http.Client{Timeout: 2 * time.Second}
	if username != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{"reachable": false, "configured": true, "message": "Attendance machine is unreachable"}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return map[string]any{"reachable": false, "configured": true, "message": fmt.Sprintf("Attendance machine returned HTTP %d", resp.StatusCode)}, nil
	}
	var payload akuvoxResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode attendance response: %w", err)
	}
	salespersons, err := a.store.ListSalespersons(a.ctx)
	if err != nil {
		return nil, err
	}
	byDoorID := make(map[string]db.Salesperson)
	for _, salesperson := range salespersons {
		if strings.TrimSpace(salesperson.AttendanceUserID) != "" {
			byDoorID[strings.TrimSpace(salesperson.AttendanceUserID)] = salesperson
		}
	}
	// The device has no date filter. Its response is newest-first; the first
	// 2000 records are enough to cover the current day in normal operation.
	items := payload.Data.Items
	if len(items) > 2000 {
		items = items[:2000]
	}
	type candidate struct {
		salespersonID int64
		userID        string
		name          string
		item          struct {
			ID, Date, Time, Type, Status string
		}
		eventAt time.Time
	}
	candidates := make([]candidate, 0, len(items))
	for _, item := range items {
		if item.Status != "Success" {
			continue
		}
		salesperson, ok := byDoorID[strings.TrimSpace(item.UserID)]
		if !ok {
			continue
		} // visitors and unassigned device users are ignored
		eventAt, err := time.ParseInLocation("2006-01-02 15:04:05", item.Date+" "+item.Time, time.Local)
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{salespersonID: salesperson.ID, userID: item.UserID, name: strings.TrimSpace(item.Name), item: struct {
			ID, Date, Time, Type, Status string
		}{item.ID, item.Date, item.Time, item.Type, item.Status}, eventAt: eventAt})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].salespersonID != candidates[j].salespersonID {
			return candidates[i].salespersonID < candidates[j].salespersonID
		}
		return candidates[i].eventAt.Before(candidates[j].eventAt)
	})
	existing := make(map[string][]time.Time)
	rows, err := a.conn.QueryContext(a.ctx, `SELECT salesperson_id, event_date, event_at FROM salesperson_attendance_events ORDER BY salesperson_id, event_date, event_at`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var salespersonID int64
		var eventDate, eventAtValue string
		if err := rows.Scan(&salespersonID, &eventDate, &eventAtValue); err != nil {
			rows.Close()
			return nil, err
		}
		parsed, err := time.Parse(time.RFC3339, eventAtValue)
		if err == nil {
			existing[fmt.Sprintf("%d|%s", salespersonID, eventDate)] = append(existing[fmt.Sprintf("%d|%s", salespersonID, eventDate)], parsed)
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	imported := 0
	for _, candidate := range candidates {
		key := fmt.Sprintf("%d|%s", candidate.salespersonID, candidate.item.Date)
		duplicate := false
		for _, previous := range existing[key] {
			if absDuration(candidate.eventAt, previous) < time.Duration(duplicateInterval)*time.Second {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		_, err = a.conn.ExecContext(a.ctx, `INSERT OR IGNORE INTO salesperson_attendance_events
			(device_id, event_id, salesperson_id, attendance_user_id, salesperson_name, event_date, event_time, event_at, type, status)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, ip, candidate.item.ID, candidate.salespersonID, candidate.userID,
			candidate.name, candidate.item.Date, candidate.item.Time, candidate.eventAt.Format(time.RFC3339), candidate.item.Type, candidate.item.Status)
		if err != nil {
			return nil, err
		}
		existing[key] = append(existing[key], candidate.eventAt)
		imported++
	}
	return map[string]any{"reachable": true, "configured": true, "message": "Connected", "events": imported}, nil
}

func (a *App) collapseAttendanceDuplicates(intervalSeconds int64) (int, error) {
	if intervalSeconds <= 0 {
		return 0, nil
	}
	rows, err := a.conn.QueryContext(a.ctx, `SELECT id, salesperson_id, event_date, event_at
		FROM salesperson_attendance_events ORDER BY salesperson_id, event_date, event_at, id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	last := make(map[string]time.Time)
	remove := make([]int64, 0)
	for rows.Next() {
		var id, salespersonID int64
		var date, eventAt string
		if err := rows.Scan(&id, &salespersonID, &date, &eventAt); err != nil {
			return 0, err
		}
		parsed, err := time.Parse(time.RFC3339, eventAt)
		if err != nil {
			continue
		}
		key := fmt.Sprintf("%d|%s", salespersonID, date)
		if previous, ok := last[key]; ok && parsed.Sub(previous) < time.Duration(intervalSeconds)*time.Second {
			remove = append(remove, id)
			continue
		}
		last[key] = parsed
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, id := range remove {
		if _, err := a.conn.ExecContext(a.ctx, `DELETE FROM salesperson_attendance_events WHERE id = ?`, id); err != nil {
			return 0, err
		}
	}
	return len(remove), nil
}

func parseAttendanceTime(value string) time.Time { t, _ := time.Parse(time.RFC3339, value); return t }
func absDuration(a, b time.Time) time.Duration {
	d := a.Sub(b)
	if d < 0 {
		return -d
	}
	return d
}

func attendanceEventMap(e db.AttendanceEvent) map[string]any {
	return map[string]any{"id": e.ID, "employeeId": e.EmployeeID, "employeeName": e.EmployeeName, "date": e.EventDate, "time": e.EventTime, "eventAt": e.EventAt, "type": e.Type}
}

func (a *App) ListAttendance(date string) ([]map[string]any, error) {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	rows, err := a.conn.QueryContext(a.ctx, `SELECT id, salesperson_id, salesperson_name, attendance_user_id, event_date, event_time, event_at, type, status
		FROM salesperson_attendance_events WHERE event_date = ? ORDER BY event_at`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, salespersonID int64
		var name, userID, eventDate, eventTime, eventAt, eventType, status string
		if err := rows.Scan(&id, &salespersonID, &name, &userID, &eventDate, &eventTime, &eventAt, &eventType, &status); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"id": id, "employeeId": salespersonID, "employeeName": name, "userId": userID, "date": eventDate, "time": eventTime, "eventAt": eventAt, "type": eventType, "status": status})
	}
	return result, rows.Err()
}

func (a *App) SubmitAttendanceComplaint(employeeID int64, date string, originalEventID int64, requestedTime, requestedType, action, note string) error {
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "replace" && action != "delete" {
		return fmt.Errorf("attendance request action must be replace or delete")
	}
	requestedType = strings.ToUpper(strings.TrimSpace(requestedType))
	if action == "replace" && requestedType != "IN" && requestedType != "OUT" {
		return fmt.Errorf("attendance type must be IN or OUT")
	}
	if _, err := a.store.GetSalespersonByID(a.ctx, employeeID); err != nil {
		return err
	}
	var originalTime string
	if err := a.conn.QueryRowContext(a.ctx, `SELECT event_time FROM salesperson_attendance_events WHERE id = ? AND salesperson_id = ? AND event_date = ?`, originalEventID, employeeID, date).Scan(&originalTime); err != nil {
		return fmt.Errorf("attendance event not found: %w", err)
	}
	changeNote := fmt.Sprintf("Replace %s with %s %s", originalTime, requestedType, requestedTime)
	if action == "delete" {
		changeNote = fmt.Sprintf("Delete %s", originalTime)
	}
	_, err := a.conn.ExecContext(a.ctx, `INSERT INTO attendance_change_requests (salesperson_id, attendance_date, original_event_id, original_time, requested_time, requested_type, action, note, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?)`, employeeID, date, originalEventID, originalTime, requestedTime, requestedType, action, changeNote, time.Now().Format(time.RFC3339))
	return err
}

func (a *App) ListAttendanceComplaints(month string) ([]map[string]any, error) {
	rows, err := a.conn.QueryContext(a.ctx, `SELECT r.id, r.salesperson_id, s.full_name, r.attendance_date, COALESCE(NULLIF(r.original_time, ''), e.event_time, ''), r.requested_time, r.requested_type, r.action, r.note, r.status, r.created_at, r.reviewed_at, r.changed_from, r.changed_to, r.approved_by
		FROM attendance_change_requests r JOIN salespersons s ON s.id = r.salesperson_id
		LEFT JOIN salesperson_attendance_events e ON e.id = r.original_event_id
		WHERE r.attendance_date LIKE ? ORDER BY r.attendance_date DESC, r.created_at DESC`, month+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, salespersonID int64
		var name, date, originalTime, requestedTime, requestedType, action, note, status, createdAt, reviewedAt, changedFrom, changedTo, approvedBy string
		if err := rows.Scan(&id, &salespersonID, &name, &date, &originalTime, &requestedTime, &requestedType, &action, &note, &status, &createdAt, &reviewedAt, &changedFrom, &changedTo, &approvedBy); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"id": id, "employeeId": salespersonID, "employeeName": name, "date": date, "originalTime": originalTime, "requestedTime": requestedTime, "requestedType": requestedType, "action": action, "note": note, "status": status, "approved": status == "approved", "createdAt": createdAt, "reviewedAt": reviewedAt, "changedFrom": changedFrom, "changedTo": changedTo, "approvedBy": approvedBy})
	}
	return result, rows.Err()
}

func (a *App) ApproveAttendanceComplaint(id int64, approved bool, password string) error {
	var approverPasswordHash string
	if err := a.conn.QueryRowContext(a.ctx, `SELECT approver_password_hash FROM attendance_config WHERE id = 1`).Scan(&approverPasswordHash); err != nil {
		return err
	}
	if approverPasswordHash == "" || hashPassword(password) != approverPasswordHash {
		return fmt.Errorf("invalid attendance approver password")
	}
	status := "rejected"
	if approved {
		status = "approved"
	}
	if approved {
		var salespersonID, originalEventID int64
		var date, requestedTime, requestedType, action, name, attendanceUserID string
		err := a.conn.QueryRowContext(a.ctx, `SELECT r.salesperson_id, r.original_event_id, r.attendance_date, r.requested_time, r.requested_type, r.action, s.full_name, s.attendance_user_id
			FROM attendance_change_requests r JOIN salespersons s ON s.id = r.salesperson_id WHERE r.id = ?`, id).
			Scan(&salespersonID, &originalEventID, &date, &requestedTime, &requestedType, &action, &name, &attendanceUserID)
		if err != nil {
			return err
		}
		var originalEvent struct {
			deviceID, eventID, attendanceUserID, salespersonName, eventDate, eventTime, eventAt, eventType, status string
		}
		if err := a.conn.QueryRowContext(a.ctx, `SELECT device_id, event_id, attendance_user_id, salesperson_name, event_date, event_time, event_at, type, status
			FROM salesperson_attendance_events WHERE id = ? AND salesperson_id = ?`, originalEventID, salespersonID).Scan(
			&originalEvent.deviceID, &originalEvent.eventID, &originalEvent.attendanceUserID, &originalEvent.salespersonName, &originalEvent.eventDate, &originalEvent.eventTime, &originalEvent.eventAt, &originalEvent.eventType, &originalEvent.status); err != nil {
			return err
		}
		if _, err := a.conn.ExecContext(a.ctx, `INSERT INTO attendance_event_audit
			(request_id, original_event_id, device_id, event_id, salesperson_id, attendance_user_id, salesperson_name, event_date, event_time, event_at, type, status, action, audited_at, approved_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'manager approval')`, id, originalEventID, originalEvent.deviceID, originalEvent.eventID, salespersonID, originalEvent.attendanceUserID, originalEvent.salespersonName, originalEvent.eventDate, originalEvent.eventTime, originalEvent.eventAt, originalEvent.eventType, originalEvent.status, action, time.Now().Format(time.RFC3339)); err != nil {
			return err
		}
		if originalEventID > 0 {
			if _, err := a.conn.ExecContext(a.ctx, `DELETE FROM salesperson_attendance_events WHERE id = ? AND salesperson_id = ? AND event_date = ?`, originalEventID, salespersonID, date); err != nil {
				return err
			}
		}
		if action == "replace" {
			eventAt, err := time.ParseInLocation("2006-01-02 15:04", date+" "+requestedTime, time.Local)
			if err != nil {
				return fmt.Errorf("invalid requested attendance time: %w", err)
			}
			if _, err := a.conn.ExecContext(a.ctx, `INSERT OR IGNORE INTO salesperson_attendance_events
			(device_id, event_id, salesperson_id, attendance_user_id, salesperson_name, event_date, event_time, event_at, type, status)
			VALUES ('manual', ?, ?, ?, ?, ?, ?, ?, ?, 'Approved')`, fmt.Sprintf("change-request-%d", id), salespersonID, attendanceUserID, name, date, requestedTime, eventAt.Format(time.RFC3339), requestedType); err != nil {
				return err
			}
		}
		changedFrom := fmt.Sprintf("%s", requestedTime)
		if originalEventID > 0 {
			var originalTime string
			_ = a.conn.QueryRowContext(a.ctx, `SELECT original_time FROM attendance_change_requests WHERE id = ?`, id).Scan(&originalTime)
			if originalTime != "" {
				changedFrom = originalTime
			}
		}
		changedTo := fmt.Sprintf("%s %s", requestedType, requestedTime)
		if action == "delete" {
			changedTo = "deleted"
		}
		if _, err := a.conn.ExecContext(a.ctx, `UPDATE attendance_change_requests SET changed_from = ?, changed_to = ?, approved_by = 'manager approval', note = ?, reviewed_at = ? WHERE id = ?`, changedFrom, changedTo, fmt.Sprintf("Changed from %s to %s by manager approval", changedFrom, changedTo), time.Now().Format(time.RFC3339), id); err != nil {
			return err
		}
	} else {
		if _, err := a.conn.ExecContext(a.ctx, `DELETE FROM salesperson_attendance_events WHERE device_id = 'manual' AND event_id = ?`, fmt.Sprintf("change-request-%d", id)); err != nil {
			return err
		}
	}
	_, err := a.conn.ExecContext(a.ctx, `UPDATE attendance_change_requests SET status = ?, reviewed_at = ? WHERE id = ?`, status, time.Now().Format(time.RFC3339), id)
	return err
}

func (a *App) ExportAttendanceMonth(month string) (string, error) {
	rows, err := a.conn.QueryContext(a.ctx, `SELECT id, salesperson_id, salesperson_name, attendance_user_id, event_date, event_time, event_at, type, status
		FROM salesperson_attendance_events WHERE event_date LIKE ? ORDER BY event_date, event_at`, month+"%")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	complaints, err := a.ListAttendanceComplaints(month)
	if err != nil {
		return "", err
	}
	clean := map[string]any{"month": month, "events": make([]map[string]any, 0), "complaints": complaints}
	for rows.Next() {
		var id, salespersonID int64
		var name, userID, eventDate, eventTime, eventAt, eventType, status string
		if err := rows.Scan(&id, &salespersonID, &name, &userID, &eventDate, &eventTime, &eventAt, &eventType, &status); err != nil {
			return "", err
		}
		clean["events"] = append(clean["events"].([]map[string]any), map[string]any{"id": id, "employeeId": salespersonID, "employeeName": name, "userId": userID, "date": eventDate, "time": eventTime, "eventAt": eventAt, "type": eventType, "status": status})
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(clean, "", "  ")
	return string(data), err
}
