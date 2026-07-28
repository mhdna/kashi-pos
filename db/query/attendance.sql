-- name: InsertAttendanceEvent :exec
INSERT OR IGNORE INTO attendance_events (device_id, event_id, employee_id, akuvox_user_id, employee_name, event_date, event_time, event_at, type, status)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListAttendanceEvents :many
SELECT * FROM attendance_events WHERE event_date = ? ORDER BY event_at;

-- name: ListAttendanceEventsByMonth :many
SELECT * FROM attendance_events WHERE event_date LIKE ? ORDER BY event_date, event_at;

-- name: ListAttendanceEventsByEmployeeDate :many
SELECT * FROM attendance_events WHERE employee_id = ? AND event_date = ? ORDER BY event_at;

-- name: CreateAttendanceComplaint :one
INSERT INTO attendance_complaints (employee_id, attendance_date, requested_time, note, approved, created_at)
VALUES (?, ?, ?, ?, 0, ?) RETURNING *;

-- name: ListAttendanceComplaints :many
SELECT * FROM attendance_complaints WHERE attendance_date LIKE ? ORDER BY attendance_date DESC, created_at DESC;

-- name: ListAttendanceComplaintsByDate :many
SELECT * FROM attendance_complaints WHERE attendance_date = ? ORDER BY created_at;

-- name: ApproveAttendanceComplaint :exec
UPDATE attendance_complaints SET approved = ? WHERE id = ?;

-- name: UpdateUserAttendanceID :exec
UPDATE users SET attendance_user_id = ? WHERE id = ?;
