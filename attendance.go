package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

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
	a.attendanceSyncMu.Lock()
	defer a.attendanceSyncMu.Unlock()
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
	requestBody, err := json.Marshal(map[string]any{
		"target": "doorlog",
		"action": "get",
		"data": map[string]string{
			"TimeStart": date + " 00:00:00",
			"TimeEnd":   date + " 23:59:59",
			"Status":    "Success",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encode attendance request: %w", err)
	}
	req, err := http.NewRequestWithContext(a.ctx, http.MethodPost, url, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("create attendance request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Akuvox returns the complete door-log history, which can be a large JSON
	// response and may take more than a few seconds to transfer and decode.
	client := &http.Client{Timeout: 15 * time.Second}
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
		fullName      string
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
		candidates = append(candidates, candidate{salespersonID: salesperson.ID, userID: item.UserID, name: strings.TrimSpace(item.Name), fullName: salesperson.FullName, item: struct {
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
	centralURL, err := a.syncCentralAPIURL()
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	syncConfigured := centralURL != ""

	// One transaction for the whole import so the batched sync_outbox row
	// below commits atomically with the events it describes.
	tx, err := a.conn.BeginTx(a.ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	imported := 0
	newEvents := make([]branchAttendanceEvent, 0)
	recorded := make([]recordedAttendance, 0)
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
		eventAtRFC := candidate.eventAt.Format(time.RFC3339)
		if _, err = tx.ExecContext(a.ctx, `INSERT OR IGNORE INTO salesperson_attendance_events
			(device_id, event_id, salesperson_id, attendance_user_id, salesperson_name, event_date, event_time, event_at, type, status)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, ip, candidate.item.ID, candidate.salespersonID, candidate.userID,
			candidate.name, candidate.item.Date, candidate.item.Time, eventAtRFC, candidate.item.Type, candidate.item.Status); err != nil {
			return nil, err
		}
		existing[key] = append(existing[key], candidate.eventAt)
		imported++
		recorded = append(recorded, recordedAttendance{
			salespersonID: candidate.salespersonID,
			name:          candidate.fullName,
			date:          candidate.item.Date,
			time:          candidate.item.Time,
			eventAt:       candidate.eventAt,
			eventAtRFC:    eventAtRFC,
			deviceType:    candidate.item.Type,
		})
		if syncConfigured {
			newEvents = append(newEvents, branchAttendanceEvent{
				Ref:              ip + "|" + candidate.item.ID,
				SalespersonName:  candidate.name,
				AttendanceUserID: candidate.userID,
				EventDate:        candidate.item.Date,
				EventTime:        candidate.item.Time,
				EventAt:          eventAtRFC,
				Type:             candidate.item.Type,
				Status:           candidate.item.Status,
			})
		}
	}

	if len(newEvents) > 0 {
		body, err := json.Marshal(branchAttendanceEventsPayload{Events: newEvents})
		if err != nil {
			return nil, fmt.Errorf("marshal attendance events sync: %w", err)
		}
		if err := queueSyncOutboxRaw(a.ctx, tx, "attendance_events", uuid.NewString(), string(body)); err != nil {
			return nil, fmt.Errorf("queue attendance events sync: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if syncConfigured {
		a.kickSyncWorker()
	}
	a.announceRecordedAttendance(recorded)
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

func absDuration(a, b time.Time) time.Duration {
	d := a.Sub(b)
	if d < 0 {
		return -d
	}
	return d
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

// CreateManualAttendanceEvent lets an admin (e.g. kashi-admin) record an
// attendance event directly - device_id 'admin-manual' distinguishes it from
// device-synced rows (which use the Akuvox IP) and manager-approved requests
// (which use 'manual'). It syncs to central the same way device events do.
func (a *App) CreateManualAttendanceEvent(salespersonID int64, eventAt, eventType string) (map[string]any, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if salespersonID <= 0 {
		return nil, fmt.Errorf("salesperson is required")
	}
	eventType = strings.ToUpper(strings.TrimSpace(eventType))
	if eventType != "IN" && eventType != "OUT" {
		return nil, fmt.Errorf("type must be IN or OUT")
	}
	parsedAt, err := parseAttendanceTimestamp(eventAt)
	if err != nil {
		return nil, err
	}
	var name, attendanceUserID string
	if err := a.conn.QueryRowContext(a.ctx, `SELECT full_name, attendance_user_id FROM salespersons WHERE id = ?`, salespersonID).Scan(&name, &attendanceUserID); err != nil {
		return nil, fmt.Errorf("salesperson not found: %w", err)
	}
	centralURL, err := a.syncCentralAPIURL()
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}

	tx, err := a.conn.BeginTx(a.ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	eventID := uuid.NewString()
	eventDate := parsedAt.Format("2006-01-02")
	eventTime := parsedAt.Format("15:04:05")
	eventAtRFC := parsedAt.Format(time.RFC3339)
	if _, err := tx.ExecContext(a.ctx, `INSERT INTO salesperson_attendance_events
		(device_id, event_id, salesperson_id, attendance_user_id, salesperson_name, event_date, event_time, event_at, type, status)
		VALUES ('admin-manual', ?, ?, ?, ?, ?, ?, ?, ?, 'Approved')`,
		eventID, salespersonID, attendanceUserID, name, eventDate, eventTime, eventAtRFC, eventType); err != nil {
		return nil, err
	}

	if centralURL != "" {
		body, err := json.Marshal(branchAttendanceEventsPayload{Events: []branchAttendanceEvent{{
			Ref:              "admin-manual|" + eventID,
			SalespersonName:  name,
			AttendanceUserID: attendanceUserID,
			EventDate:        eventDate,
			EventTime:        eventTime,
			EventAt:          eventAtRFC,
			Type:             eventType,
			Status:           "Approved",
		}}})
		if err != nil {
			return nil, fmt.Errorf("marshal attendance event sync: %w", err)
		}
		if err := queueSyncOutboxRaw(a.ctx, tx, "attendance_events", uuid.NewString(), string(body)); err != nil {
			return nil, fmt.Errorf("queue attendance event sync: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if centralURL != "" {
		a.kickSyncWorker()
	}
	return map[string]any{"id": eventID, "salespersonId": salespersonID, "employeeName": name, "eventAt": eventAtRFC, "type": eventType}, nil
}

func parseAttendanceTimestamp(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("timestamp is required")
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02T15:04"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q; use YYYY-MM-DD HH:MM[:SS]", value)
}

func (a *App) SubmitAttendanceComplaint(employeeID int64, date string, originalEventID int64, requestedTime, requestedType, action, note string) error {
	action = strings.ToLower(strings.TrimSpace(action))
	// "add" is for a punch the device never recorded at all (someone forgot
	// to clock in, or the reader missed them): there is no original event to
	// fix, only a new one to create once a manager approves.
	if action != "replace" && action != "delete" && action != "add" {
		return fmt.Errorf("attendance request action must be add, replace or delete")
	}
	requestedType = strings.ToUpper(strings.TrimSpace(requestedType))
	if action != "delete" && requestedType != "IN" && requestedType != "OUT" {
		return fmt.Errorf("attendance type must be IN or OUT")
	}
	requestedTime = strings.TrimSpace(requestedTime)
	if action != "delete" {
		if _, err := time.ParseInLocation("2006-01-02 15:04", date+" "+requestedTime, time.Local); err != nil {
			return fmt.Errorf("invalid attendance date or time: %w", err)
		}
	}
	salesperson, err := a.store.GetSalespersonByID(a.ctx, employeeID)
	if err != nil {
		return err
	}
	var originalTime string
	if action == "add" {
		originalEventID = 0
	} else if err := a.conn.QueryRowContext(a.ctx, `SELECT event_time FROM salesperson_attendance_events WHERE id = ? AND salesperson_id = ? AND event_date = ?`, originalEventID, employeeID, date).Scan(&originalTime); err != nil {
		return fmt.Errorf("attendance event not found: %w", err)
	}
	changeNote := fmt.Sprintf("Replace %s with %s %s", originalTime, requestedType, requestedTime)
	switch action {
	case "delete":
		changeNote = fmt.Sprintf("Delete %s", originalTime)
	case "add":
		changeNote = fmt.Sprintf("Add %s %s", requestedType, requestedTime)
	}
	centralURL, err := a.syncCentralAPIURL()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	tx, err := a.conn.BeginTx(a.ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	occurredAt := time.Now().UTC()
	res, err := tx.ExecContext(a.ctx, `INSERT INTO attendance_change_requests (salesperson_id, attendance_date, original_event_id, original_time, requested_time, requested_type, action, note, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?)`, employeeID, date, originalEventID, originalTime, requestedTime, requestedType, action, changeNote, occurredAt.Format("2006-01-02 15:04:05"))
	if err != nil {
		return err
	}

	if centralURL != "" {
		requestID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		ref := fmt.Sprintf("acr-%d-complaint", requestID)
		body, err := json.Marshal(branchAttendanceChangePayload{
			Ref:             ref,
			Kind:            "complaint",
			SalespersonName: salesperson.FullName,
			AttendanceDate:  date,
			OriginalTime:    originalTime,
			RequestedTime:   requestedTime,
			RequestedType:   requestedType,
			Action:          action,
			Status:          "pending",
			Note:            changeNote,
			OccurredAt:      occurredAt,
		})
		if err != nil {
			return fmt.Errorf("marshal attendance complaint sync: %w", err)
		}
		if err := queueSyncOutboxRaw(a.ctx, tx, "attendance_change", ref, string(body)); err != nil {
			return fmt.Errorf("queue attendance complaint sync: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	if centralURL != "" {
		a.kickSyncWorker()
	}
	return nil
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

// verifyManagerPassword checks password against the manager/approver
// password configured in attendance_config - the only manager-role password
// this app has, reused for any action that needs manager sign-off (not just
// attendance approvals).
func (a *App) verifyManagerPassword(password string) error {
	var approverPasswordHash string
	if err := a.conn.QueryRowContext(a.ctx, `SELECT approver_password_hash FROM attendance_config WHERE id = 1`).Scan(&approverPasswordHash); err != nil {
		return err
	}
	if approverPasswordHash == "" || hashPassword(password) != approverPasswordHash {
		return fmt.Errorf("invalid manager password")
	}
	return nil
}

func (a *App) ApproveAttendanceComplaint(id int64, approved bool, password string) error {
	if !a.attendanceManagerApprovalEnabled() {
		return errManagerApprovalOff
	}
	if err := a.verifyManagerPassword(password); err != nil {
		return err
	}
	return a.reviewAttendanceComplaint(id, approved, "manager approval")
}

// reviewAttendanceComplaint applies a manager's decision on a pending request;
// actor is who decided, recorded on the request, its audit row and the copy
// sent to kashi. ApproveAttendanceComplaint (manager password, on the till)
// and kashi's review_attendance_request command (kashi checked its own
// user's rights) both land here. Only a pending request can be reviewed, so
// a decision arriving twice is refused rather than applied twice.
func (a *App) reviewAttendanceComplaint(id int64, approved bool, actor string) error {
	var currentStatus string
	if err := a.conn.QueryRowContext(a.ctx, `SELECT status FROM attendance_change_requests WHERE id = ?`, id).Scan(&currentStatus); err != nil {
		return fmt.Errorf("attendance request not found: %w", err)
	}
	if currentStatus != "pending" {
		return fmt.Errorf("this attendance request was already %s", currentStatus)
	}
	centralURL, err := a.syncCentralAPIURL()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	// The whole review — audit row, event delete/insert, request status —
	// runs in one transaction so the sync_outbox row queued at the end
	// commits atomically with it, the same guarantee every sqlc-based sync
	// call site gets (see queueBranchInvoiceSync's doc comment).
	tx, err := a.conn.BeginTx(a.ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	status := "rejected"
	if approved {
		status = "approved"
	}

	// Pulled up so both branches, and the sync payload, share the request's
	// identity.
	var reqName, reqDate, reqRequestedTime, reqRequestedType, reqAction, reqOriginalTime string
	if err := tx.QueryRowContext(a.ctx, `SELECT s.full_name, r.attendance_date, r.requested_time, r.requested_type, r.action, r.original_time
		FROM attendance_change_requests r JOIN salespersons s ON s.id = r.salesperson_id WHERE r.id = ?`, id).
		Scan(&reqName, &reqDate, &reqRequestedTime, &reqRequestedType, &reqAction, &reqOriginalTime); err != nil {
		return err
	}

	var changedFrom, changedTo string
	// What kashi needs to mirror an approval: the punch it took away (its
	// upload ref) and the punch it created, marked with who approved it.
	var originalEventRef string
	var newPunch *branchAttendanceEvent
	if approved {
		var salespersonID, originalEventID int64
		var date, requestedTime, requestedType, action, name, attendanceUserID string
		err := tx.QueryRowContext(a.ctx, `SELECT r.salesperson_id, r.original_event_id, r.attendance_date, r.requested_time, r.requested_type, r.action, s.full_name, s.attendance_user_id
			FROM attendance_change_requests r JOIN salespersons s ON s.id = r.salesperson_id WHERE r.id = ?`, id).
			Scan(&salespersonID, &originalEventID, &date, &requestedTime, &requestedType, &action, &name, &attendanceUserID)
		if err != nil {
			return err
		}
		// An "add" request has no original punch: nothing to audit or remove,
		// only the new one to insert below.
		if originalEventID > 0 {
			var originalEvent struct {
				deviceID, eventID, attendanceUserID, salespersonName, eventDate, eventTime, eventAt, eventType, status string
			}
			if err := tx.QueryRowContext(a.ctx, `SELECT device_id, event_id, attendance_user_id, salesperson_name, event_date, event_time, event_at, type, status
				FROM salesperson_attendance_events WHERE id = ? AND salesperson_id = ?`, originalEventID, salespersonID).Scan(
				&originalEvent.deviceID, &originalEvent.eventID, &originalEvent.attendanceUserID, &originalEvent.salespersonName, &originalEvent.eventDate, &originalEvent.eventTime, &originalEvent.eventAt, &originalEvent.eventType, &originalEvent.status); err != nil {
				return err
			}
			if _, err := tx.ExecContext(a.ctx, `INSERT INTO attendance_event_audit
				(request_id, original_event_id, device_id, event_id, salesperson_id, attendance_user_id, salesperson_name, event_date, event_time, event_at, type, status, action, audited_at, approved_by)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, originalEventID, originalEvent.deviceID, originalEvent.eventID, salespersonID, originalEvent.attendanceUserID, originalEvent.salespersonName, originalEvent.eventDate, originalEvent.eventTime, originalEvent.eventAt, originalEvent.eventType, originalEvent.status, action, time.Now().UTC().Format("2006-01-02 15:04:05"), actor); err != nil {
				return err
			}
			if _, err := tx.ExecContext(a.ctx, `DELETE FROM salesperson_attendance_events WHERE id = ? AND salesperson_id = ? AND event_date = ?`, originalEventID, salespersonID, date); err != nil {
				return err
			}
			originalEventRef = originalEvent.deviceID + "|" + originalEvent.eventID
		}
		if action == "replace" || action == "add" {
			eventAt, err := time.ParseInLocation("2006-01-02 15:04", date+" "+requestedTime, time.Local)
			if err != nil {
				return fmt.Errorf("invalid requested attendance time: %w", err)
			}
			if _, err := tx.ExecContext(a.ctx, `INSERT OR IGNORE INTO salesperson_attendance_events
			(device_id, event_id, salesperson_id, attendance_user_id, salesperson_name, event_date, event_time, event_at, type, status)
			VALUES ('manual', ?, ?, ?, ?, ?, ?, ?, ?, 'Approved')`, fmt.Sprintf("change-request-%d", id), salespersonID, attendanceUserID, name, date, requestedTime, eventAt.Format(time.RFC3339), requestedType); err != nil {
				return err
			}
			changeNote := "Added (missed punch)"
			if action == "replace" && reqOriginalTime != "" {
				changeNote = "Changed from " + reqOriginalTime
			}
			newPunch = &branchAttendanceEvent{
				Ref:              fmt.Sprintf("manual|change-request-%d", id),
				SalespersonName:  name,
				AttendanceUserID: attendanceUserID,
				EventDate:        date,
				EventTime:        requestedTime,
				EventAt:          eventAt.Format(time.RFC3339),
				Type:             requestedType,
				Status:           "Approved",
				ChangedBy:        actor,
				ChangeNote:       changeNote,
			}
		}
		changedFrom = requestedTime
		if originalEventID > 0 {
			var originalTime string
			_ = tx.QueryRowContext(a.ctx, `SELECT original_time FROM attendance_change_requests WHERE id = ?`, id).Scan(&originalTime)
			if originalTime != "" {
				changedFrom = originalTime
			}
		}
		changedTo = fmt.Sprintf("%s %s", requestedType, requestedTime)
		switch action {
		case "delete":
			changedTo = "deleted"
		case "add":
			changedFrom = "none"
		}
		if _, err := tx.ExecContext(a.ctx, `UPDATE attendance_change_requests SET changed_from = ?, changed_to = ?, approved_by = ?, note = ?, reviewed_at = ? WHERE id = ?`, changedFrom, changedTo, actor, fmt.Sprintf("Changed from %s to %s by %s", changedFrom, changedTo, actor), time.Now().UTC().Format("2006-01-02 15:04:05"), id); err != nil {
			return err
		}
	} else {
		if _, err := tx.ExecContext(a.ctx, `DELETE FROM salesperson_attendance_events WHERE device_id = 'manual' AND event_id = ?`, fmt.Sprintf("change-request-%d", id)); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(a.ctx, `UPDATE attendance_change_requests SET status = ?, reviewed_at = ? WHERE id = ?`, status, time.Now().UTC().Format("2006-01-02 15:04:05"), id); err != nil {
		return err
	}

	if centralURL != "" {
		// The new punch goes up like any punch, so kashi's attendance list
		// has it -- labelled with who approved it, not as a device punch.
		if newPunch != nil {
			body, err := json.Marshal(branchAttendanceEventsPayload{Events: []branchAttendanceEvent{*newPunch}})
			if err != nil {
				return fmt.Errorf("marshal attendance event sync: %w", err)
			}
			if err := queueSyncOutboxRaw(a.ctx, tx, "attendance_events", uuid.NewString(), string(body)); err != nil {
				return fmt.Errorf("queue attendance event sync: %w", err)
			}
		}
		ref := fmt.Sprintf("acr-%d-review", id)
		note := "rejected"
		if approved {
			note = strings.TrimSpace(fmt.Sprintf("%s -> %s", changedFrom, changedTo))
		}
		body, err := json.Marshal(branchAttendanceChangePayload{
			Ref:              ref,
			Kind:             "approval",
			SalespersonName:  reqName,
			AttendanceDate:   reqDate,
			OriginalTime:     reqOriginalTime,
			RequestedTime:    reqRequestedTime,
			RequestedType:    reqRequestedType,
			Action:           reqAction,
			Status:           status,
			Note:             note,
			Actor:            actor,
			OccurredAt:       time.Now().UTC(),
			OriginalEventRef: originalEventRef,
		})
		if err != nil {
			return fmt.Errorf("marshal attendance change sync: %w", err)
		}
		if err := queueSyncOutboxRaw(a.ctx, tx, "attendance_change", ref, string(body)); err != nil {
			return fmt.Errorf("queue attendance change sync: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	if centralURL != "" {
		a.kickSyncWorker()
	}
	return nil
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

// reviewAttendanceRequestPayload is what kashi's review_attendance_request
// command carries (see resolveCommandTarget in kashi's api/branch_command.go):
// the request's own ref ("acr-<id>-complaint", as this till sent it), the
// decision, and who made it in kashi.
type reviewAttendanceRequestPayload struct {
	ClientRef string `json:"client_ref"`
	Approved  bool   `json:"approved"`
	Actor     string `json:"actor"`
}

// executeReviewAttendanceRequestCommand applies an approval or rejection made
// in kashi. It goes through reviewAttendanceComplaint like a manager's
// decision here, minus the manager password -- kashi already checked its own
// user's rights -- and the result syncs back to kashi the same way.
func (a *App) executeReviewAttendanceRequestCommand(payload json.RawMessage) error {
	var p reviewAttendanceRequestPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("decode review_attendance_request payload: %w", err)
	}
	var id int64
	if _, err := fmt.Sscanf(strings.TrimSpace(p.ClientRef), "acr-%d-complaint", &id); err != nil || id <= 0 {
		return fmt.Errorf("not an attendance request ref: %q", p.ClientRef)
	}
	actor := strings.TrimSpace(p.Actor)
	if actor == "" {
		actor = "kashi"
	}
	return a.reviewAttendanceComplaint(id, p.Approved, actor)
}

// attendanceManagerApprovalFlag lets a manager decide attendance requests on
// the till itself, with the manager password -- approve or reject a request,
// or add a missed punch straight away. On unless turned off (here or from
// kashi's branch settings); when off, requests wait for kashi.
const attendanceManagerApprovalFlag = "attendance_manager_approval"

var errManagerApprovalOff = fmt.Errorf("manager approval on the till is turned off for this branch -- approve it from kashi")

func (a *App) attendanceManagerApprovalEnabled() bool {
	if v, err := a.store.GetFeatureFlag(a.ctx, attendanceManagerApprovalFlag); err == nil {
		return v != 0
	}
	return true
}

// GetAttendanceManagerApprovalEnabled is for the attendance screen, which
// shows the manager's approve/add buttons only while it's on.
func (a *App) GetAttendanceManagerApprovalEnabled() bool {
	if a.store == nil {
		return false
	}
	return a.attendanceManagerApprovalEnabled()
}

// SetAttendanceManagerApprovalEnabled is called by the Settings page and by
// kashi's update_settings.
func (a *App) SetAttendanceManagerApprovalEnabled(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := a.setFeatureFlagBool(a.ctx, attendanceManagerApprovalFlag, enabled); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

// AddAttendanceWithManagerApproval records a punch the device missed, on the
// manager's password, in one step: it files an "add" request and approves it
// at once, so it leaves the same request, audit and kashi trail as one a
// cashier filed and a manager approved.
func (a *App) AddAttendanceWithManagerApproval(salespersonID int64, date, requestedTime, requestedType, password string) error {
	if !a.attendanceManagerApprovalEnabled() {
		return errManagerApprovalOff
	}
	if err := a.verifyManagerPassword(password); err != nil {
		return err
	}
	if err := a.SubmitAttendanceComplaint(salespersonID, date, 0, requestedTime, requestedType, "add", ""); err != nil {
		return err
	}
	var id int64
	if err := a.conn.QueryRowContext(a.ctx, `SELECT MAX(id) FROM attendance_change_requests WHERE salesperson_id = ?`, salespersonID).Scan(&id); err != nil {
		return err
	}
	return a.reviewAttendanceComplaint(id, true, "manager approval")
}
