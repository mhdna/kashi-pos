-- An expense or loan keyed in by mistake is cancelled, not deleted: the row
-- stays, listed as cancelled, and neither the shift's spend nor the month
-- totals count it. A till admin cancels one here; kashi cancels one with a
-- cancel_record command, and either side tells the other.
ALTER TABLE expenses ADD COLUMN cancelled_at text;
ALTER TABLE loans ADD COLUMN cancelled_at text;
