ALTER TABLE attendance_change_requests ADD COLUMN original_time TEXT NOT NULL DEFAULT '';
ALTER TABLE attendance_change_requests ADD COLUMN changed_from TEXT NOT NULL DEFAULT '';
ALTER TABLE attendance_change_requests ADD COLUMN changed_to TEXT NOT NULL DEFAULT '';
ALTER TABLE attendance_change_requests ADD COLUMN approved_by TEXT NOT NULL DEFAULT '';
