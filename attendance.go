package main

import (
	"encoding/json"
	"fmt"
	"net/http"
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

func (a *App) attendanceConfig() (string, string, string, bool, error) {
	var ip, username, password string
	var enabled int64
	err := a.conn.QueryRowContext(a.ctx, `SELECT akuvox_ip, akuvox_username, akuvox_password, attendance_enabled FROM attendance_config WHERE id = 1`).Scan(&ip, &username, &password, &enabled)
	return strings.TrimSpace(ip), username, password, enabled != 0, err
}

func (a *App) GetAttendanceConfig() (map[string]any, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	ip, username, _, enabled, err := a.attendanceConfig()
	if err != nil {
		return nil, err
	}
	return map[string]any{"ip": ip, "username": username, "enabled": enabled}, nil
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

func (a *App) SyncAttendance(date string) (map[string]any, error) {
	if a.store == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	ip, username, password, enabled, err := a.attendanceConfig()
	if err != nil {
		return nil, err
	}
	if !enabled || ip == "" {
		return map[string]any{"reachable": false, "configured": false, "message": "Attendance machine is not configured"}, nil
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	url := "http://" + strings.TrimRight(ip, "/") + "/api/doorlock/get"
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
	users, err := a.store.ListUsers(a.ctx)
	if err != nil {
		return nil, err
	}
	byDoorID := make(map[string]db.User)
	for _, user := range users {
		if user.AttendanceUserID != "" {
			byDoorID[strings.TrimSpace(user.AttendanceUserID)] = user
		}
	}
	for _, item := range payload.Data.Items {
		if item.Date != date || item.Status != "Success" {
			continue
		}
		user, ok := byDoorID[strings.TrimSpace(item.UserID)]
		if !ok {
			continue
		} // visitors and unassigned device users are ignored
		eventAt, err := time.ParseInLocation("2006-01-02 15:04:05", item.Date+" "+item.Time, time.Local)
		if err != nil {
			continue
		}
		existing, _ := a.store.ListAttendanceEventsByEmployeeDate(a.ctx, db.ListAttendanceEventsByEmployeeDateParams{EmployeeID: user.ID, EventDate: date})
		duplicate := false
		for _, previous := range existing {
			if absDuration(eventAt, parseAttendanceTime(previous.EventAt)) <= 30*time.Second {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		_ = a.store.InsertAttendanceEvent(a.ctx, db.InsertAttendanceEventParams{DeviceID: ip, EventID: item.ID, EmployeeID: user.ID, AkuvoxUserID: item.UserID, EmployeeName: strings.TrimSpace(item.Name), EventDate: item.Date, EventTime: item.Time, EventAt: eventAt.Format(time.RFC3339), Type: item.Type, Status: item.Status})
	}
	return map[string]any{"reachable": true, "configured": true, "message": "Connected", "events": len(payload.Data.Items)}, nil
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
	events, err := a.store.ListAttendanceEvents(a.ctx, date)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(events))
	for i, event := range events {
		result[i] = attendanceEventMap(event)
	}
	return result, nil
}

func (a *App) SubmitAttendanceComplaint(employeeID int64, date, requestedTime, note string) error {
	if strings.TrimSpace(note) == "" {
		return fmt.Errorf("note is required")
	}
	_, err := a.store.CreateAttendanceComplaint(a.ctx, db.CreateAttendanceComplaintParams{EmployeeID: employeeID, AttendanceDate: date, RequestedTime: requestedTime, Note: strings.TrimSpace(note), CreatedAt: time.Now().Format(time.RFC3339)})
	return err
}

func (a *App) ListAttendanceComplaints(month string) ([]map[string]any, error) {
	items, err := a.store.ListAttendanceComplaints(a.ctx, month+"%")
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(items))
	for i, item := range items {
		result[i] = map[string]any{"id": item.ID, "employeeId": item.EmployeeID, "date": item.AttendanceDate, "requestedTime": item.RequestedTime, "note": item.Note, "approved": item.Approved != 0, "createdAt": item.CreatedAt}
	}
	return result, nil
}

func (a *App) ApproveAttendanceComplaint(id int64, approved bool) error {
	return a.store.ApproveAttendanceComplaint(a.ctx, db.ApproveAttendanceComplaintParams{ID: id, Approved: boolInt(approved)})
}

func (a *App) ExportAttendanceMonth(month string) (string, error) {
	events, err := a.store.ListAttendanceEventsByMonth(a.ctx, month+"%")
	if err != nil {
		return "", err
	}
	complaints, err := a.ListAttendanceComplaints(month)
	if err != nil {
		return "", err
	}
	clean := map[string]any{"month": month, "events": make([]map[string]any, 0, len(events)), "complaints": complaints}
	for _, event := range events {
		clean["events"] = append(clean["events"].([]map[string]any), attendanceEventMap(event))
	}
	data, err := json.MarshalIndent(clean, "", "  ")
	return string(data), err
}
